package main

import (
	"context"
	"encoding/json"
	"fmt"
)

func sanitizeAlertForLLM(ctx context.Context, alert EnrichedAlert) (EnrichedAlert, error) {
	raw, err := sanitizerClient.SanitizePayload(ctx, "ai-worker", "llm", alert)
	if err != nil {
		return EnrichedAlert{}, err
	}
	var safe EnrichedAlert
	if err := json.Unmarshal(raw, &safe); err != nil {
		return EnrichedAlert{}, fmt.Errorf("decode sanitized alert")
	}
	return safe, nil
}

func sanitizeGroupForLLM(ctx context.Context, req GroupRequest) (GroupRequest, error) {
	raw, err := sanitizerClient.SanitizePayload(ctx, "ai-worker", "llm", req)
	if err != nil {
		return GroupRequest{}, err
	}
	var safe GroupRequest
	if err := json.Unmarshal(raw, &safe); err != nil {
		return GroupRequest{}, fmt.Errorf("decode sanitized group")
	}
	return safe, nil
}
