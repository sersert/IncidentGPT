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
