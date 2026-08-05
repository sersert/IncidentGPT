package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Verifier struct {
	secret []byte
	skew   time.Duration
	mu     sync.Mutex
	seen   map[string]time.Time
}

func NewVerifier(secret string, skew time.Duration) *Verifier {
	return &Verifier{secret: []byte(secret), skew: skew, seen: map[string]time.Time{}}
}

func (v *Verifier) Verify(r *http.Request, body []byte) error {
	if len(v.secret) == 0 {
		return fmt.Errorf("auth is not configured")
	}
	tsRaw := r.Header.Get("X-IncidentGPT-Timestamp")
	requestID := r.Header.Get("X-IncidentGPT-Request-ID")
	got := r.Header.Get("X-IncidentGPT-Signature")
	if tsRaw == "" || requestID == "" || got == "" {
		return fmt.Errorf("missing signature headers")
	}
	ts, err := time.Parse(time.RFC3339, tsRaw)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	if delta := time.Since(ts); delta > v.skew || delta < -v.skew {
		return fmt.Errorf("timestamp outside allowed clock skew")
	}
	want := Sign(v.secret, tsRaw, requestID, r.Method, r.URL.Path, body)
	gotBytes, err := hex.DecodeString(got)
	if err != nil {
		return fmt.Errorf("invalid signature encoding")
	}
	wantBytes, _ := hex.DecodeString(want)
	if !hmac.Equal(gotBytes, wantBytes) {
		return fmt.Errorf("invalid signature")
	}
	if !v.markSeen(requestID, ts) {
		return fmt.Errorf("replayed request id")
	}
	return nil
}

func Sign(secret []byte, timestamp, requestID, method, path string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	var b bytes.Buffer
	b.WriteString(timestamp)
	b.WriteByte('\n')
	b.WriteString(requestID)
	b.WriteByte('\n')
	b.WriteString(method)
	b.WriteByte('\n')
	b.WriteString(path)
	b.WriteByte('\n')
	b.WriteString(hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(b.Bytes())
	return hex.EncodeToString(mac.Sum(nil))
}

func (v *Verifier) markSeen(requestID string, ts time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	cutoff := time.Now().Add(-2 * v.skew)
	for id, seenAt := range v.seen {
		if seenAt.Before(cutoff) {
			delete(v.seen, id)
		}
	}
	if _, ok := v.seen[requestID]; ok {
		return false
	}
	v.seen[requestID] = ts
	return true
}
