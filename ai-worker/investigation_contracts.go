package main

import (
	"crypto/subtle"
	"fmt"
	"strings"
	"time"
)

// Shared v1 wire contracts. Observations are data, never tool instructions.
type Evidence struct {
	Version          int        `json:"schema_version"`
	ID               string     `json:"evidence_id"`
	IncidentID       string     `json:"incident_id"`
	Source           string     `json:"source_type"`
	ClusterID        string     `json:"cluster_id"`
	Resource         string     `json:"resource"`
	ObservedFrom     *time.Time `json:"observation_start,omitempty"`
	ObservedTo       *time.Time `json:"observation_end,omitempty"`
	CollectedAt      *time.Time `json:"collection_time,omitempty"`
	IngestedAt       time.Time  `json:"ingested_at"`
	Excerpt          string     `json:"excerpt"`
	SourceQuery      string     `json:"source_query,omitempty"`
	SourceURL        string     `json:"source_url,omitempty"`
	CollectorVersion string     `json:"collector_version"`
	Availability     string     `json:"availability"` // available, empty, failed, timed_out, disabled, unauthorized, unavailable
}
type Hypothesis struct {
	Text          string   `json:"text"`
	Supporting    []string `json:"supporting_evidence_ids"`
	Contradicting []string `json:"contradicting_evidence_ids"`
}
type AnalysisVersion struct {
	Version           int            `json:"schema_version"`
	ID                string         `json:"analysis_id"`
	IncidentID        string         `json:"incident_id"`
	EvidenceIDs       []string       `json:"input_evidence_ids"`
	Model             string         `json:"model"`
	Provider          string         `json:"provider"`
	PromptVersion     string         `json:"prompt_version"`
	StartedAt         time.Time      `json:"started_at"`
	CompletedAt       time.Time      `json:"completed_at"`
	Status            string         `json:"status"`
	Summary           string         `json:"summary"`
	Hypotheses        []Hypothesis   `json:"hypotheses"`
	Missing           []string       `json:"missing_information"`
	NextChecks        []string       `json:"suggested_next_checks"`
	HumanConfirmation string         `json:"human_confirmation"`
	PreviousID        string         `json:"previous_analysis_id,omitempty"`
	Composition       string         `json:"composition_hash"`
	Usage             map[string]any `json:"usage,omitempty"`
	Notification      string         `json:"notification_status,omitempty"`
}
type AnalysisInput struct {
	Report      *ReportSource        `json:"report_source,omitempty"`
	JobKind     string               `json:"job_kind,omitempty"`
	Operation   *DiagnosticOperation `json:"operation,omitempty"`
	IngestKey   string               `json:"ingest_key,omitempty"`
	Version     int                  `json:"schema_version"`
	IncidentID  string               `json:"incident_id"`
	AnalysisID  string               `json:"analysis_id"`
	ClusterID   string               `json:"cluster_id"`
	Evidence    []Evidence           `json:"evidence"`
	PreviousID  string               `json:"previous_analysis_id,omitempty"`
	Composition string               `json:"composition_hash"`
	Context     []string             `json:"confirmed_context,omitempty"`
}
type DurableJob struct {
	Version    int        `json:"schema_version"`
	ID         string     `json:"job_id"`
	IncidentID string     `json:"incident_id"`
	AnalysisID string     `json:"analysis_id,omitempty"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	Attempts   int        `json:"attempts"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ErrorCode  string     `json:"error_code,omitempty"`
}

func validateAnalysis(a AnalysisVersion, input AnalysisInput) error {
	if a.StartedAt.IsZero() || a.CompletedAt.IsZero() || a.CompletedAt.Before(a.StartedAt) {
		return fmt.Errorf("invalid analysis timestamps")
	}
	if a.Version != 1 || a.IncidentID != input.IncidentID || a.ID != input.AnalysisID || a.Status != "complete" || len(a.Summary) == 0 || len(a.Summary) > 4000 || len(a.Hypotheses) > 8 || len(a.Missing) > 20 || len(a.NextChecks) > 20 {
		return fmt.Errorf("invalid analysis contract")
	}
	allowed := map[string]bool{}
	for _, e := range input.Evidence {
		if e.IncidentID != input.IncidentID || len(e.Excerpt) > 8192 {
			return fmt.Errorf("invalid input evidence")
		}
		allowed[e.ID] = true
	}
	if len(a.EvidenceIDs) != len(allowed) {
		return fmt.Errorf("invalid analysis inputs")
	}
	used := map[string]bool{}
	for _, id := range a.EvidenceIDs {
		if !allowed[id] || used[id] {
			return fmt.Errorf("unknown analysis input")
		}
		used[id] = true
	}
	for _, h := range a.Hypotheses {
		if len(h.Text) > 4000 || len(h.Supporting) > 100 || len(h.Contradicting) > 100 {
			return fmt.Errorf("hypothesis exceeds budget")
		}
		for _, ids := range [][]string{h.Supporting, h.Contradicting} {
			for _, id := range ids {
				if !allowed[id] {
					return fmt.Errorf("unverified evidence reference")
				}
			}
		}
	}
	for _, items := range [][]string{a.Missing, a.NextChecks} {
		for _, s := range items {
			if len(s) > 2000 {
				return fmt.Errorf("analysis exceeds budget")
			}
		}
	}
	if strings.Contains(a.Provider, "?") || len(a.Model) > 200 {
		return fmt.Errorf("invalid provider metadata")
	}
	return nil
}

func constantToken(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

type DiagnosticScope struct {
	ClusterID string    `json:"cluster_id"`
	Namespace string    `json:"namespace"`
	Pod       string    `json:"pod"`
	Node      string    `json:"node"`
	Workload  string    `json:"workload"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
}
type DiagnosticOperation struct {
	ID          string          `json:"operation_id"`
	IncidentID  string          `json:"incident_id"`
	Actor       string          `json:"actor"`
	Action      string          `json:"action"`
	Question    string          `json:"question"`
	Scope       DiagnosticScope `json:"scope"`
	Checks      []string        `json:"checks"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	CollectedAt *time.Time      `json:"collected_at,omitempty"`
	EvidenceIDs []string        `json:"evidence_ids"`
}
type DiagnosticRequest struct {
	Version     int             `json:"schema_version"`
	IncidentID  string          `json:"incident_id"`
	OperationID string          `json:"operation_id"`
	Scope       DiagnosticScope `json:"scope"`
	Checks      []string        `json:"checks"`
}
type DiagnosticResult struct {
	Evidence    []Evidence `json:"evidence"`
	Status      string     `json:"status"`
	CollectedAt time.Time  `json:"collected_at"`
}

type ReportSource struct {
	Version            int       `json:"schema_version"`
	ReportID           string    `json:"report_id"`
	Kind               string    `json:"kind"`
	IncidentID         string    `json:"incident_id"`
	Composition        string    `json:"composition_hash"`
	CapturedAt         time.Time `json:"captured_at"`
	Actor              string    `json:"actor"`
	Markdown           string    `json:"source_markdown"`
	AnalysisID         string    `json:"analysis_id,omitempty"`
	EvidenceIDs        []string  `json:"evidence_ids,omitempty"`
	ChangeIDs          []string  `json:"change_event_ids,omitempty"`
	ResolutionRevision int       `json:"resolution_revision"`
	LifecycleRevision  int       `json:"lifecycle_revision"`
}
