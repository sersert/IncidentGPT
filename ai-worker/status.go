package main

import (
	"net/http"
	"net/url"
)

type dependencyCheck struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	checks := map[string]dependencyCheck{
		"config":          {OK: appCfg.TGBotToken != "" && appCfg.TGChannelID != "" && appCfg.ORAPIKey != ""},
		"openrouter_url":  {OK: validHTTPURL(appCfg.ORBaseURL)},
		"telegram_config": {OK: appCfg.TGBotToken != "" && appCfg.TGChannelID != ""},
	}
	for name, check := range checks {
		if check.OK {
			continue
		}
		check.Error = "not configured"
		if name == "openrouter_url" {
			check.Error = "invalid url"
		}
		checks[name] = check
	}

	ready := true
	for _, check := range checks {
		if !check.OK {
			ready = false
			break
		}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]interface{}{
		"service": "ai-worker",
		"ready":   ready,
		"checks":  checks,
	})
}

func statusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": "ai-worker",
		"openrouter": map[string]interface{}{
			"base_url_configured": appCfg.ORBaseURL != "",
			"model":               appCfg.ORModel,
			"timeout":             appCfg.ORTimeout.String(),
			"max_tokens":          appCfg.ORMaxTokens,
		},
		"telegram": map[string]interface{}{
			"channel_configured":     appCfg.TGChannelID != "",
			"thread_chat_configured": appCfg.TGThreadChatID != "",
			"parse_mode":             appCfg.TGParseMode,
		},
		"runtime": map[string]interface{}{
			"active_llm_slots": len(llmSemaphore),
		},
	})
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme != "" && u.Host != ""
}
