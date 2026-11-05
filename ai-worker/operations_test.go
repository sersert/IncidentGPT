package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func setupControl(t *testing.T) (*atomic.Bool, *atomic.Int64, *atomic.Bool) {
	t.Helper()
	oldCfg, oldRuntime := appCfg, aiRuntime
	t.Cleanup(func() { appCfg = oldCfg; aiRuntime = oldRuntime })
	aiRuntime = &aiController{active: map[uint64]context.CancelFunc{}}
	enabled := &atomic.Bool{}
	revision := &atomic.Int64{}
	unavailable := &atomic.Bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 32) {
			t.Error("missing service authentication")
		}
		if unavailable.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		writeJSON(w, 200, map[string]any{"enabled": enabled.Load(), "revision": revision.Load(), "updated_at": time.Now().UTC()})
	}))
	t.Cleanup(server.Close)
	t.Setenv("AI_CONTROL_URL", server.URL)
	t.Setenv("OPERATIONS_SERVICE_TOKEN", strings.Repeat("t", 32)+"\n")
	return enabled, revision, unavailable
}
func TestPauseBlocksLegacyAndStructuredProviderCalls(t *testing.T) {
	enabled, revision, unavailable := setupControl(t)
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": "OK"}}}})
	}))
	defer provider.Close()
	appCfg = Config{ORAPIKey: "provider-secret", ORBaseURL: provider.URL, ORModel: "fixture", ORTimeout: time.Second, ORMaxTokens: 20}
	if _, err := callOpenRouterWithSystem(context.Background(), "system", "fixture"); err == nil || calls.Load() != 0 {
		t.Fatal("legacy request bypassed pause")
	}
	if _, _, err := structuredProvider(context.Background(), AnalysisInput{Version: 1}); err == nil || calls.Load() != 0 {
		t.Fatal("structured request bypassed pause")
	}
	enabled.Store(true)
	revision.Store(1)
	if text, err := callOpenRouterWithSystem(context.Background(), "system", "fixture"); err != nil || text != "OK" || calls.Load() != 1 {
		t.Fatalf("enable did not resume provider: %s %v", text, err)
	}
	aiRuntime.mu.Lock()
	status := aiRuntime.model
	active := len(aiRuntime.active)
	aiRuntime.mu.Unlock()
	if status.Status != "healthy" || active != 0 {
		t.Fatal("real inference result was not observed")
	}
	unavailable.Store(true)
	if _, err := callOpenRouterWithSystem(context.Background(), "system", "fixture"); err == nil || calls.Load() != 1 {
		t.Fatal("control outage spent provider tokens")
	}
	// Telegram is independent of the model endpoint and remains usable while paused.
	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]bool{"ok": true}) }))
	defer telegram.Close()
	resp, err := httpClient.Post(telegram.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal("pause blocked unrelated notification traffic")
	}
	resp.Body.Close()
}
func TestPauseCancelsActiveProviderRequests(t *testing.T) {
	enabled, revision, _ := setupControl(t)
	enabled.Store(true)
	revision.Store(1)
	started := make(chan struct{})
	canceled := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
			close(canceled)
		case <-time.After(6 * time.Second):
		}
	}))
	defer provider.Close()
	appCfg = Config{ORAPIKey: "fixture", ORBaseURL: provider.URL, ORTimeout: 5 * time.Second, ORModel: "fixture", ORMaxTokens: 20}
	finished := make(chan error, 1)
	go func() { _, err := callOpenRouterWithSystem(context.Background(), "system", "fixture"); finished <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("provider never started")
	}
	enabled.Store(false)
	revision.Store(2)
	_ = aiRuntime.refresh(context.Background())
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("paused request returned success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("provider request was not canceled")
	}
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("provider did not observe HTTP cancellation")
	}
}
func TestModelProbeIsExplicitAuthenticatedAndRateLimited(t *testing.T) {
	enabled, revision, _ := setupControl(t)
	enabled.Store(true)
	revision.Store(1)
	oldSan := sanitizerClient
	defer func() { sanitizerClient = oldSan }()
	modelProbeLimit.Lock()
	modelProbeLimit.at = time.Time{}
	modelProbeLimit.Unlock()
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sanitizerPayloadRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": json.RawMessage(req.Payload)})
	}))
	defer sanitizer.Close()
	sanitizerClient = newSanitizerClient(sanitizer.URL, "hmac", time.Second)
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["max_tokens"] != float64(8) {
			t.Error("probe exceeded token budget")
		}
		writeJSON(w, 200, map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": "OK"}}}})
	}))
	defer provider.Close()
	appCfg = Config{ORAPIKey: "secret", ORBaseURL: provider.URL, ORModel: "fixture"}
	request := func(auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/internal/operations/model-probe", nil)
		if auth {
			r.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
		}
		w := httptest.NewRecorder()
		workerModelProbe(w, r)
		return w
	}
	if w := request(false); w.Code != 401 {
		t.Fatal("unauthorized probe allowed")
	}
	r := httptest.NewRequest("GET", "/internal/operations", nil)
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
	w := httptest.NewRecorder()
	workerOperations(w, r)
	if w.Code != 200 || calls.Load() != 0 {
		t.Fatal("passive monitoring generated tokens")
	}
	if w := request(true); w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("probe failed: %d %s", w.Code, w.Body.String())
	}
	if w := request(true); w.Code != 429 || calls.Load() != 1 {
		t.Fatal("probe limiter failed")
	}
	modelProbeLimit.Lock()
	modelProbeLimit.at = time.Time{}
	modelProbeLimit.Unlock()
	enabled.Store(false)
	revision.Store(2)
	if w := request(true); w.Code != 423 || calls.Load() != 1 {
		t.Fatal("paid probe ran while paused")
	}
	enabled.Store(true)
	revision.Store(3)
	if w := request(true); w.Code != 200 || calls.Load() != 2 {
		t.Fatal("paused preflight consumed probe budget")
	}
}
func TestStaleControlResponseCannotReenableAI(t *testing.T) {
	_, _, _ = setupControl(t)
	on, off := true, false
	aiRuntime.apply(controlState{Enabled: &off, Revision: 3}, nil)
	aiRuntime.apply(controlState{Enabled: &on, Revision: 2}, nil)
	if !aiRuntime.paused() {
		t.Fatal("stale response reopened a newer pause")
	}
	aiRuntime.apply(controlState{Enabled: &on, Revision: 4}, nil)
	aiRuntime.apply(controlState{Enabled: &off, Revision: 0}, nil)
	if !aiRuntime.paused() {
		t.Fatal("Redis data loss retained permission to spend tokens")
	}
}

type countingProviderTransport struct{ calls *atomic.Int32 }

func (t countingProviderTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"OK"}}]}`))}, nil
}
func TestCanonicalProviderURLsCannotBypassPauseOrRedirectGuard(t *testing.T) {
	setupControl(t)
	var calls atomic.Int32
	transport := controlledProviderTransport{base: countingProviderTransport{calls: &calls}}
	for _, endpoint := range []string{"HTTPS://openrouter.ai/api/v1/chat/completions", "https://OPENROUTER.AI:443/api/v1/chat/completions", "https://openrouter.ai/api/v1/chat/completions?route=primary"} {
		appCfg.ORBaseURL = endpoint
		request, _ := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", nil)
		if _, err := transport.RoundTrip(request); err == nil || calls.Load() != 0 {
			t.Fatalf("noncanonical URL bypassed pause: %s", endpoint)
		}
		redirect, _ := http.NewRequest("GET", "https://other.example/", nil)
		if err := httpClient.CheckRedirect(redirect, []*http.Request{request}); err != http.ErrUseLastResponse {
			t.Fatalf("provider redirect escaped guard: %s", endpoint)
		}
	}
}
func TestMissingControlStateCanBeRecoveredWithoutWorkerRestart(t *testing.T) {
	setupControl(t)
	on, off := true, false
	aiRuntime.apply(controlState{Enabled: &on, Revision: 7}, nil)
	aiRuntime.apply(controlState{Enabled: &off, Revision: 0}, nil)
	if !aiRuntime.paused() {
		t.Fatal("loss of control record did not pause")
	}
	aiRuntime.apply(controlState{Enabled: &on, Revision: 1}, nil)
	if aiRuntime.paused() {
		t.Fatal("operator could not enable after safe reset")
	}
}
