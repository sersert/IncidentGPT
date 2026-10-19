package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestAnalysisRejectsForeignCitations(t *testing.T) {
	input := AnalysisInput{IncidentID: "INC-1", AnalysisID: "A-1", Evidence: []Evidence{{ID: "E-1", IncidentID: "INC-1"}}}
	a := AnalysisVersion{StartedAt: time.Now().UTC().Add(-time.Second), CompletedAt: time.Now().UTC(), Version: 1, ID: "A-1", IncidentID: "INC-1", EvidenceIDs: []string{"E-1"}, Status: "complete", Summary: "Pod termination is a hypothesis", Hypotheses: []Hypothesis{{Text: "OOM", Supporting: []string{"E-foreign"}}}}
	if validateAnalysis(a, input) == nil {
		t.Fatal("foreign citation accepted")
	}
	a.Hypotheses[0].Supporting = []string{"E-1"}
	if err := validateAnalysis(a, input); err != nil {
		t.Fatal(err)
	}
	a.IncidentID = "INC-other"
	if validateAnalysis(a, input) == nil {
		t.Fatal("foreign incident accepted")
	}
}
func TestJobsRedisRecoveryAndFencing(t *testing.T) {
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
	// Each test uses a disposable Redis service. Remove only this queue's pending
	// state left by earlier failed runs; never point the fixture at production.
	disk.rdb.Del(ctx, jobsDue)
	now := time.Now().UTC()
	inc := &Incident{ID: newStableID("fixture"), CreatedAt: now, Alerts: []*Alert{{ID: "ALT-1"}}}
	store.Put(inc)
	input := AnalysisInput{Version: 1, IncidentID: inc.ID, AnalysisID: newStableID("analysis"), Evidence: []Evidence{{Version: 1, ID: newStableID("evidence"), IncidentID: inc.ID, Availability: "available", Excerpt: "OOMKilled"}}}
	jid := newStableID("job")
	if _, err := enqueueInput(ctx, inc, input, jid); err != nil {
		t.Fatal(err)
	}
	if _, err := enqueueInput(ctx, inc, input, jid); err != nil {
		t.Fatal(err)
	}
	if disk.rdb.ZCard(ctx, jobsDue).Val() != 1 {
		t.Fatal("duplicate scheduling")
	}
	first, err := claimJob(ctx)
	if err != nil || first == nil {
		t.Fatalf("claim: %v", err)
	}
	if second, err := claimJob(ctx); err != nil || second != nil {
		t.Fatal("lease not exclusive")
	}
	// Simulate owner termination: expiration and persisted due time allow a new
	// process to acquire the same immutable input without another ingest request.
	first.Lease = time.Now().Add(-time.Second).UnixMilli()
	raw, _ := json.Marshal(first)
	disk.rdb.Set(ctx, "igpt:job:"+jid, raw, 0)
	disk.rdb.ZAdd(ctx, jobsDue, redis.Z{Score: float64(time.Now().Add(-time.Second).UnixMilli()), Member: jid})
	recovered, err := claimJob(ctx)
	if err != nil || recovered == nil || recovered.Owner == first.Owner || recovered.Job.Attempts != 2 {
		t.Fatalf("recovery: %v", err)
	}
	a := &AnalysisVersion{Version: 1, ID: input.AnalysisID, IncidentID: inc.ID, Status: "complete", Summary: "hypothesis", CompletedAt: time.Now().UTC()}
	if finishJob(ctx, first, a, "") == nil {
		t.Fatal("stale owner completed job")
	}
	// A newer requested analysis must remain current even if an old result arrives.
	newer := newStableID("analysis")
	disk.rdb.Set(ctx, "igpt:analysis:requested:"+inc.ID, newer, 0)
	disk.rdb.Set(ctx, analysisLatest(inc.ID), newer, 0)
	if err := finishJob(ctx, recovered, a, ""); err != nil {
		t.Fatal(err)
	}
	if disk.rdb.Get(ctx, analysisLatest(inc.ID)).Val() != newer {
		t.Fatal("late result replaced latest")
	}
	if finishJob(ctx, recovered, a, "") == nil {
		t.Fatal("duplicate completion mutated history")
	}
	if disk.rdb.ZCard(ctx, "igpt:analysis:history:"+inc.ID).Val() != 1 {
		t.Fatal("duplicate analysis")
	}
	if disk.rdb.Exists(ctx, "igpt:evidence:"+input.Evidence[0].ID).Val() != 1 {
		t.Fatal("evidence not atomically persisted")
	}
}

func TestProvidedEvidenceDoesNotInventCollectionIntervals(t *testing.T) {
	inc := &Incident{ID: "INC-1", ClusterID: "prod"}
	evidence := makeEvidence(inc, []EnrichedAlert{{StartsAt: "2026-10-07T12:00:00Z", EnrichedAt: "2026-10-07T12:01:00Z", PromSample: map[string]any{"value": 1}}})
	if evidence[0].ObservedFrom == nil || evidence[2].ObservedFrom != nil || evidence[2].CollectedAt != nil {
		t.Fatal("alert time substituted for query/collection time")
	}
	if evidence[3].Availability != "unavailable" {
		t.Fatal("missing legacy logs represented as successful empty query")
	}
}

func TestDurableBatchReplayAfterAcknowledgment(t *testing.T) {
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
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "true")
	t.Setenv("ANALYSIS_SERVICE_TOKEN", "service-token")
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sanitizerPayloadRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": req.Payload})
	}))
	defer sanitizer.Close()
	t.Setenv("SANITIZER_URL", sanitizer.URL)
	t.Setenv("SANITIZER_AUTH_SHARED_SECRET", "hmac-key")
	cluster := newStableID("cluster")
	batch := newStableID("batch")
	payload, _ := json.Marshal(map[string]any{"group_key": cluster + "/ns", "alerts": []EnrichedAlert{{ClusterName: cluster, Fingerprint: "fp", Status: "firing", Labels: map[string]string{"namespace": "ns", "alertname": "PodDown"}}}})
	run := func() map[string]any {
		t.Helper()
		r := httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(payload))
		r.Header.Set("Authorization", "Bearer service-token")
		r.Header.Set("X-IncidentGPT-Batch-ID", batch)
		w := httptest.NewRecorder()
		handleIngest(w, r)
		if w.Code != 202 {
			t.Fatalf("ingest %d %s", w.Code, w.Body.String())
		}
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return result
	}
	first := run()
	inc := store.Get(first["incident_id"].(string))
	inc.Status = "acknowledged"
	store.Put(inc)
	duplicate := run()
	if duplicate["job_id"] != first["job_id"] || duplicate["duplicate"] != true {
		t.Fatal("batch replay scheduled new analysis")
	}
}
