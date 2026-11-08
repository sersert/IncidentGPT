package main

import (
	"bytes"
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

func TestEvaluationSnapshotExcludesAnswers(t *testing.T) {
	input := AnalysisInput{Version: 1, IncidentID: "INC", AnalysisID: "snapshot", Evidence: []Evidence{{Version: 1, ID: "E1", IncidentID: "INC", Source: "logs", Excerpt: "OOMKilled"}}}
	if validateEvaluationInput(input) != nil {
		t.Fatal("valid observations rejected")
	}
	input.Context = []string{"confirmed cause"}
	if validateEvaluationInput(input) == nil {
		t.Fatal("reference context accepted")
	}
	input.Context = nil
	input.Evidence[0].Source = "confirmed_resolution"
	if validateEvaluationInput(input) == nil {
		t.Fatal("knowledge evidence accepted")
	}
}
func TestEvaluationRedisIsolationIdempotencyAndFencing(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires disposable Redis")
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
	disk.rdb.Del(ctx, evaluationDue, "igpt:evaluation:active", "igpt:evaluation:rate-limit", evaluationCases, evaluationRuns)
	t.Setenv("EVALUATION_ENABLED", "true")
	t.Setenv("OPERATIONS_ENABLED", "false")
	t.Setenv("TIMELINE_READ_USER", "operator")
	t.Setenv("TIMELINE_READ_PASSWORD", "password")
	t.Setenv("ANALYSIS_SERVICE_TOKEN", strings.Repeat("t", 32))
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sanitizerPayloadRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": req.Payload})
	}))
	defer sanitizer.Close()
	t.Setenv("SANITIZER_URL", sanitizer.URL)
	t.Setenv("SANITIZER_AUTH_SHARED_SECRET", "fixture-hmac")
	p := EvaluationProfile{ID: "alpha", Label: "Alpha", Model: "test/alpha", PromptVersion: "evidence-analysis/v2", MaxTokens: 128}
	p.Revision = profileRevision(p)
	q := p
	q.ID = "beta"
	q.Label = "Beta"
	q.Model = "test/beta"
	q.Revision = profileRevision(q)
	calls := 0
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 32) {
			t.Error("missing service auth")
		}
		if r.Method == "GET" {
			writeJSON(w, 200, map[string]any{"enabled": true, "profiles": []EvaluationProfile{p, q}})
			return
		}
		calls++
		var req EvaluationRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		raw, _ := json.Marshal(req)
		if strings.Contains(string(raw), "DO_NOT_SEND_REFERENCE") || strings.Contains(string(raw), "OLD_ANALYSIS") || strings.Contains(string(raw), "HISTORY_ANSWER") {
			t.Error("reference leaked into inference")
		}
		now := time.Now().UTC()
		ids := []string{}
		for _, e := range req.Input.Evidence {
			ids = append(ids, e.ID)
		}
		writeJSON(w, 200, AnalysisVersion{Version: 1, ID: req.Input.AnalysisID, IncidentID: req.Input.IncidentID, EvidenceIDs: ids, Model: req.Profile.Model, PromptVersion: req.Profile.PromptVersion, StartedAt: now.Add(-time.Second), CompletedAt: now, Status: "complete", Summary: "A competing hypothesis", Notification: "disabled", HumanConfirmation: "unconfirmed"})
	}))
	defer worker.Close()
	t.Setenv("STRUCTURED_WORKER_URL", worker.URL+"/v1/analyze")
	call := func(handler http.HandlerFunc, method string, payload any, values map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(payload)
		r := httptest.NewRequest(method, "http://preview/evaluation", bytes.NewReader(raw))
		r.SetBasicAuth("operator", "password")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://preview")
		for k, v := range values {
			r.SetPathValue(k, v)
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	inc := &Incident{ID: newStableID("fixture"), ClusterID: "prod", CreatedAt: time.Now(), Alerts: []*Alert{{ID: "ALT"}}}
	store.Put(inc)
	aid := newStableID("analysis")
	composition := incidentComposition(inc)
	observation := Evidence{Version: 1, ID: newStableID("evidence"), IncidentID: inc.ID, Source: "logs", Availability: "available", Excerpt: "OOMKilled", ClusterID: "prod"}
	memory := observation
	memory.ID = newStableID("memory")
	memory.Source = "confirmed_resolution"
	memory.Excerpt = "HISTORY_ANSWER"
	for _, e := range []Evidence{observation, memory} {
		raw, _ := json.Marshal(e)
		disk.rdb.Set(ctx, "igpt:evidence:"+e.ID, raw, 0)
	}
	original := AnalysisVersion{Version: 1, ID: aid, IncidentID: inc.ID, Status: "complete", Summary: "OLD_ANALYSIS", Composition: composition, EvidenceIDs: []string{observation.ID, memory.ID}, CompletedAt: time.Now().UTC()}
	raw, _ := json.Marshal(original)
	disk.rdb.Set(ctx, analysisKey(aid), raw, 0)
	disk.rdb.Set(ctx, analysisLatest(inc.ID), aid, 0)
	resolution := Resolution{IncidentID: inc.ID, Status: "confirmed", Composition: composition, Cause: "DO_NOT_SEND_REFERENCE", Revision: 3, Actor: "reviewer"}
	raw, _ = json.Marshal(resolution)
	disk.rdb.Set(ctx, "igpt:resolution:"+inc.ID, raw, 0)
	w := call(handleEvaluationCases, "POST", map[string]any{"name": "Saved incident", "incident_id": inc.ID, "analysis_id": aid}, nil)
	if w.Code != 201 {
		t.Fatalf("capture %d %s", w.Code, w.Body.String())
	}
	var c EvaluationCase
	_ = json.Unmarshal(w.Body.Bytes(), &c)
	if c.Reference.Cause != "DO_NOT_SEND_REFERENCE" || len(c.Input.Evidence) != 1 || c.SourceAnalysis != aid {
		t.Fatal("snapshot/reference separation failed")
	}
	observation.Excerpt = "later data"
	raw, _ = json.Marshal(observation)
	disk.rdb.Set(ctx, "igpt:evidence:"+observation.ID, raw, 0)
	frozen, err := loadEvaluationCase(ctx, c.ID)
	if err != nil || frozen.Input.Evidence[0].Excerpt != "OOMKilled" {
		t.Fatal("snapshot changed")
	}
	payload := map[string]any{"profile_ids": []string{"alpha", "beta"}, "request_id": newStableID("request"), "confirm_provider_calls": true}
	w = call(handleEvaluationRuns, "POST", payload, map[string]string{"case": c.ID})
	if w.Code != 202 {
		t.Fatalf("enqueue %d %s", w.Code, w.Body.String())
	}
	var accepted struct {
		ID string `json:"run_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &accepted)
	if w = call(handleEvaluationRuns, "POST", payload, map[string]string{"case": c.ID}); w.Code != 202 || !strings.Contains(w.Body.String(), `"duplicate":true`) {
		t.Fatal("duplicate request was not idempotent")
	}
	if disk.rdb.ZCard(ctx, evaluationDue).Val() != 2 {
		t.Fatal("comparison not atomic")
	}
	for i := 0; i < 2; i++ {
		job, err := claimEvaluation(ctx)
		if err != nil || job == nil {
			t.Fatalf("claim %v", err)
		}
		if next, _ := claimEvaluation(ctx); next != nil {
			t.Fatal("global lease not exclusive")
		}
		result, code := executeEvaluation(ctx, job)
		if result == nil {
			t.Fatal(code)
		}
		stale := *job
		stale.Owner = "foreign"
		if finishEvaluation(ctx, &stale, result, "") == nil {
			t.Fatal("stale owner committed")
		}
		if finishEvaluation(ctx, job, result, "") != nil {
			t.Fatal("completion failed")
		}
		if finishEvaluation(ctx, job, result, "") == nil {
			t.Fatal("duplicate completion")
		}
	}
	if calls != 2 || disk.rdb.Get(ctx, analysisLatest(inc.ID)).Val() != aid || disk.rdb.ZCard(ctx, "igpt:analysis:history:"+inc.ID).Val() != 0 {
		t.Fatal("evaluation changed live analysis")
	}
	review := map[string]any{"revision": 0, "verdict": "partial", "note": "Needs evidence"}
	values := map[string]string{"run": accepted.ID, "profile": "alpha"}
	if w = call(handleEvaluationReview, "PUT", review, values); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call(handleEvaluationReview, "PUT", review, values); w.Code != 409 {
		t.Fatal("stale review accepted")
	}
	// A worker crash after submitting a paid request must never trigger a replay.
	jid := accepted.ID + ":crashed"
	input := c.Input
	input.AnalysisID = jid
	job := EvaluationJob{ID: jid, RunID: accepted.ID, Input: input, Profile: p, Status: "running", Owner: "dead", Lease: time.Now().Add(-time.Second).UnixMilli()}
	raw, _ = json.Marshal(job)
	key := "igpt:evaluation:job:" + jid
	disk.rdb.Set(ctx, key, raw, evaluationTTL)
	disk.rdb.ZAdd(ctx, evaluationDue, redis.Z{Score: float64(time.Now().Add(-time.Second).UnixMilli()), Member: key})
	if got, err := claimEvaluation(ctx); err != nil || got != nil {
		t.Fatal("lost paid attempt was executed again")
	}
	raw, _ = disk.rdb.Get(ctx, key).Bytes()
	_ = json.Unmarshal(raw, &job)
	if job.Status != "failed" || job.ErrorCode != "interrupted_no_automatic_retry" {
		t.Fatal("interrupted request not surfaced")
	}
	// Authentication is independent of legacy API mode and rejects foreign Origin.
	r := httptest.NewRequest("POST", "http://preview/evaluation", strings.NewReader(`{}`))
	r.SetBasicAuth("operator", "password")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://foreign.example")
	w = httptest.NewRecorder()
	handleEvaluationCases(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin accepted")
	}
	sanitizer.Close()
	w = call(handleEvaluationCases, "POST", map[string]any{"name": "No sanitizer", "scenario": "test"}, nil)
	if w.Code != 503 {
		t.Fatal("unsanitized case persisted")
	}
}

func TestEvaluationUsageNeverRestoresStrings(t *testing.T) {
	usage := evaluationUsage(map[string]any{"total_tokens": "[REDACTED_TOKEN]", "prompt_tokens": float64(42), "completion_tokens": 0.5, "cost": 0.001, "secret": "private"})
	if len(usage) != 2 || usage["prompt_tokens"] != float64(42) || usage["cost"] != 0.001 {
		t.Fatalf("invalid numeric usage: %v", usage)
	}
}
