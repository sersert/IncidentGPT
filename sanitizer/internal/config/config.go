package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CustomPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
}

type Config struct {
	ListenAddress       string
	MetricsAddress      string
	FailClosed          bool
	MaxDepth            int
	MaxInputBytes       int64
	RedactEmails        bool
	RedactPhones        bool
	RedactCertificates  bool
	RedactUsernames     bool
	RedactResourceNames bool
	IPMode              string
	HashKey             string
	AuthSharedSecret    string
	AuthMaxClockSkew    time.Duration
	MaxCustomPatterns   int
	MaxPatternLength    int
	CustomPatterns      []CustomPattern
	RequestTimeout      time.Duration
	ShutdownTimeout     time.Duration
	Version             string
	Commit              string
	BuildTime           string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddress:      envOr("SANITIZER_LISTEN_ADDRESS", ":8080"),
		MetricsAddress:     envOr("SANITIZER_METRICS_ADDRESS", ":9090"),
		FailClosed:         envBool("SANITIZER_FAIL_CLOSED", true),
		MaxDepth:           envInt("SANITIZER_MAX_DEPTH", 20),
		MaxInputBytes:      int64(envInt("SANITIZER_MAX_INPUT_BYTES", 1048576)),
		RedactEmails:       envBool("SANITIZER_REDACT_EMAILS", false),
		RedactPhones:       envBool("SANITIZER_REDACT_PHONES", false),
		RedactCertificates: envBool("SANITIZER_REDACT_CERTIFICATES", false),
		RedactUsernames:    envBool("SANITIZER_REDACT_USERNAMES", false),
		RedactResourceNames: envBool(
			"SANITIZER_REDACT_RESOURCE_NAMES",
			false,
		),
		IPMode:            envOr("SANITIZER_IP_MODE", "none"),
		HashKey:           strings.TrimSpace(os.Getenv("SANITIZER_HASH_KEY")),
		AuthSharedSecret:  strings.TrimSpace(os.Getenv("SANITIZER_AUTH_SHARED_SECRET")),
		AuthMaxClockSkew:  envDuration("SANITIZER_AUTH_MAX_CLOCK_SKEW", 60*time.Second),
		MaxCustomPatterns: envInt("SANITIZER_MAX_CUSTOM_PATTERNS", 50),
		MaxPatternLength:  envInt("SANITIZER_MAX_PATTERN_LENGTH", 500),
		RequestTimeout:    envDuration("SANITIZER_REQUEST_TIMEOUT", 3*time.Second),
		ShutdownTimeout:   envDuration("SANITIZER_SHUTDOWN_TIMEOUT", 10*time.Second),
		Version:           envOr("SANITIZER_VERSION", "1.0.0"),
		Commit:            envOr("SANITIZER_COMMIT", "dev"),
		BuildTime:         envOr("SANITIZER_BUILD_TIME", "unknown"),
	}
	if err := json.Unmarshal([]byte(envOr("SANITIZER_CUSTOM_PATTERNS", "[]")), &cfg.CustomPatterns); err != nil {
		return cfg, fmt.Errorf("parse SANITIZER_CUSTOM_PATTERNS: %w", err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.MaxDepth < 1 {
		return fmt.Errorf("SANITIZER_MAX_DEPTH must be positive")
	}
	if c.MaxInputBytes < 1 {
		return fmt.Errorf("SANITIZER_MAX_INPUT_BYTES must be positive")
	}
	switch c.IPMode {
	case "none", "partial", "full":
	default:
		return fmt.Errorf("SANITIZER_IP_MODE must be none, partial or full")
	}
	if c.RedactResourceNames && c.HashKey == "" {
		return fmt.Errorf("SANITIZER_HASH_KEY is required when resource pseudonymization is enabled")
	}
	if c.AuthSharedSecret == "" {
		return fmt.Errorf("SANITIZER_AUTH_SHARED_SECRET is required")
	}
	if len(c.CustomPatterns) > c.MaxCustomPatterns {
		return fmt.Errorf("too many custom patterns")
	}
	nameRE := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
	for _, p := range c.CustomPatterns {
		if !nameRE.MatchString(p.Name) {
			return fmt.Errorf("invalid custom pattern name")
		}
		if p.Pattern == "" || len(p.Pattern) > c.MaxPatternLength {
			return fmt.Errorf("invalid custom pattern length")
		}
		if strings.TrimSpace(p.Replacement) == "" || strings.ContainsAny(p.Replacement, "\r\n") {
			return fmt.Errorf("invalid custom pattern replacement")
		}
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return fmt.Errorf("compile custom pattern %s: %w", p.Name, err)
		}
		if re.MatchString("") {
			return fmt.Errorf("custom pattern %s matches empty string", p.Name)
		}
	}
	return nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return def
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func envInt(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return d
}
