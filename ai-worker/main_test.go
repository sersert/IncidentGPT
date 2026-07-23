package main

import (
	"testing"
	"time"
)

func TestLoadConfigReadsOpenRouterTimeoutSeconds(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")
	t.Setenv("TELEGRAM_CHANNEL_ID", "-100123")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-token")
	t.Setenv("OPENROUTER_TIMEOUT_SECONDS", "42")

	cfg := loadConfig()

	if cfg.ORTimeout != 42*time.Second {
		t.Fatalf("ORTimeout = %s, want 42s", cfg.ORTimeout)
	}
}

func TestOpenRouterBackgroundContextUsesConfiguredTimeout(t *testing.T) {
	appCfg.ORTimeout = 7 * time.Second

	ctx, cancel := openRouterBackgroundContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("openRouterBackgroundContext() has no deadline")
	}

	got := time.Until(deadline)
	if got < 6*time.Second || got > 7*time.Second {
		t.Fatalf("openRouterBackgroundContext() timeout = %s, want about 7s", got)
	}
}
