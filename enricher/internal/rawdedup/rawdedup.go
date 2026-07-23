package rawdedup

import (
	"strings"
	"time"
)

// KeyFor returns the Redis deduplication key for raw alert delivery.
// The boolean is false when deduplication should be bypassed.
func KeyFor(ttl time.Duration, fingerprint string) (string, bool) {
	if ttl <= 0 {
		return "", false
	}

	fp := strings.TrimSpace(fingerprint)
	if fp == "" {
		return "", false
	}

	return "raw:" + fp, true
}
