package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const evaluationTTL = 90 * 24 * time.Hour
const evaluationDue = "igpt:evaluation:due"
const evaluationCases = "igpt:evaluation:cases"
const evaluationRuns = "igpt:evaluation:runs"

type EvaluationReference struct {
	Cause    string `json:"cause"`
	Kind     string `json:"kind"`
	Actor    string `json:"actor"`
	Revision int    `json:"resolution_revision,omitempty"`
}
type EvaluationCase struct {
	ID                string              `json:"case_id"`
	Name              string              `json:"name"`
	Source            string              `json:"source"`
	SourceIncident    string              `json:"source_incident_id,omitempty"`
	SourceAnalysis    string              `json:"source_analysis_id,omitempty"`
	SourceCompletedAt *time.Time          `json:"source_completed_at,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	Actor             string              `json:"created_by"`
	SnapshotHash      string              `json:"snapshot_hash"`
	Input             AnalysisInput       `json:"input"`
	Reference         EvaluationReference `json:"reference"`
}
type EvaluationRun struct {
	ID          string              `json:"run_id"`
	Case        EvaluationCase      `json:"test_case"`
	Profiles    []EvaluationProfile `json:"profiles"`
	CreatedAt   time.Time           `json:"created_at"`
	Actor       string              `json:"created_by"`
	RequestHash string              `json:"request_hash"`
}
type EvaluationReview struct {
	Revision int       `json:"revision"`
	Verdict  string    `json:"verdict"`
	Note     string    `json:"note"`
	Actor    string    `json:"reviewed_by"`
	At       time.Time `json:"reviewed_at"`
}
type EvaluationJob struct {
	ID         string            `json:"job_id"`
	RunID      string            `json:"run_id"`
	Profile    EvaluationProfile `json:"profile"`
	Input      AnalysisInput     `json:"input"`
	Status     string            `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Result     *AnalysisVersion  `json:"result,omitempty"`
	ErrorCode  string            `json:"error_code,omitempty"`
	Review     EvaluationReview  `json:"review"`
	Owner      string            `json:"owner,omitempty"`
	Lease      int64             `json:"lease_until,omitempty"`
}

func evaluationEnabled() bool { return env("EVALUATION_ENABLED", "false") == "true" }

var evaluationHTTP = &http.Client{Timeout: 55 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func evaluationEndpoint(path string) (string, error) {
	u, err := url.Parse(env("STRUCTURED_WORKER_URL", "http://ai-worker:8080/v1/analyze"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid evaluation worker configuration")
	}
	u.Path = "/v1/evaluation/" + path
	u.RawPath = ""
	return u.String(), nil
}
func evaluationRequest(ctx context.Context, method, path string, payload any) ([]byte, int, error) {
	endpoint, err := evaluationEndpoint(path)
	if err != nil {
		return nil, 0, err
	}
	token := env("ANALYSIS_SERVICE_TOKEN", "")
	if len(token) < 32 {
		return nil, 0, fmt.Errorf("evaluation service credential required")
	}
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := evaluationHTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("evaluation worker unavailable")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (256<<10)+1))
	if err != nil || len(raw) > 256<<10 {
		return nil, resp.StatusCode, fmt.Errorf("evaluation response exceeds budget")
	}
	return raw, resp.StatusCode, nil
}
func configuredEvaluationProfiles(ctx context.Context) ([]EvaluationProfile, error) {
	raw, status, err := evaluationRequest(ctx, "GET", "profiles", nil)
	var out struct {
		Enabled  bool                `json:"enabled"`
		Profiles []EvaluationProfile `json:"profiles"`
	}
	if err != nil || status != 200 || json.Unmarshal(raw, &out) != nil || !out.Enabled || len(out.Profiles) < 1 || len(out.Profiles) > 16 {
		return nil, fmt.Errorf("evaluation profiles unavailable")
	}
	seen := map[string]bool{}
	for _, p := range out.Profiles {
		if validateEvaluationProfile(p) != nil || seen[p.ID] {
			return nil, fmt.Errorf("invalid worker profile")
		}
		seen[p.ID] = true
	}
	return out.Profiles, nil
}
func evaluationAuth(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == "GET" {
		if !sharedAdminAuth(w, r) {
			return false
		}
	} else {
		if !sharedAdminMutationAuth(w, r) {
			return false
		}
	}
	if !evaluationEnabled() {
		http.Error(w, "model evaluation is disabled", 404)
		return false
	}
	if store.disk == nil {
		http.Error(w, "evaluation requires persistent Redis", 503)
		return false
	}
	return true
}
func handleEvaluationConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !sharedAdminAuth(w, r) {
		return
	}
	if !evaluationEnabled() {
		writeJSON(w, 200, map[string]any{"enabled": false, "profiles": []EvaluationProfile{}})
		return
	}
	if store.disk == nil {
		http.Error(w, "evaluation requires persistent Redis", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	profiles, err := configuredEvaluationProfiles(ctx)
	if err != nil {
		writeJSON(w, 200, map[string]any{"enabled": true, "profiles": []EvaluationProfile{}, "profile_status": "unavailable", "max_profiles_per_run": 4, "retention_days": 90})
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": true, "profiles": profiles, "max_profiles_per_run": 4, "retention_days": 90})
}
func evaluationSnapshotHash(input AnalysisInput) string {
	input.AnalysisID = "snapshot"
	raw, _ := json.Marshal(input)
	return digest(string(raw))
}
func loadEvaluationCase(ctx context.Context, id string) (EvaluationCase, error) {
	var c EvaluationCase
	raw, err := store.disk.rdb.Get(ctx, "igpt:evaluation:case:"+id).Bytes()
	if err != nil {
		return c, err
	}
	if json.Unmarshal(raw, &c) != nil || c.ID != id || validateEvaluationInput(c.Input) != nil || c.SnapshotHash != evaluationSnapshotHash(c.Input) {
		return c, fmt.Errorf("evaluation snapshot integrity failure")
	}
	return c, nil
}
func caseSummary(c EvaluationCase) map[string]any {
	return map[string]any{"case_id": c.ID, "name": c.Name, "source": c.Source, "source_incident_id": c.SourceIncident, "source_analysis_id": c.SourceAnalysis, "created_at": c.CreatedAt, "created_by": c.Actor, "snapshot_hash": c.SnapshotHash, "evidence_count": len(c.Input.Evidence), "has_reference": c.Reference.Cause != ""}
}

var saveEvaluationCaseScript = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',ARGV[1])
if redis.call('ZCARD',KEYS[1])>=200 then return 0 end
redis.call('SET',KEYS[2],ARGV[4],'EX',ARGV[5]);redis.call('ZADD',KEYS[1],ARGV[2],ARGV[3]);return 1`)

func handleEvaluationCases(w http.ResponseWriter, r *http.Request) {
	if !evaluationAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if r.Method == "GET" {
		offset, limit := intQuery(r, "offset", 0), intQuery(r, "limit", 25)
		if offset < 0 || offset > 200 || limit < 1 || limit > 50 {
			http.Error(w, "invalid pagination", 400)
			return
		}
		ids, err := store.disk.rdb.ZRevRangeByScore(ctx, evaluationCases, &redis.ZRangeBy{Min: fmt.Sprint(time.Now().Add(-evaluationTTL).UnixMilli()), Max: "+inf", Offset: int64(offset), Count: int64(limit)}).Result()
		if err != nil {
			http.Error(w, "case library unavailable", 503)
			return
		}
		items := []map[string]any{}
		for _, id := range ids {
			c, err := loadEvaluationCase(ctx, id)
			if err == redis.Nil {
				continue
			}
			if err != nil {
				http.Error(w, "case snapshot unavailable", 503)
				return
			}
			items = append(items, caseSummary(c))
		}
		writeJSON(w, 200, map[string]any{"items": items, "offset": offset, "next_offset": offset + len(ids), "has_more": len(ids) == limit})
		return
	}
	var request struct {
		Name          string `json:"name"`
		IncidentID    string `json:"incident_id"`
		AnalysisID    string `json:"analysis_id"`
		Scenario      string `json:"scenario"`
		ExpectedCause string `json:"expected_cause"`
	}
	if decodeChange(r, &request) != nil || len(strings.TrimSpace(request.Name)) == 0 || len(request.Name) > 160 || len(request.Scenario) > 8192 || (request.Scenario != "" && strings.TrimSpace(request.Scenario) == "") || len(request.ExpectedCause) > 4000 {
		http.Error(w, "invalid bounded test case", 400)
		return
	}
	actor, _, _ := r.BasicAuth()
	if len(actor) > 200 {
		http.Error(w, "invalid actor", 400)
		return
	}
	c := EvaluationCase{ID: newStableID("case"), Name: request.Name, CreatedAt: time.Now().UTC(), Actor: actor, Reference: EvaluationReference{Cause: request.ExpectedCause, Kind: "engineer_reference", Actor: actor}}
	if request.Scenario != "" {
		if request.IncidentID != "" || request.AnalysisID != "" {
			http.Error(w, "choose an incident snapshot or a custom scenario", 400)
			return
		}
		c.Source = "manual"
		c.Input = AnalysisInput{Version: 1, IncidentID: c.ID, AnalysisID: "snapshot", ClusterID: "synthetic", Composition: digest(c.ID), Evidence: []Evidence{{Version: 1, ID: "test-" + digest(c.ID), IncidentID: c.ID, ClusterID: "synthetic", Source: "test_case", Availability: "available", Excerpt: request.Scenario, IngestedAt: c.CreatedAt, CollectorVersion: "engineer-scenario/v1"}}}
	} else {
		c.Source = "incident"
		inc := store.Get(request.IncidentID)
		if inc == nil {
			http.Error(w, "source incident not found", 404)
			return
		}
		raw, err := store.disk.rdb.Get(ctx, analysisKey(request.AnalysisID)).Bytes()
		var a AnalysisVersion
		if err != nil || json.Unmarshal(raw, &a) != nil || a.ID != request.AnalysisID || a.IncidentID != inc.ID || a.Status != "complete" || len(a.EvidenceIDs) == 0 || len(a.EvidenceIDs) > 200 {
			http.Error(w, "select a retained structured analysis belonging to the incident", 400)
			return
		}
		c.SourceIncident, c.SourceAnalysis = inc.ID, a.ID
		c.SourceCompletedAt = &a.CompletedAt
		c.Input = AnalysisInput{Version: 1, IncidentID: inc.ID, AnalysisID: "snapshot", ClusterID: incidentCluster(inc), Composition: a.Composition}
		for _, eid := range a.EvidenceIDs {
			raw, err := store.disk.rdb.Get(ctx, "igpt:evidence:"+eid).Bytes()
			var e Evidence
			if err != nil || json.Unmarshal(raw, &e) != nil || e.ID != eid || e.IncidentID != inc.ID {
				http.Error(w, "source observation unavailable", 503)
				return
			}
			if evaluationObservation(e.Source) && e.Source != "test_case" {
				c.Input.Evidence = append(c.Input.Evidence, e)
			}
		}
		if request.ExpectedCause == "" {
			raw, err := store.disk.rdb.Get(ctx, "igpt:resolution:"+inc.ID).Bytes()
			var resolution Resolution
			if err != nil && err != redis.Nil {
				http.Error(w, "reference unavailable", 503)
				return
			}
			if err == nil {
				if json.Unmarshal(raw, &resolution) != nil {
					http.Error(w, "reference invalid", 503)
					return
				}
				if resolution.Status == "confirmed" && resolution.Composition != "" && resolution.Composition == a.Composition {
					c.Reference = EvaluationReference{Cause: resolution.Cause, Kind: "confirmed_resolution", Actor: resolution.Actor, Revision: resolution.Revision}
				}
			}
		}
	}
	if len(c.Input.Evidence) == 0 {
		http.Error(w, "no retained observations; knowledge and previous analyses are excluded", 400)
		return
	}
	safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", c)
	var clean EvaluationCase
	if err != nil || json.Unmarshal(safe, &clean) != nil || clean.ID != c.ID || clean.SourceIncident != c.SourceIncident || clean.SourceAnalysis != c.SourceAnalysis || clean.Input.IncidentID != c.Input.IncidentID || validateEvaluationInput(clean.Input) != nil {
		http.Error(w, "case sanitization failed", 503)
		return
	}
	clean.Actor = c.Actor
	clean.CreatedAt = c.CreatedAt
	clean.Reference.Actor = c.Reference.Actor
	clean.SnapshotHash = evaluationSnapshotHash(clean.Input)
	raw, _ := json.Marshal(clean)
	n, err := saveEvaluationCaseScript.Run(ctx, store.disk.rdb, []string{evaluationCases, "igpt:evaluation:case:" + c.ID}, time.Now().Add(-evaluationTTL).UnixMilli(), c.CreatedAt.UnixMilli(), c.ID, raw, int(evaluationTTL.Seconds())).Int()
	if err != nil {
		http.Error(w, "case persistence unavailable", 503)
		return
	}
	if n != 1 {
		http.Error(w, "case library limit reached (200); wait for retention expiry", 409)
		return
	}
	writeJSON(w, 201, clean)
}
func handleEvaluationCase(w http.ResponseWriter, r *http.Request) {
	if !evaluationAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	c, err := loadEvaluationCase(ctx, r.PathValue("case"))
	if err == redis.Nil {
		http.Error(w, "test case not found or expired", 404)
		return
	}
	if err != nil {
		http.Error(w, "case unavailable", 503)
		return
	}
	writeJSON(w, 200, c)
}
