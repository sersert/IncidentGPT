package pseudonymizer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Pseudonymizer struct {
	key []byte
}

func New(key string) *Pseudonymizer {
	return &Pseudonymizer{key: []byte(key)}
}

func (p *Pseudonymizer) Enabled() bool {
	return len(p.key) > 0
}

func (p *Pseudonymizer) Resource(resourceType, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !p.Enabled() {
		return value
	}
	mac := hmac.New(sha256.New, p.key)
	_, _ = mac.Write([]byte(resourceType + ":" + value))
	sum := hex.EncodeToString(mac.Sum(nil))
	return resourceType + "-" + sum[:6]
}
