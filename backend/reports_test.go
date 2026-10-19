package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLifecycleDurationsDoNotInventRecovery(t *testing.T) {
	start := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	ack := start.Add(30 * time.Second)
	analysisStart := start.Add(time.Minute)
	analysisEnd := analysisStart.Add(2 * time.Second)
	closure := start.Add(20 * time.Minute)
	records := []metricIncident{{Incident: &Incident{}, Lifecycle: Lifecycle{FirstSymptom: &start, AlertReceipt: &start, Acknowledged: &ack, AnalysisStarted: &analysisStart, AnalysisCompleted: &analysisEnd, Closed: &closure}}}
	if got := metricValues(records, "analysis"); len(got) != 1 || got[0] != 2 {
		t.Fatal("AI latency conflated with incident duration", got)
	}
	if len(metricValues(records, "recovery")) != 0 || metricMean(metricValues(records, "recovery")) != nil {
		t.Fatal("missing recovery invented")
	}
	if metricValues(records, "closure")[0] != 1200 || metricValues(records, "ack")[0] != 30 {
		t.Fatal("event durations incorrect")
	}
	secondRecord := records[0]
	secondEnd := analysisStart.Add(100 * time.Second)
	secondRecord.Lifecycle.AnalysisCompleted = &secondEnd
	percentile := metricSummary(append(records, secondRecord), "analysis")["p90_seconds"].(*float64)
	if *percentile != 100 {
		t.Fatal("nearest-rank p90 incorrect for small samples")
	}
	zero := time.Time{}
	if elapsed(&zero, &analysisEnd) != nil {
		t.Fatal("unknown start represented as measured duration")
	}
	invalid := start.Add(-time.Second)
	if elapsed(&start, &invalid) != nil {
		t.Fatal("negative interval accepted")
	}
}
func TestOfflineIncidentWorkflow(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires disposable Redis; see deploy/test-compose.yaml")
	}
	var fixture struct {
		Namespace      string `json:"namespace"`
		Workload       string `json:"workload"`
		Revision       string `json:"revision"`
		Image          string `json:"image"`
		ChangeMinutes  int    `json:"change_minutes_before"`
		SymptomMinutes int    `json:"symptom_minutes_before"`
		Alert          string `json:"alert"`
		Observation    string `json:"observation"`
		Cause          string `json:"confirmed_cause"`
		Action         string `json:"restoring_action"`
		Failed         string `json:"failed_attempt"`
	}
	data, err := os.ReadFile("testdata/incident-workflow.json")
	if err != nil || json.Unmarshal(data, &fixture) != nil {
		t.Fatal("offline fixture unavailable")
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
	for key, value := range map[string]string{"DURABLE_ANALYSIS_ENABLED": "true", "GUIDED_DIAGNOSTICS_ENABLED": "true", "REPORTS_ENABLED": "true", "CHANGES_ENABLED": "true", "TIMELINE_READ_USER": "admin", "TIMELINE_READ_PASSWORD": "pw", "ANALYSIS_SERVICE_TOKEN": "service", "CHANGE_INGEST_TOKEN": "changes", "SANITIZER_AUTH_SHARED_SECRET": "hmac"} {
		t.Setenv(key, value)
	}
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-IncidentGPT-Signature") == "" {
			t.Error("sanitizer authentication missing")
		}
		var req sanitizerPayloadRequest
		json.NewDecoder(r.Body).Decode(&req)
		safe := strings.ReplaceAll(string(req.Payload), "fixture-secret", "[REDACTED]")
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": json.RawMessage(safe)})
	}))
	defer sanitizer.Close()
	t.Setenv("SANITIZER_URL", sanitizer.URL)
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer service" {
			t.Error("worker identity missing")
		}
		var input AnalysisInput
		json.NewDecoder(r.Body).Decode(&input)
		ids := []string(nil)
		for _, e := range input.Evidence {
			ids = append(ids, e.ID)
		}
		completed := time.Now().UTC()
		a := AnalysisVersion{Version: 1, ID: input.AnalysisID, IncidentID: input.IncidentID, EvidenceIDs: ids, StartedAt: completed.Add(-2 * time.Second), CompletedAt: completed, Model: "offline-fixture", Provider: "deterministic-test", Status: "complete", Summary: "OOM is a hypothesis; node failure remains unconfirmed", Hypotheses: []Hypothesis{{Text: "OOM", Supporting: ids[:1]}}, Missing: []string{"Service recovery unknown"}, NextChecks: []string{"Verify pod termination and node conditions"}}
		writeJSON(w, 200, a)
	}))
	defer worker.Close()
	t.Setenv("STRUCTURED_WORKER_URL", worker.URL)
	cluster := newStableID("cluster")
	now := time.Now().UTC()
	changeTime := now.Add(-time.Duration(fixture.ChangeMinutes) * time.Minute)
	symptom := now.Add(-time.Duration(fixture.SymptomMinutes) * time.Minute)
	change := ChangeEvent{SchemaVersion: 1, ID: newStableID("deployment"), ClusterID: cluster, Namespace: fixture.Namespace, Workload: fixture.Workload, Source: "argocd", Type: "deployment", SourceAt: &changeTime, Revision: fixture.Revision, Images: []string{fixture.Image}}
	router := backendHandler()
	request := func(method, path, id string, payload any, _ http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(payload)
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.SetPathValue("id", id)
		r.SetBasicAuth("admin", "pw")
		r.Header.Set("X-Auth-User", "spoofed")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	raw, _ := json.Marshal(change)
	r := httptest.NewRequest("POST", "/api/v1/changes", bytes.NewReader(raw))
	r.Header.Set("Authorization", "Bearer changes")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("change ingestion", w.Code, w.Body.String())
	}
	var changeResult map[string]any
	json.Unmarshal(w.Body.Bytes(), &changeResult)
	alerts := []EnrichedAlert{{ClusterName: cluster, Status: "firing", Fingerprint: newStableID("fp"), StartsAt: symptom.Format(time.RFC3339Nano), Labels: map[string]string{"namespace": fixture.Namespace, "deployment": fixture.Workload, "pod": "api-1", "node": "node-1", "alertname": fixture.Alert}, Annotations: map[string]string{"summary": "fixture-secret"}}, {ClusterName: cluster, Status: "firing", Fingerprint: newStableID("fp"), StartsAt: symptom.Format(time.RFC3339Nano), Labels: map[string]string{"namespace": fixture.Namespace, "deployment": fixture.Workload, "pod": "api-2", "alertname": "PodNotReady"}}}
	raw, _ = json.Marshal(map[string]any{"group_key": cluster + "/workload/api", "alerts": alerts})
	r = httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(raw))
	r.Header.Set("Authorization", "Bearer service")
	r.Header.Set("X-IncidentGPT-Batch-ID", newStableID("batch"))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != 202 {
		t.Fatal("durable ingest", w.Code, w.Body.String())
	}
	var accepted map[string]any
	json.Unmarshal(w.Body.Bytes(), &accepted)
	id := accepted["incident_id"].(string)
	job, err := claimJob(ctx)
	if err != nil || job == nil || job.Job.ID != accepted["job_id"] {
		t.Fatal("durable claim", err)
	}
	analysis, err := executeAnalysis(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	if err := finishJob(ctx, job, analysis, ""); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/api/v1/incidents/"+id+"/changes", id, nil, handleIncidentChanges); w.Code != 200 || !strings.Contains(w.Body.String(), fixture.Revision) {
		t.Fatal("deployment not associated", w.Body.String())
	}
	evidenceID := job.Input.Evidence[0].ID
	resolution := Resolution{Status: "confirmed", Cause: fixture.Cause, Action: fixture.Action, Failed: []string{fixture.Failed}, EvidenceIDs: []string{evidenceID}, FailureMode: fixture.Alert}
	if w := request("PUT", "/api/v1/incidents/"+id+"/resolution", id, resolution, handleResolution); w.Code != 200 || !strings.Contains(w.Body.String(), `"confirming_actor":"admin"`) {
		t.Fatal("resolution confirmation", w.Code, w.Body.String())
	}
	if w := request("GET", "/api/v1/knowledge/search?cluster_id="+cluster+"&service="+fixture.Workload+"&q=OOM", id, nil, handleKnowledgeSearch); w.Code != 200 || !strings.Contains(w.Body.String(), fixture.Cause) {
		t.Fatal("confirmed memory lookup", w.Body.String())
	}
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req DiagnosticRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Scope.ClusterID != cluster || req.Scope.Pod != "api-1" || len(req.Checks) > 6 {
			t.Error("diagnostic scope expanded")
		}
		at := time.Now().UTC()
		evidence := []Evidence{}
		for _, check := range req.Checks {
			availability, excerpt := "disabled", "Integration disabled in offline fixture"
			if check == "pod_termination" {
				availability, excerpt = "available", fixture.Observation
			}
			evidence = append(evidence, Evidence{Version: 1, ID: "diagnostic-" + req.OperationID + "-" + check, IncidentID: id, ClusterID: cluster, Source: check, Availability: availability, Excerpt: excerpt, CollectedAt: &at})
		}
		writeJSON(w, 200, DiagnosticResult{Status: "partial", CollectedAt: at, Evidence: evidence})
	}))
	defer collector.Close()
	t.Setenv("DIAGNOSTIC_COLLECTOR_URL", collector.URL)
	w = request("POST", "/api/v1/incidents/"+id+"/investigations", id, map[string]string{"action": "hypothesis", "question": "OOM or node failure?"}, handleInvestigations)
	if w.Code != 202 {
		t.Fatal("diagnostic schedule", w.Code, w.Body.String())
	}
	job, err = claimJob(ctx)
	if err != nil || job == nil || job.Job.Kind != "diagnostics" {
		t.Fatal("diagnostic claim", err)
	}
	if err := collectDiagnosticEvidence(ctx, job); err != nil {
		t.Fatal(err)
	}
	analysis, err = executeAnalysis(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	if err := finishJob(ctx, job, analysis, ""); err != nil {
		t.Fatal(err)
	}
	diagnosticAnalysisID := analysis.ID
	life, err := readLifecycle(ctx, store.Get(id))
	if err != nil {
		t.Fatal(err)
	}
	if w := request("PUT", "/api/v1/incidents/"+id+"/lifecycle", id, map[string]any{"revision": life.Revision, "verified_recovery_at": time.Now().UTC(), "evidence_ids": []string{job.Input.Evidence[0].ID}}, handleLifecycle); w.Code != 200 {
		t.Fatal("verified recovery", w.Code, w.Body.String())
	}
	if w := request("PATCH", "/api/v1/incidents/"+id, id, map[string]string{"status": "acknowledged"}, handleIncidentPatch); w.Code != 200 {
		t.Fatal("acknowledgement", w.Code, w.Body.String())
	}
	inc := store.Get(id)
	if w := request("POST", "/api/v1/incidents/"+id+"/regroup", id, map[string]string{"alert_id": inc.Alerts[1].ID}, handleRegroup); w.Code != 200 {
		t.Fatal("split", w.Code, w.Body.String())
	}
	if disk.rdb.Exists(ctx, analysisKey(diagnosticAnalysisID)).Val() != 1 {
		t.Fatal("split lost original analysis")
	}
	generate := func(kind string) ReportVersion {
		t.Helper()
		w := request("POST", "/api/v1/incidents/"+id+"/"+kind, id, nil, handleReports)
		if w.Code != 202 {
			t.Fatal("report generation", w.Code, w.Body.String())
		}
		var scheduled map[string]string
		json.Unmarshal(w.Body.Bytes(), &scheduled)
		job, err := claimJob(ctx)
		if err != nil || job == nil || job.Job.Kind != "report" {
			t.Fatal("report claim", err)
		}
		if kind == "postmortem" {
			blocked := request("POST", "/api/v1/incidents/"+id+"/postmortem", id, nil, handleReports)
			if blocked.Code != 409 {
				t.Fatal("report concurrency budget bypassed", blocked.Code)
			}
			previous := job
			previous.Lease = time.Now().Add(-time.Second).UnixMilli()
			expired, _ := json.Marshal(previous)
			disk.rdb.Set(ctx, "igpt:job:"+previous.Job.ID, expired, 0)
			disk.rdb.ZAdd(ctx, jobsDue, redis.Z{Score: float64(previous.Lease), Member: previous.Job.ID})
			recovered, err := claimJob(ctx)
			if err != nil || recovered == nil || recovered.Owner == previous.Owner {
				t.Fatal("report lease recovery", err)
			}
			runReportJob(ctx, previous)
			if disk.rdb.Exists(ctx, "igpt:report:"+scheduled["report_id"]).Val() != 0 {
				t.Fatal("stale report owner published")
			}
			job = recovered
		}
		runReportJob(ctx, job)
		raw, err := disk.rdb.Get(ctx, "igpt:report:"+scheduled["report_id"]).Bytes()
		var report ReportVersion
		if err != nil || json.Unmarshal(raw, &report) != nil {
			t.Fatal("report completion", err)
		}
		return report
	}
	first := generate("postmortem")
	if !strings.Contains(first.Markdown, "Customer impact, customer counts and financial impact: Unknown") || !strings.Contains(first.Markdown, fixture.Revision) || !strings.Contains(first.Markdown, "previous or unknown composition") || !strings.Contains(first.Markdown, "partial") || len(first.Source.ChangeIDs) != 1 {
		t.Fatal("report facts/provenance missing", first.Markdown)
	}
	edit := map[string]any{"report_id": first.ID, "revision": 1, "markdown": first.Markdown + "\nHuman impact: service degraded; fixture-secret", "review_status": "reviewed", "action_items": []ReportAction{{Text: "Add memory regression check", Owner: "oncall", Status: "open", DueDate: "2026-12-01"}}}
	if w := request("PUT", "/api/v1/incidents/"+id+"/postmortem", id, edit, handleReports); w.Code != 200 || strings.Contains(w.Body.String(), "fixture-secret") {
		t.Fatal("sanitized edit", w.Code, w.Body.String())
	}
	if w := request("PUT", "/api/v1/incidents/"+id+"/postmortem", id, edit, handleReports); w.Code != 409 {
		t.Fatal("stale edit accepted", w.Code)
	}
	second := generate("postmortem")
	if second.ID == first.ID || second.Revision != 1 {
		t.Fatal("regeneration overwrote draft")
	}
	w = request("GET", "/api/v1/incidents/"+id+"/postmortem/export?report_id="+first.ID, id, nil, handleReports)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Human impact") || !strings.Contains(w.Body.String(), "Add memory regression check") || strings.Contains(w.Body.String(), "fixture-secret") || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/markdown") {
		t.Fatal("export/manual edit lost", w.Code, w.Body.String())
	}
	handoff := generate("handoff")
	if len(handoff.Markdown) > 8<<10 || !strings.Contains(handoff.Markdown, "collection") || !strings.Contains(handoff.Markdown, "partial") {
		t.Fatal("handoff status/size invalid")
	}
	unauthorized := httptest.NewRequest("GET", "/api/v1/incidents/"+id+"/postmortem/export", nil)
	unauthorized.SetPathValue("id", id)
	unauthorized.Header.Set("X-Auth-User", "admin")
	w = httptest.NewRecorder()
	handleReports(w, unauthorized)
	if w.Code != 401 {
		t.Fatal("spoofed export identity accepted")
	}
	sanitizer.Close()
	edit["revision"] = 2
	if w := request("PUT", "/api/v1/incidents/"+id+"/postmortem", id, edit, handleReports); w.Code != 503 {
		t.Fatal("sanitizer failure did not fail closed", w.Code)
	}
	// A cause-unknown record stays unknown in a draft; no model-generated cause is substituted.
	unresolved := &Incident{ID: newStableID("fixture"), ClusterID: cluster, Namespace: fixture.Namespace, CreatedAt: time.Now().UTC()}
	store.Put(unresolved)
	source, err := buildReportSource(ctx, unresolved, "postmortem", "admin")
	if err != nil || !strings.Contains(source.Markdown, "Confirmed cause and restoring action: Unknown") {
		t.Fatal("unresolved cause invented", err)
	}
}

func TestLifecycleAnalyticsReadsActualEvents(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires disposable Redis")
	}
	// A dedicated logical test database avoids incomplete records deliberately
	// seeded by callback-fencing tests in the default database.
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 14})
	defer client.Close()
	ctx := context.Background()
	if client.Ping(ctx).Err() != nil {
		t.Fatal("Redis unavailable")
	}
	client.Del(ctx, keyIncidentIdx)
	old := store
	defer func() { store = old }()
	store = &Store{disk: &Storage{rdb: client, ctx: ctx}}
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	now := time.Now().UTC()
	receipt := now.Add(-20 * time.Minute)
	ack := receipt.Add(time.Minute)
	closed := receipt.Add(15 * time.Minute)
	inc := &Incident{ID: newStableID("metric"), CreatedAt: receipt, DurationSec: func() *int { v := 1200; return &v }()}
	store.Put(inc)
	l := Lifecycle{Version: 1, IncidentID: inc.ID, Revision: 1, FirstSymptom: &receipt, AlertReceipt: &receipt, Acknowledged: &ack, Closed: &closed}
	raw, _ := json.Marshal(l)
	client.Set(ctx, "igpt:lifecycle:"+inc.ID, raw, 0)
	analysis := AnalysisVersion{ID: newStableID("analysis"), IncidentID: inc.ID, StartedAt: now.Add(-2 * time.Second), CompletedAt: now}
	raw, _ = json.Marshal(analysis)
	client.Set(ctx, analysisKey(analysis.ID), raw, 0)
	client.Set(ctx, analysisLatest(inc.ID), analysis.ID, 0)
	r := httptest.NewRequest("GET", "/api/v1/analytics/summary?period=30d", nil)
	r.SetBasicAuth("admin", "pw")
	w := httptest.NewRecorder()
	handleAnalyticsSummary(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var output struct {
		Analysis struct {
			Mean    float64 `json:"mean_seconds"`
			Samples int     `json:"samples"`
		} `json:"analysis"`
		Recovery struct {
			Mean    *float64 `json:"mean_seconds"`
			Samples int      `json:"samples"`
			Missing int      `json:"missing"`
		} `json:"verified_recovery"`
		Reduction *float64 `json:"mttr_reduction_pct"`
	}
	json.Unmarshal(w.Body.Bytes(), &output)
	if output.Analysis.Mean != 2 || output.Analysis.Samples != 1 || output.Recovery.Mean != nil || output.Recovery.Samples != 0 || output.Recovery.Missing != 1 || output.Reduction != nil {
		t.Fatal("metrics invented event times or benefit", w.Body.String())
	}
	r = httptest.NewRequest("GET", "/api/v1/incidents/"+inc.ID+"/lifecycle", nil)
	r.SetPathValue("id", inc.ID)
	r.SetBasicAuth("admin", "pw")
	w = httptest.NewRecorder()
	handleLifecycle(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), "verified_recovery_at") {
		t.Fatal("recovery inferred from administrative closure")
	}
}

func TestBrowserMutationOrigin(t *testing.T) {
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	for _, origin := range []string{"https://foreign.example", "null"} {
		r := httptest.NewRequest("POST", "https://incident.example/api/v1/incidents/INC-1/postmortem", nil)
		r.SetBasicAuth("admin", "pw")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		if changeReadAuth(w, r) || w.Code != 403 {
			t.Fatal("foreign browser mutation accepted", origin)
		}
	}
	r := httptest.NewRequest("POST", "http://localhost:3000/api/v1/incidents/INC-1/postmortem", nil)
	r.SetBasicAuth("admin", "pw")
	r.Header.Set("Origin", "http://localhost:3000")
	if !changeReadAuth(httptest.NewRecorder(), r) {
		t.Fatal("same UI host and port denied")
	}
}
