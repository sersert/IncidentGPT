package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTelegramSendUsesSanitizedText(t *testing.T) {
	var telegramBody string
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sanitize/text" {
			t.Fatalf("unexpected sanitizer path %s", r.URL.Path)
		}
		var req sanitizerTextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(req.Text, "secret-token-value") {
			t.Fatalf("test setup expected raw text to reach sanitizer")
		}
		_ = json.NewEncoder(w).Encode(sanitizerTextResponse{
			RequestID: req.RequestID,
			Status:    "sanitized",
			Text:      "Authorization: Bearer [REDACTED_TOKEN]",
		})
	}))
	defer sanitizer.Close()

	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(body)
		telegramBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": map[string]any{
				"message_id": 42,
			},
		})
	}))
	defer telegram.Close()

	appCfg = Config{
		TGBotToken:       "test-token",
		TGChannelID:      "chat",
		TGParseMode:      "Markdown",
		TGAPIBaseURL:     telegram.URL,
		SanitizerURL:     sanitizer.URL,
		SanitizerSecret:  "shared-secret",
		SanitizerTimeout: time.Second,
	}
	sanitizerClient = newSanitizerClient(appCfg.SanitizerURL, appCfg.SanitizerSecret, appCfg.SanitizerTimeout)

	if _, err := sendTelegramMessage(t.Context(), "chat", "Authorization: Bearer secret-token-value", 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(telegramBody, "secret-token-value") {
		t.Fatalf("telegram received unsanitized text: %s", telegramBody)
	}
	if !strings.Contains(telegramBody, "REDACTED") {
		t.Fatalf("telegram did not receive sanitized marker: %s", telegramBody)
	}
}
