package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
)

// Server-defined profiles never contain credentials or provider endpoints.
type EvaluationProfile struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
	MaxTokens     int    `json:"max_tokens"`
	Revision      string `json:"revision"`
}
type EvaluationRequest struct {
	Profile EvaluationProfile `json:"profile"`
	Input   AnalysisInput     `json:"input"`
}

var evaluationName = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)
var evaluationModel = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,199}$`)

func profileRevision(p EvaluationProfile) string {
	p.Revision = ""
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func validateEvaluationProfile(p EvaluationProfile) error {
	if !evaluationName.MatchString(p.ID) || len(p.Label) == 0 || len(p.Label) > 100 || !evaluationModel.MatchString(p.Model) || p.MaxTokens < 64 || p.MaxTokens > 4000 || (p.PromptVersion != "evidence-analysis/v2" && p.PromptVersion != "evidence-analysis/alternatives-v1") || p.Revision != profileRevision(p) {
		return fmt.Errorf("invalid evaluation profile")
	}
	return nil
}
func evaluationObservation(source string) bool {
	switch source {
	case "alert", "kubernetes", "metrics", "logs", "pod_termination", "node_conditions", "workload_rollout", "pod_events", "pod_restarts", "test_case":
		return true
	}
	return false
}
func validateEvaluationInput(input AnalysisInput) error {
	if input.Version != 1 || input.IncidentID == "" || input.AnalysisID == "" || len(input.Evidence) == 0 || len(input.Evidence) > 200 || len(input.Context) > 0 || input.Operation != nil || input.Report != nil || input.PreviousID != "" || input.IngestKey != "" || input.JobKind != "" {
		return fmt.Errorf("evaluation requires frozen observations only")
	}
	ids := map[string]bool{}
	for _, e := range input.Evidence {
		if e.Version != 1 || e.ID == "" || ids[e.ID] || e.IncidentID != input.IncidentID || !evaluationObservation(e.Source) || len(e.Excerpt) > 8192 {
			return fmt.Errorf("invalid evaluation observation")
		}
		ids[e.ID] = true
	}
	raw, _ := json.Marshal(input)
	if len(raw) > 128<<10 {
		return fmt.Errorf("evaluation snapshot exceeds budget")
	}
	return nil
}

// Only bounded provider-reported numbers are metadata. Never restore arbitrary
// strings redacted by the sanitizer, or infer missing usage/cost as zero.
func evaluationUsage(usage map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens", "cost"} {
		value, ok := usage[key].(float64)
		if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			continue
		}
		if key == "cost" {
			if value > 1e6 {
				continue
			}
		} else if value > 1e9 || math.Trunc(value) != value {
			continue
		}
		result[key] = value
	}
	return result
}
