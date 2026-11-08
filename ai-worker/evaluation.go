package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type evaluationRequestKey struct{}

func evaluationProfiles() ([]EvaluationProfile, error) {
	if os.Getenv("EVALUATION_ENABLED") != "true" {
		return []EvaluationProfile{}, nil
	}
	raw := os.Getenv("EVALUATION_PROFILES_JSON")
	if len(raw) > 16384 {
		return nil, fmt.Errorf("profile configuration exceeds budget")
	}
	var profiles []EvaluationProfile
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&profiles) != nil || d.Decode(&struct{}{}) != io.EOF || len(profiles) < 1 || len(profiles) > 16 {
		return nil, fmt.Errorf("invalid evaluation profiles")
	}
	seen := map[string]bool{}
	for i := range profiles {
		p := &profiles[i]
		if p.PromptVersion == "" {
			p.PromptVersion = evidencePromptVersion
		}
		if p.MaxTokens == 0 {
			p.MaxTokens = 2000
		}
		p.Revision = profileRevision(*p)
		if validateEvaluationProfile(*p) != nil || seen[p.ID] {
			return nil, fmt.Errorf("invalid evaluation profiles")
		}
		seen[p.ID] = true
	}
	return profiles, nil
}
func evaluationProfilesHandler(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("ANALYSIS_SERVICE_TOKEN")
	if len(token) < 32 || r.Header.Get("Origin") != "" || !constantToken(r.Header.Get("Authorization"), "Bearer "+token) {
		http.Error(w, "unauthorized", 401)
		return
	}
	profiles, err := evaluationProfiles()
	if err != nil {
		http.Error(w, "evaluation profiles unavailable", 503)
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": os.Getenv("EVALUATION_ENABLED") == "true", "profiles": profiles})
}
func decodeEvaluationRequest(raw []byte) (AnalysisInput, *EvaluationProfile, error) {
	if os.Getenv("EVALUATION_ENABLED") != "true" {
		return AnalysisInput{}, nil, fmt.Errorf("evaluation disabled")
	}
	var req EvaluationRequest
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&req) != nil || d.Decode(&struct{}{}) != io.EOF || validateEvaluationInput(req.Input) != nil {
		return AnalysisInput{}, nil, fmt.Errorf("invalid evaluation request")
	}
	profiles, err := evaluationProfiles()
	if err != nil {
		return AnalysisInput{}, nil, err
	}
	for _, p := range profiles {
		if p.ID == req.Profile.ID && p == req.Profile {
			return req.Input, &p, nil
		}
	}
	return AnalysisInput{}, nil, fmt.Errorf("evaluation profile changed or is not allowed")
}
