package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"incidentgpt-sanitizer/internal/auth"
	"incidentgpt-sanitizer/internal/config"
	"incidentgpt-sanitizer/internal/detector"
	"incidentgpt-sanitizer/internal/domain"
	"incidentgpt-sanitizer/internal/masking"
	"incidentgpt-sanitizer/internal/metrics"
	"incidentgpt-sanitizer/internal/policy"
	"incidentgpt-sanitizer/internal/pseudonymizer"
)

func testServer(t *testing.T, cfg config.Config) http.Handler {
	t.Helper()
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 20
	}
	if cfg.MaxInputBytes == 0 {
		cfg.MaxInputBytes = 1 << 20
	}
	if cfg.IPMode == "" {
		cfg.IPMode = "none"
	}
	if cfg.AuthSharedSecret == "" {
		cfg.AuthSharedSecret = "shared-secret"
	}
	if cfg.AuthMaxClockSkew == 0 {
		cfg.AuthMaxClockSkew = time.Minute
	}
	d, err := detector.New(cfg.CustomPatterns)
	if err != nil {
		t.Fatal(err)
	}
	engine := masking.NewEngine(cfg, d, policy.NewEngine(cfg), pseudonymizer.New(cfg.HashKey))
	return New(cfg, engine, auth.NewVerifier(cfg.AuthSharedSecret, cfg.AuthMaxClockSkew), metrics.New(), nil).Handler()
}

func signedRequest(t *testing.T, secret, requestID, path string, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	ts := time.Now().UTC().Format(time.RFC3339)
	bodyHash := sha256.Sum256(body)
	var sigBase bytes.Buffer
	sigBase.WriteString(ts)
	sigBase.WriteByte('\n')
	sigBase.WriteString(requestID)
	sigBase.WriteByte('\n')
	sigBase.WriteString(http.MethodPost)
	sigBase.WriteByte('\n')
	sigBase.WriteString(path)
	sigBase.WriteByte('\n')
	sigBase.WriteString(hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(sigBase.Bytes())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-IncidentGPT-Timestamp", ts)
	req.Header.Set("X-IncidentGPT-Request-ID", requestID)
	req.Header.Set("X-IncidentGPT-Signature", hex.EncodeToString(mac.Sum(nil)))
	return req
}

func TestSanitizeTextAPI(t *testing.T) {
	handler := testServer(t, config.Config{})
	body := []byte(`{"request_id":"req-1","source":"ai-worker","destination":"telegram","text":"Authorization: Bearer secret-token-value"}`)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, signedRequest(t, "shared-secret", "req-1", "/v1/sanitize/text", body))
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d body=%s", rr.Code, rr.Body.String())
	}
	var resp domain.TextResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resp.Text, "secret-token-value") || !strings.Contains(resp.Text, "[REDACTED_TOKEN]") {
		t.Fatalf("unsafe text response: %#v", resp)
	}
}

func TestInspectDoesNotReturnValues(t *testing.T) {
	handler := testServer(t, config.Config{})
	body := []byte(`{"request_id":"req-2","source":"enricher","destination":"llm","payload":{"authorization":"Bearer secret-token-value"}}`)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, signedRequest(t, "shared-secret", "req-2", "/v1/inspect", body))
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status %d body=%s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "secret-token-value") {
		t.Fatalf("inspect leaked sensitive value: %s", rr.Body.String())
	}
}

func TestInvalidSignatureReplayAndPayloadTooLarge(t *testing.T) {
	handler := testServer(t, config.Config{})
	body := []byte(`{"request_id":"req-3","source":"ai-worker","destination":"telegram","text":"ok"}`)

	bad := httptest.NewRecorder()
	req := signedRequest(t, "wrong-secret", "req-3a", "/v1/sanitize/text", body)
	handler.ServeHTTP(bad, req)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad signature, got %d", bad.Code)
	}

	handler = testServer(t, config.Config{MaxInputBytes: 64})
	tooLarge := httptest.NewRecorder()
	handler.ServeHTTP(tooLarge, signedRequest(t, "shared-secret", "req-3b", "/v1/sanitize/text", body))
	if tooLarge.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", tooLarge.Code)
	}

	handler = testServer(t, config.Config{})
	replayReq := signedRequest(t, "shared-secret", "req-3c", "/v1/sanitize/text", body)
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, replayReq)
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, replayReq)
	if first.Code != http.StatusOK || second.Code != http.StatusUnauthorized {
		t.Fatalf("expected replay rejection, first=%d second=%d", first.Code, second.Code)
	}
}
