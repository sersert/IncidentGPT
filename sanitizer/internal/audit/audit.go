package audit

import (
	"encoding/json"
	"log"
	"sort"
	"time"

	"incidentgpt-sanitizer/internal/domain"
)

type Event struct {
	Timestamp      string   `json:"timestamp"`
	RequestID      string   `json:"request_id"`
	Client         string   `json:"client"`
	Destination    string   `json:"destination"`
	Decision       string   `json:"decision"`
	RedactionCount int      `json:"redaction_count"`
	Rules          []string `json:"rules"`
	PolicyVersion  string   `json:"policy_version"`
}

func Log(requestID, client, destination, decision string, summary domain.Summary) {
	rules := make([]string, 0, len(summary.AppliedRules))
	for rule := range summary.AppliedRules {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	event := Event{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		RequestID:      requestID,
		Client:         client,
		Destination:    destination,
		Decision:       decision,
		RedactionCount: summary.RedactionCount,
		Rules:          rules,
		PolicyVersion:  "v1",
	}
	b, _ := json.Marshal(event)
	log.Printf("AUDIT: %s", string(b))
}
