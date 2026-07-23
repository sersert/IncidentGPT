package tests

import (
	"testing"
	"time"

	"incident-enricher/internal/rawdedup"
)

func TestRawDedupKeyFor(t *testing.T) {
	got, ok := rawdedup.KeyFor(time.Minute, "abc123")
	if !ok {
		t.Fatal("KeyFor() disabled dedup unexpectedly")
	}
	if got != "raw:abc123" {
		t.Fatalf("KeyFor() = %q, want %q", got, "raw:abc123")
	}
}

func TestRawDedupKeyForTrimsFingerprint(t *testing.T) {
	got, ok := rawdedup.KeyFor(time.Minute, "  fp-1  ")
	if !ok {
		t.Fatal("KeyFor() disabled dedup unexpectedly")
	}
	if got != "raw:fp-1" {
		t.Fatalf("KeyFor() = %q, want %q", got, "raw:fp-1")
	}
}

func TestRawDedupKeyForBypassCases(t *testing.T) {
	tests := []struct {
		name        string
		ttl         time.Duration
		fingerprint string
	}{
		{name: "zero ttl", ttl: 0, fingerprint: "fp-1"},
		{name: "negative ttl", ttl: -time.Second, fingerprint: "fp-1"},
		{name: "empty fingerprint", ttl: time.Minute, fingerprint: ""},
		{name: "blank fingerprint", ttl: time.Minute, fingerprint: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, ok := rawdedup.KeyFor(tt.ttl, tt.fingerprint)
			if ok {
				t.Fatalf("KeyFor() enabled dedup with key %q", key)
			}
			if key != "" {
				t.Fatalf("KeyFor() key = %q, want empty", key)
			}
		})
	}
}
