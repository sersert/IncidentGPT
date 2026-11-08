package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type controlState struct {
	Enabled   *bool     `json:"enabled"`
	Revision  int       `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}
type modelObservation struct {
	Status    string    `json:"status"`
	Code      string    `json:"code"`
	CheckedAt time.Time `json:"checked_at"`
}
type aiController struct {
	mu      sync.Mutex
	state   controlState
	known   bool
	checked time.Time
	active  map[uint64]context.CancelFunc
	next    uint64
	model   modelObservation
}

var aiRuntime = &aiController{active: map[uint64]context.CancelFunc{}}
var operationsHTTP = &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
var errAIPaused = fmt.Errorf("ai_paused_or_control_unavailable")

func controlRequired() bool { return strings.TrimSpace(os.Getenv("AI_CONTROL_URL")) != "" }
func (c *aiController) apply(state controlState, err error) {
	c.mu.Lock()
	if err == nil && state.Revision < c.state.Revision {
		err = errAIPaused // A restored/lost Redis record must never reopen the provider.
	}
	c.checked = time.Now().UTC()
	c.known = err == nil
	if err == nil && state.Revision >= c.state.Revision {
		c.state = state
	}
	stop := err != nil || c.state.Enabled == nil || !*c.state.Enabled
	var cancel []context.CancelFunc
	if stop {
		for _, fn := range c.active {
			cancel = append(cancel, fn)
		}
	}
	c.mu.Unlock()
	for _, fn := range cancel {
		fn()
	}
}
func (c *aiController) refresh(ctx context.Context) error {
	if !controlRequired() {
		return nil
	}
	token := strings.TrimSpace(os.Getenv("OPERATIONS_SERVICE_TOKEN"))
	var state controlState
	if len(token) < 32 {
		c.apply(state, errAIPaused)
		return errAIPaused
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", os.Getenv("AI_CONTROL_URL"), nil)
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+token)
		var resp *http.Response
		resp, err = operationsHTTP.Do(req)
		if err == nil {
			defer resp.Body.Close()
			raw, e := io.ReadAll(io.LimitReader(resp.Body, 4097))
			if e != nil || len(raw) > 4096 || resp.StatusCode != 200 || json.Unmarshal(raw, &state) != nil || state.Enabled == nil || state.Revision < 0 {
				err = errAIPaused
			}
		}
	}
	c.apply(state, err)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil || !c.known || c.state.Enabled == nil || !*c.state.Enabled {
		return errAIPaused
	}
	return nil
}
func (c *aiController) paused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return controlRequired() && (!c.known || c.state.Enabled == nil || !*c.state.Enabled)
}
func (c *aiController) begin(ctx context.Context) (context.Context, func(), error) {
	if err := c.refresh(ctx); err != nil {
		return nil, nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.known || c.state.Enabled == nil || !*c.state.Enabled {
		return nil, nil, errAIPaused
	}
	child, cancel := context.WithCancel(ctx)
	c.next++
	id := c.next
	c.active[id] = cancel
	return child, func() { cancel(); c.mu.Lock(); delete(c.active, id); c.mu.Unlock() }, nil
}
func startAIControlPolling() {
	if !controlRequired() {
		return
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			_ = aiRuntime.refresh(context.Background())
			<-ticker.C
		}
	}()
}

type controlledProviderTransport struct{ base http.RoundTripper }

func (t controlledProviderTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if !controlRequired() || r.Method != "POST" || r.URL.String() != appCfg.ORBaseURL {
		return t.base.RoundTrip(r)
	}
	ctx, done, err := aiRuntime.begin(r.Context())
	if err != nil {
		return nil, err
	}
	skipObservation := r.Context().Value(evaluationRequestKey{}) == true
	resp, err := t.base.RoundTrip(r.Clone(ctx))
	if err != nil {
		done()
		if !skipObservation {
			if aiRuntime.paused() {
				aiRuntime.observe("unknown", "canceled_while_paused")
			} else {
				aiRuntime.observe("unhealthy", "request_failed")
			}
		}
		return nil, err
	}
	resp.Body = &observedModelBody{ReadCloser: resp.Body, status: resp.StatusCode, done: done, skipObservation: skipObservation}
	return resp, nil
}
func (c *aiController) observe(status, code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.model = modelObservation{Status: status, Code: code, CheckedAt: time.Now().UTC()}
}

type observedModelBody struct {
	skipObservation bool
	io.ReadCloser
	status   int
	done     func()
	data     bytes.Buffer
	overflow bool
	once     sync.Once
}

func (b *observedModelBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if b.data.Len()+n <= 128<<10 {
		_, _ = b.data.Write(p[:n])
	} else {
		b.overflow = true
	}
	return n, err
}
func (b *observedModelBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(func() {
		defer b.done()
		if b.skipObservation {
			b.data.Reset()
			return
		}
		status, code := "unhealthy", fmt.Sprintf("http_%d", b.status)
		if b.status >= 200 && b.status < 300 {
			var result struct {
				Choices []struct {
					Message struct {
						Content   string            `json:"content"`
						ToolCalls []json.RawMessage `json:"tool_calls"`
					} `json:"message"`
				} `json:"choices"`
			}
			if b.overflow {
				status, code = "unknown", "response_exceeds_observation_budget"
			} else if json.Unmarshal(b.data.Bytes(), &result) == nil && len(result.Choices) > 0 && (result.Choices[0].Message.Content != "" || len(result.Choices[0].Message.ToolCalls) > 0) {
				status, code = "healthy", "inference_response_received"
			} else {
				code = "invalid_provider_response"
			}
		}
		aiRuntime.observe(status, code)
		b.data.Reset()
	})
	return err
}
func operationsAuthorized(w http.ResponseWriter, r *http.Request) bool {
	token := strings.TrimSpace(os.Getenv("OPERATIONS_SERVICE_TOKEN"))
	if r.Header.Get("Origin") != "" || len(token) < 32 || !constantToken(r.Header.Get("Authorization"), "Bearer "+token) {
		http.Error(w, "unauthorized", 401)
		return false
	}
	return true
}
func workerOperations(w http.ResponseWriter, r *http.Request) {
	if !operationsAuthorized(w, r) {
		return
	}
	aiRuntime.mu.Lock()
	state := aiRuntime.state
	known := aiRuntime.known
	checked := aiRuntime.checked
	active := len(aiRuntime.active)
	model := aiRuntime.model
	aiRuntime.mu.Unlock()
	if model.CheckedAt.IsZero() || time.Since(model.CheckedAt) > 5*time.Minute {
		model.Status = "unknown"
		model.Code = "no_recent_inference"
	}
	writeJSON(w, 200, map[string]any{"service": "ai-worker", "managed": controlRequired(), "control_known": known, "control": state, "control_checked_at": checked, "active_provider_requests": active, "model": model, "telegram": telegramOperations(r.Context()), "model_name": appCfg.ORModel, "checked_at": time.Now().UTC()})
}

var telegramCheck struct {
	sync.Mutex
	at     time.Time
	status string
}

func telegramOperations(ctx context.Context) map[string]any {
	telegramCheck.Lock()
	defer telegramCheck.Unlock()
	if time.Since(telegramCheck.at) > time.Minute {
		telegramCheck.status = "not_configured"
		telegramCheck.at = time.Now().UTC()
		if appCfg.TGBotToken != "" {
			telegramCheck.status = "unhealthy"
			ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(appCfg.TGAPIBaseURL, "/")+"/bot"+appCfg.TGBotToken+"/getMe", nil)
			if err == nil {
				resp, e := operationsHTTP.Do(req)
				if e == nil {
					defer resp.Body.Close()
					var data struct {
						OK bool `json:"ok"`
					}
					raw, e := io.ReadAll(io.LimitReader(resp.Body, 8193))
					if e == nil && len(raw) <= 8192 && resp.StatusCode == 200 && json.Unmarshal(raw, &data) == nil && data.OK {
						telegramCheck.status = "healthy"
					}
				}
			}
		}
	}
	return map[string]any{"status": telegramCheck.status, "checked_at": telegramCheck.at}
}

var modelProbeLimit struct {
	sync.Mutex
	at time.Time
}

func workerModelProbe(w http.ResponseWriter, r *http.Request) {
	if !operationsAuthorized(w, r) {
		return
	}
	if !controlRequired() {
		http.Error(w, "AI control is not configured", 503)
		return
	}
	modelProbeLimit.Lock()
	if time.Since(modelProbeLimit.at) < time.Minute {
		modelProbeLimit.Unlock()
		http.Error(w, "probe limited to once per minute", 429)
		return
	}
	modelProbeLimit.at = time.Now()
	modelProbeLimit.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if aiRuntime.refresh(ctx) != nil {
		http.Error(w, "AI paused or control unavailable", 423)
		return
	}
	safe, err := sanitizerClient.SanitizePayload(ctx, "ai-worker", "llm", []map[string]string{{"role": "user", "content": "Reply with OK."}})
	if err != nil {
		http.Error(w, "sanitizer unavailable", 503)
		return
	}
	var messages any
	if json.Unmarshal(safe, &messages) != nil {
		http.Error(w, "invalid sanitized probe", 503)
		return
	}
	body, _ := json.Marshal(map[string]any{"model": appCfg.ORModel, "max_tokens": 8, "messages": messages})
	req, err := http.NewRequestWithContext(ctx, "POST", appCfg.ORBaseURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "invalid provider configuration", 503)
		return
	}
	req.Header.Set("Authorization", "Bearer "+appCfg.ORAPIKey)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, "probe failed or AI paused", 502)
		return
	}
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 128<<10+1))
	_ = resp.Body.Close()
	var result struct {
		Choices []struct {
			Message struct {
				Content   string            `json:"content"`
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	success := readErr == nil && len(raw) <= 128<<10 && resp.StatusCode >= 200 && resp.StatusCode < 300 && json.Unmarshal(raw, &result) == nil && len(result.Choices) > 0 && (result.Choices[0].Message.Content != "" || len(result.Choices[0].Message.ToolCalls) > 0)
	writeJSON(w, 200, map[string]any{"success": success, "latency_ms": time.Since(started).Milliseconds()})
}
