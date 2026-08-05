package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SanitizerClient struct {
	baseURL string
	secret  string
	client  *http.Client

	mu          sync.Mutex
	failures    int
	circuitOpen time.Time
}

type sanitizerPayloadRequest struct {
	RequestID   string          `json:"request_id"`
	Source      string          `json:"source"`
	Destination string          `json:"destination"`
	ContentType string          `json:"content_type"`
	Payload     json.RawMessage `json:"payload"`
}

type sanitizerPayloadResponse struct {
	RequestID string          `json:"request_id"`
	Status    string          `json:"status"`
	Payload   json.RawMessage `json:"payload"`
}

type sanitizerTextRequest struct {
	RequestID   string `json:"request_id"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Text        string `json:"text"`
}

type sanitizerTextResponse struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Text      string `json:"text"`
}

func newSanitizerClient(baseURL, secret string, timeout time.Duration) *SanitizerClient {
	return &SanitizerClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		secret:  strings.TrimSpace(secret),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *SanitizerClient) SanitizePayload(ctx context.Context, source, destination string, payload any) (json.RawMessage, error) {
	if c.baseURL == "" || c.secret == "" {
		return nil, fmt.Errorf("sanitizer is not configured")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload for sanitizer: %w", err)
	}
	reqBody, err := json.Marshal(sanitizerPayloadRequest{
		RequestID:   requestID(),
		Source:      source,
		Destination: destination,
		ContentType: "application/json",
		Payload:     raw,
	})
	if err != nil {
		return nil, err
	}
	body, err := c.do(ctx, http.MethodPost, "/v1/sanitize", reqBody)
	if err != nil {
		return nil, err
	}
	var resp sanitizerPayloadResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode sanitizer response")
	}
	if resp.Status != "sanitized" {
		return nil, fmt.Errorf("sanitizer returned unsafe status")
	}
	return resp.Payload, nil
}

func (c *SanitizerClient) SanitizeText(ctx context.Context, source, destination, text string) (string, error) {
	if c.baseURL == "" || c.secret == "" {
		return "", fmt.Errorf("sanitizer is not configured")
	}
	reqBody, err := json.Marshal(sanitizerTextRequest{
		RequestID:   requestID(),
		Source:      source,
		Destination: destination,
		Text:        text,
	})
	if err != nil {
		return "", err
	}
	body, err := c.do(ctx, http.MethodPost, "/v1/sanitize/text", reqBody)
	if err != nil {
		return "", err
	}
	var resp sanitizerTextResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode sanitizer text response")
	}
	if resp.Status != "sanitized" {
		return "", fmt.Errorf("sanitizer returned unsafe text status")
	}
	return resp.Text, nil
}

func (c *SanitizerClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if err := c.beforeRequest(); err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff(attempt))
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		signSanitizerRequest(req, []byte(c.secret), body)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			if retryableError(err) {
				continue
			}
			break
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			c.recordSuccess()
			return respBody, nil
		}
		lastErr = fmt.Errorf("sanitizer HTTP %d", resp.StatusCode)
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout {
			continue
		}
		break
	}
	c.recordFailure()
	return nil, lastErr
}

func (c *SanitizerClient) beforeRequest() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.circuitOpen) {
		return fmt.Errorf("sanitizer circuit breaker is open")
	}
	return nil
}

func (c *SanitizerClient) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = 0
	c.circuitOpen = time.Time{}
}

func (c *SanitizerClient) recordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	if c.failures >= 5 {
		c.circuitOpen = time.Now().Add(15 * time.Second)
	}
}

func signSanitizerRequest(req *http.Request, secret []byte, body []byte) {
	ts := time.Now().UTC().Format(time.RFC3339)
	id := requestID()
	bodyHash := sha256.Sum256(body)
	var b bytes.Buffer
	b.WriteString(ts)
	b.WriteByte('\n')
	b.WriteString(id)
	b.WriteByte('\n')
	b.WriteString(req.Method)
	b.WriteByte('\n')
	b.WriteString(req.URL.Path)
	b.WriteByte('\n')
	b.WriteString(hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(b.Bytes())
	req.Header.Set("X-IncidentGPT-Timestamp", ts)
	req.Header.Set("X-IncidentGPT-Request-ID", id)
	req.Header.Set("X-IncidentGPT-Signature", hex.EncodeToString(mac.Sum(nil)))
}

func retryableError(err error) bool {
	var netErr net.Error
	return strings.Contains(err.Error(), "connection reset") || (errors.As(err, &netErr) && netErr.Timeout())
}

func backoff(attempt int) time.Duration {
	base := time.Duration(100*(1<<attempt)) * time.Millisecond
	return base + time.Duration(rand.Intn(75))*time.Millisecond
}

func requestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
}
