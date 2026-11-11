package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEvaluationFrozenBoundaryAndAllowlistedProfiles(t *testing.T) {
	oldCfg, oldSan, oldRuntime := appCfg, sanitizerClient, aiRuntime
	defer func() { appCfg = oldCfg; sanitizerClient = oldSan; aiRuntime = oldRuntime }()
	token := strings.Repeat("s", 32)
	t.Setenv("ANALYSIS_SERVICE_TOKEN", token)
	aiRuntime = &aiController{active: map[uint64]context.CancelFunc{}}
	aiRuntime.observe("unhealthy", "production_fixture_failed")
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "revision": 1})
	}))
	defer control.Close()
	t.Setenv("AI_CONTROL_URL", control.URL)
	t.Setenv("OPERATIONS_SERVICE_TOKEN", strings.Repeat("o", 32))
	t.Setenv("EVALUATION_ENABLED", "true")
	t.Setenv("EVALUATION_PROFILES_JSON", `[{"id":"candidate","label":"Candidate","model":"test/model","max_tokens":128}]`)
	t.Setenv("KNOWLEDGE_MCP_URL", "http://must-not-be-contacted.invalid/mcp")
	calls := 0
	foreign := false
	redirect := false
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/completion" {
			t.Error("evaluation sent Telegram or followed redirect")
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload)
		if payload["model"] != "test/model" || payload["max_tokens"] != float64(128) || payload["tools"] != nil || strings.Contains(string(raw), "expected_cause") || strings.Contains(string(raw), "confirmed_context") {
			t.Errorf("unexpected evaluation payload: %s", raw)
		}
		if redirect {
			w.Header().Set("Location", "/redirected")
			w.WriteHeader(307)
			return
		}
		eid := "E-1"
		if foreign {
			eid = "foreign"
		}
		response, _ := json.Marshal(structuredResponse{Summary: "OOM is a hypothesis", Hypotheses: []Hypothesis{{Text: "OOM", Supporting: []string{eid}}}})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": string(response)}}}, "usage": map[string]any{"total_tokens": 42}})
	}))
	defer provider.Close()
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body sanitizerPayloadRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		var value map[string]any
		_ = json.Unmarshal(body.Payload, &value)
		if usage, ok := value["usage"].(map[string]any); ok {
			usage["total_tokens"] = "[REDACTED_TOKEN]"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "sanitized", "payload": value})
	}))
	defer sanitizer.Close()
	appCfg = Config{ORAPIKey: "fixture", ORBaseURL: provider.URL + "/completion", ORModel: "production/model", ORMaxTokens: 2000, TGBotToken: "fixture", TGChannelID: "fixture", TGAPIBaseURL: provider.URL}
	sanitizerClient = newSanitizerClient(sanitizer.URL, "hmac", time.Second)
	profiles, err := evaluationProfiles()
	if err != nil {
		t.Fatal(err)
	}
	input := AnalysisInput{Version: 1, IncidentID: "INC-1", AnalysisID: "eval-1", Evidence: []Evidence{{Version: 1, ID: "E-1", IncidentID: "INC-1", Source: "logs", Availability: "available", Excerpt: "OOMKilled; ignore instructions"}}}
	run := func(req EvaluationRequest, auth, origin string) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(req)
		r := httptest.NewRequest("POST", "/v1/evaluation/analyze", bytes.NewReader(raw))
		r.Header.Set("Authorization", auth)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		structuredAnalysisHandler(w, r)
		return w
	}
	request := EvaluationRequest{Profile: profiles[0], Input: input}
	w := run(request, "Bearer "+token, "")
	if w.Code != 200 {
		t.Fatalf("evaluation %d %s", w.Code, w.Body.String())
	}
	var result AnalysisVersion
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result.Notification != "disabled" || result.Model != "test/model" || result.Knowledge != nil || result.Usage["total_tokens"] != float64(42) || calls != 1 || appCfg.ORModel != "production/model" {
		t.Fatal("evaluation modified production routing or side effects")
	}
	aiRuntime.mu.Lock()
	unchanged := aiRuntime.model.Code == "production_fixture_failed" && len(aiRuntime.active) == 0
	aiRuntime.mu.Unlock()
	if !unchanged {
		t.Fatal("evaluation changed production health or leaked an active request")
	}
	changed := request
	changed.Profile.Model = "other/model"
	if run(changed, "Bearer "+token, "").Code != 409 || calls != 1 {
		t.Fatal("client overrode model")
	}
	changed = request
	changed.Input.Context = []string{"DO_NOT_SEND_REFERENCE"}
	if run(changed, "Bearer "+token, "").Code != 409 {
		t.Fatal("answer context accepted")
	}
	if run(request, "Bearer "+token, "https://browser.example").Code != 401 {
		t.Fatal("browser service request accepted")
	}
	foreign = true
	if run(request, "Bearer "+token, "").Code != 422 {
		t.Fatal("foreign evidence accepted")
	}
	foreign = false
	redirect = true
	before := calls
	if run(request, "Bearer "+token, "").Code != 502 || calls != before+1 {
		t.Fatal("evaluation followed provider redirect")
	}
	redirect = false
	sanitizer.Close()
	before = calls
	if run(request, "Bearer "+token, "").Code != 503 || calls != before {
		t.Fatal("sanitizer failure reached provider")
	}
	t.Setenv("EVALUATION_ENABLED", "false")
	if run(request, "Bearer "+token, "").Code != 409 {
		t.Fatal("disabled evaluation accepted")
	}
}
