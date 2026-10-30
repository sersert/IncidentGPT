package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestIncidentScopeUsesStoredResources(t *testing.T) {
	inc := &Incident{ID: "INC-1", ClusterID: "prod", Namespace: "payments", Labels: map[string]string{"deployment": "api"}, Alerts: []*Alert{{Labels: map[string]string{"pod": "api-1", "node": "worker-1"}}}}
	scope := incidentDiagnosticScope(inc)
	if scope.ClusterID != "prod" || scope.Pod != "api-1" || scope.Node != "worker-1" || scope.Workload != "api" || scope.To.Sub(scope.From) != 15*time.Minute {
		t.Fatal("scope not bound to incident")
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"action":"refresh","scope":{"pod":"foreign"}}`))
	var request struct {
		Action string `json:"action"`
	}
	if decodeChange(r, &request) == nil {
		t.Fatal("caller scope override accepted")
	}
}
func TestDiagnosticRedisCancellationAndRecovery(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires dedicated Redis")
	}
	old := store
	defer func() { store = old }()
	disk := NewStorage(addr, "")
	if disk == nil {
		t.Fatal("Redis unavailable")
	}
	defer disk.rdb.Close()
	store = &Store{disk: disk}
	ctx := context.Background()
	disk.rdb.Del(ctx, jobsDue)
	now := time.Now().UTC()
	inc := &Incident{ID: newStableID("fixture"), ClusterID: "prod", Namespace: "payments", CreatedAt: now}
	store.Put(inc)
	input := AnalysisInput{Version: 1, IncidentID: inc.ID, AnalysisID: newStableID("analysis"), JobKind: "diagnostics", Operation: &DiagnosticOperation{ID: newStableID("job"), IncidentID: inc.ID, Action: "refresh", Scope: DiagnosticScope{ClusterID: "prod", Namespace: "payments", Pod: "api-1", From: now.Add(-time.Minute), To: now}, Checks: []string{"pod_termination"}, Status: "queued"}}
	// The no-history path used to encode [] and fail after the first Lua claim.
	input.Evidence = []Evidence{}
	input.Operation.EvidenceIDs = []string{}
	input.Operation.ID = newStableID("job")
	jid := input.Operation.ID
	if _, err := enqueueInput(ctx, inc, input, jid); err != nil {
		t.Fatal(err)
	}
	another := input
	another.AnalysisID = newStableID("analysis")
	if _, err := enqueueInput(ctx, inc, another, newStableID("job")); err == nil {
		t.Fatal("per-incident budget bypassed")
	}
	e, err := claimJob(ctx)
	if err != nil || e == nil {
		t.Fatalf("recoverable diagnostic job: %v", err)
	}
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DiagnosticRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Scope.Pod != "api-1" || r.Header.Get("Authorization") != "Bearer token" {
			t.Error("trusted resource/service scope missing")
		}
		at := time.Now().UTC()
		writeJSON(w, 200, DiagnosticResult{Status: "partial", CollectedAt: at, Evidence: []Evidence{{Version: 1, ID: "diagnostic-" + req.OperationID + "-pod_termination", IncidentID: inc.ID, ClusterID: "prod", Source: "pod_termination", Availability: "available", Excerpt: "OOMKilled", CollectedAt: &at}}})
	}))
	defer collector.Close()
	t.Setenv("DIAGNOSTIC_COLLECTOR_URL", collector.URL)
	t.Setenv("ANALYSIS_SERVICE_TOKEN", "token")
	if err := collectDiagnosticEvidence(ctx, e); err != nil {
		t.Fatal(err)
	}
	if e.Input.Operation.Status != "partial" || len(e.Input.Evidence) != 1 {
		t.Fatal("partial evidence lost")
	}
	// Persisted collection is reusable after worker recovery, without presenting
	// old evidence as a new collection or repeating a failed source query.
	collector.Close()
	if err := finishJob(ctx, e, nil, "provider_unavailable"); err != nil {
		t.Fatal(err)
	}
	disk.rdb.ZAdd(ctx, jobsDue, redis.Z{Score: float64(time.Now().UnixMilli()), Member: jid})
	e, err = claimJob(ctx)
	if err != nil || e == nil || e.Job.Attempts != 2 {
		t.Fatalf("persisted diagnostic recovery: %v", err)
	}
	if err := collectDiagnosticEvidence(ctx, e); err != nil {
		t.Fatal("recovery recollected completed observations")
	}
	n, err := cancelDiagnosticScript.Run(ctx, disk.rdb, []string{"igpt:job:" + jid, jobsDue, "igpt:diagnostics:active:" + inc.ID}, inc.ID, time.Now().UTC().Format(time.RFC3339Nano)).Int()
	if err != nil || n != 1 {
		t.Fatal("cancellation failed")
	}
	a := &AnalysisVersion{Version: 1, ID: e.Job.AnalysisID, IncidentID: inc.ID, Status: "complete", Summary: "late result"}
	if finishJob(ctx, e, a, "") == nil {
		t.Fatal("canceled job published analysis")
	}
	if disk.rdb.Exists(ctx, "igpt:evidence:"+e.Input.Evidence[0].ID).Val() != 1 {
		t.Fatal("cancellation erased earlier observations")
	}
}

func diagnosticTestStore(t *testing.T) (*Storage, *Incident) {
	t.Helper()
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires dedicated Redis")
	}
	old := store
	disk := NewStorage(addr, "")
	if disk == nil {
		t.Fatal("Redis unavailable")
	}
	store = &Store{disk: disk}
	t.Cleanup(func() { store = old; disk.rdb.Close() })
	disk.rdb.Del(context.Background(), jobsDue)
	inc := &Incident{ID: newStableID("fixture"), ClusterID: "prod", Namespace: "payments", Labels: map[string]string{"deployment": "api"}, CreatedAt: time.Now().UTC()}
	store.Put(inc)
	return disk, inc
}

func TestDiagnosticEmptySlicesSurviveJobLifecycle(t *testing.T) {
	for _, terminal := range []string{"complete", "canceled"} {
		t.Run(terminal, func(t *testing.T) {
			disk, inc := diagnosticTestStore(t)
			ctx := context.Background()
			memory, err := confirmedMemoryEvidence(ctx, inc)
			if err != nil || memory != nil {
				t.Fatalf("no history must return nil evidence: %v", err)
			}
			jid := newStableID("job")
			input := AnalysisInput{Version: 1, IncidentID: inc.ID, AnalysisID: newStableID("analysis"), JobKind: "diagnostics", Evidence: []Evidence{}, Context: []string{}, Operation: &DiagnosticOperation{ID: jid, IncidentID: inc.ID, Action: "refresh", Checks: []string{}, EvidenceIDs: []string{}}}
			if _, err := enqueueInput(ctx, inc, input, jid); err != nil {
				t.Fatal(err)
			}
			e, err := claimJob(ctx)
			if err != nil || e == nil {
				t.Fatalf("claim empty slices: %v", err)
			}
			if err := finishJob(ctx, e, nil, "fixture_retry"); err != nil {
				t.Fatal(err)
			}
			disk.rdb.ZAdd(ctx, jobsDue, redis.Z{Score: float64(time.Now().UnixMilli()), Member: jid})
			e, err = claimJob(ctx)
			if err != nil || e == nil || e.Job.Attempts != 2 {
				t.Fatalf("retry empty slices: %v", err)
			}
			if terminal == "complete" {
				if err := finishJob(ctx, e, &AnalysisVersion{ID: input.AnalysisID, IncidentID: inc.ID, Summary: "fixture"}, ""); err != nil {
					t.Fatal(err)
				}
			} else {
				n, err := cancelDiagnosticScript.Run(ctx, disk.rdb, []string{"igpt:job:" + jid, jobsDue, "igpt:diagnostics:active:" + inc.ID}, inc.ID, time.Now().UTC().Format(time.RFC3339Nano)).Int()
				if err != nil || n != 1 {
					t.Fatalf("cancel: %v", err)
				}
			}
			var stored jobEnvelope
			raw, err := disk.rdb.Get(ctx, "igpt:job:"+jid).Bytes()
			if err != nil || json.Unmarshal(raw, &stored) != nil || stored.Job.Status != terminal {
				t.Fatalf("terminal job must remain readable: %s, %v", raw, err)
			}
		})
	}
}

func TestDiagnosticQuestionUnicodeAndFeatureState(t *testing.T) {
	disk, inc := diagnosticTestStore(t)
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "test-password")
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "true")
	t.Setenv("GUIDED_DIAGNOSTICS_ENABLED", "true")
	for _, question := range []string{strings.Repeat("я", 500), strings.Repeat("🔎", 500), strings.Repeat("я", 501), strings.Repeat("🔎", 501)} {
		body, _ := json.Marshal(map[string]string{"action": "hypothesis", "question": question})
		r := httptest.NewRequest("POST", "/", strings.NewReader(string(body)))
		r.SetPathValue("id", inc.ID)
		r.SetBasicAuth("admin", "test-password")
		w := httptest.NewRecorder()
		handleInvestigations(w, r)
		if len([]rune(question)) > 500 {
			if w.Code != 400 {
				t.Fatalf("overlong question: %d %s", w.Code, w.Body.String())
			}
			continue
		}
		if w.Code != 202 {
			t.Fatalf("500 Unicode characters rejected: %d %s", w.Code, w.Body.String())
		}
		var response struct {
			JobID string `json:"job_id"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		n, err := cancelDiagnosticScript.Run(context.Background(), disk.rdb, []string{"igpt:job:" + response.JobID, jobsDue, "igpt:diagnostics:active:" + inc.ID}, inc.ID, time.Now().UTC().Format(time.RFC3339Nano)).Int()
		if err != nil || n != 1 {
			t.Fatalf("release active job: %v", err)
		}
	}
	for _, flags := range []struct {
		durable, guided string
		enabled         bool
	}{{"true", "true", true}, {"true", "false", false}, {"false", "true", false}} {
		t.Setenv("DURABLE_ANALYSIS_ENABLED", flags.durable)
		t.Setenv("GUIDED_DIAGNOSTICS_ENABLED", flags.guided)
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", inc.ID)
		r.SetBasicAuth("admin", "test-password")
		w := httptest.NewRecorder()
		handleInvestigations(w, r)
		var response struct {
			Enabled bool `json:"diagnostics_enabled"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Enabled != flags.enabled {
			t.Fatalf("feature state: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestDiagnosticExpiredWorkContextStillRecordsRetry(t *testing.T) {
	disk, inc := diagnosticTestStore(t)
	ctx := context.Background()
	jid := newStableID("job")
	input := AnalysisInput{Version: 1, IncidentID: inc.ID, AnalysisID: newStableID("analysis"), JobKind: "diagnostics", Operation: &DiagnosticOperation{ID: jid, IncidentID: inc.ID, Action: "refresh", Checks: []string{"logs"}}}
	if _, err := enqueueInput(ctx, inc, input, jid); err != nil {
		t.Fatal(err)
	}
	e, err := claimJob(ctx)
	if err != nil || e == nil {
		t.Fatalf("claim: %v", err)
	}
	workCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := runAnalysisJob(workCtx, e); err != nil {
		t.Fatal(err)
	}
	raw, err := disk.rdb.Get(ctx, "igpt:job:"+jid).Bytes()
	var stored jobEnvelope
	if err != nil || json.Unmarshal(raw, &stored) != nil || stored.Job.Status != "queued" || stored.Job.ErrorCode != "diagnostic_collector_unavailable" {
		t.Fatalf("expired work did not schedule retry: %s %v", raw, err)
	}
}

func TestDiagnosticActorStaysOutOfWorkerPayload(t *testing.T) {
	input := AnalysisInput{Version: 1, IncidentID: "INC-1", AnalysisID: "analysis-1", Evidence: []Evidence{{ID: "evidence-1", IncidentID: "INC-1"}}, Operation: &DiagnosticOperation{Actor: "private-admin-login"}}
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var received AnalysisInput
		if json.NewDecoder(r.Body).Decode(&received) != nil || received.Operation == nil || received.Operation.Actor != "" {
			t.Error("audit identity reached worker")
		}
		writeJSON(w, 200, AnalysisVersion{Version: 1, ID: input.AnalysisID, IncidentID: input.IncidentID, EvidenceIDs: []string{"evidence-1"}, Status: "complete", Summary: "fixture"})
	}))
	defer worker.Close()
	t.Setenv("STRUCTURED_WORKER_URL", worker.URL)
	if _, err := executeAnalysis(context.Background(), &jobEnvelope{Input: input}); err != nil {
		t.Fatal(err)
	}
	if input.Operation.Actor != "private-admin-login" {
		t.Fatal("worker payload filtering changed the stored audit identity")
	}
}
