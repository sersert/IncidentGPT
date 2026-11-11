package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func setupOperations(t *testing.T) {
	t.Helper()
	t.Setenv("OPERATIONS_ENABLED", "true")
	t.Setenv("OPERATIONS_SERVICE_TOKEN", strings.Repeat("t", 32))
	t.Setenv("TIMELINE_READ_USER", "operator")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	operationsCache.Lock()
	operationsCache.at = time.Time{}
	operationsCache.value = OperationsSnapshot{}
	operationsCache.Unlock()
	t.Cleanup(func() {
		operationsCache.Lock()
		operationsCache.at = time.Time{}
		operationsCache.value = OperationsSnapshot{}
		operationsCache.Unlock()
	})
}
func operationsRequest(method, path, body string, auth bool) *http.Request {
	r := httptest.NewRequest(method, "http://backend"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://backend")
	if auth {
		r.SetBasicAuth("operator", "pw")
	}
	return r
}
func TestOperationsChecksAreRealBoundedAndSanitized(t *testing.T) {
	setupOperations(t)
	old := store
	defer func() { store = old }()
	store = &Store{}
	var probes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probes.Add(1)
		switch r.URL.Path {
		case "/worker/internal/operations":
			writeJSON(w, 200, map[string]any{"service": "ai-worker", "managed": true, "control_known": true, "control": map[string]any{"enabled": false, "revision": 0}, "model": map[string]any{"status": "healthy", "checked_at": time.Now().Add(-6 * time.Minute), "token": "SECRET_MUST_NOT_ESCAPE"}, "untrusted_secret": "SECRET_MUST_NOT_ESCAPE"})
		case "/enricher/internal/operations":
			writeJSON(w, 200, map[string]any{"checks": []any{map[string]string{"id": "metrics", "status": "unhealthy", "detail": "Metrics unavailable"}, map[string]string{"id": "kubernetes", "status": "disabled", "detail": "Kubernetes disabled"}}})
		case "/sanitizer/readyz":
			http.Error(w, "failed", 503)
		default:
			writeJSON(w, 200, map[string]bool{"ready": true})
		}
	}))
	defer server.Close()
	t.Setenv("OPERATIONS_WORKER_URL", server.URL+"/worker")
	t.Setenv("OPERATIONS_ENRICHER_URL", server.URL+"/enricher")
	t.Setenv("SANITIZER_URL", server.URL+"/sanitizer")
	w := httptest.NewRecorder()
	handleOperations(w, operationsRequest("GET", "/api/v1/operations/status", "", false))
	if w.Code != 401 || probes.Load() != 0 {
		t.Fatal("unauthenticated monitoring performed probes")
	}
	w = httptest.NewRecorder()
	handleOperations(w, operationsRequest("GET", "/api/v1/operations/status", "", true))
	if w.Code != 200 || strings.Contains(w.Body.String(), "SECRET_MUST_NOT_ESCAPE") {
		t.Fatalf("unsafe operations response: %d %s", w.Code, w.Body.String())
	}
	var result OperationsSnapshot
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	states := map[string]string{}
	for _, component := range result.Components {
		states[component.ID] = component.Status
	}
	if states["sanitizer"] != "unhealthy" || states["model"] != "unknown" || states["redis"] != "not_configured" || states["metrics"] != "unhealthy" || result.ControlAvailable {
		t.Fatalf("fabricated availability: %v", states)
	}
	before := probes.Load()
	w = httptest.NewRecorder()
	handleOperations(w, operationsRequest("GET", "/api/v1/operations/status", "", true))
	if probes.Load() != before {
		t.Fatal("monitoring ignored shared probe cache")
	}
}
func TestAIControlPersistenceAuthorizationAndPauseDoesNotDrainRetries(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires dedicated Redis")
	}
	setupOperations(t)
	old := store
	defer func() { store = old }()
	disk := NewStorage(addr, "")
	if disk == nil {
		t.Fatal("Redis unavailable")
	}
	defer disk.rdb.Close()
	store = &Store{disk: disk}
	ctx := context.Background()
	prior, priorErr := disk.rdb.Get(ctx, aiControlKey).Result()
	_ = disk.rdb.Del(ctx, aiControlKey).Err()
	defer func() {
		if priorErr == nil {
			disk.rdb.Set(ctx, aiControlKey, prior, 0)
		} else {
			disk.rdb.Del(ctx, aiControlKey)
		}
	}()
	state, err := readAIControl(ctx)
	if err != nil || state.Enabled || state.Revision != 0 {
		t.Fatal("first enable must start paused")
	}
	var workerUnavailable atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if workerUnavailable.Load() {
			http.Error(w, "worker unavailable", 503)
			return
		}
		writeJSON(w, 200, map[string]any{"service": "ai-worker", "managed": true})
	}))
	defer server.Close()
	t.Setenv("OPERATIONS_WORKER_URL", server.URL)
	request := func(body string) *httptest.ResponseRecorder {
		r := operationsRequest("PUT", "/api/v1/operations/ai-control", body, true)
		r.Header.Set("X-Auth-User", "forged")
		w := httptest.NewRecorder()
		handleAIControl(w, r)
		return w
	}
	if w := request(`{"enabled":true,"revision":0}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := request(`{"enabled":false,"revision":0}`); w.Code != 409 {
		t.Fatal("stale control overwrote newer state")
	}
	workerUnavailable.Store(true)
	if w := request(`{"enabled":true,"revision":1}`); w.Code != 503 {
		t.Fatal("enabled without compatible live worker")
	}
	if w := request(`{"enabled":false,"revision":1}`); w.Code != 200 {
		t.Fatal("cannot persist pause while worker is down: " + w.Body.String())
	}
	state, err = readAIControl(ctx)
	if err != nil || state.Enabled || state.UpdatedBy != "operator" || state.Revision != 2 {
		t.Fatal("pause or verified actor not persisted")
	}
	raw, _ := disk.rdb.Get(ctx, aiControlKey).Bytes()
	var persisted AIControlState
	if json.Unmarshal(raw, &persisted) != nil || persisted.Enabled {
		t.Fatal("stored pause lost")
	}
	// A fresh client after process restart must read the same durable pause.
	restarted := NewStorage(addr, "")
	if restarted == nil {
		t.Fatal("Redis unavailable after restart")
	}
	store = &Store{disk: restarted}
	state, err = readAIControl(ctx)
	if err != nil || state.Enabled || state.Revision != 2 {
		t.Fatal("restart lost persisted pause")
	}
	restarted.rdb.Close()
	store = &Store{disk: disk}
	// Isolate queue members; do not flush other test jobs.
	id := newStableID("pause-test")
	queue := "igpt:test:operations:" + id
	jobKey := "igpt:job:" + id
	defer disk.rdb.Del(ctx, queue, jobKey)
	envelope := jobEnvelope{Job: DurableJob{ID: id, Kind: "analysis", Status: "queued", Attempts: 0}, Input: AnalysisInput{IncidentID: id, AnalysisID: id}}
	encoded, _ := json.Marshal(envelope)
	disk.rdb.Set(ctx, jobKey, encoded, 0)
	now := time.Now().UTC()
	disk.rdb.ZAdd(ctx, queue, redis.Z{Score: float64(now.Add(-time.Second).UnixMilli()), Member: id})
	_, _ = claimJobScript.Run(ctx, disk.rdb, []string{queue}, now.UnixMilli(), "owner", now.Add(time.Minute).UnixMilli(), now.Format(time.RFC3339Nano), "paused").Result()
	stored, _ := disk.rdb.Get(ctx, jobKey).Bytes()
	var paused jobEnvelope
	_ = json.Unmarshal(stored, &paused)
	if paused.Job.Attempts != 0 || paused.Job.Status != "queued" || paused.Job.ErrorCode != "ai_paused" {
		t.Fatal("pause consumed retry budget")
	}
	reportID := newStableID("report-test")
	reportKey := "igpt:job:" + reportID
	defer disk.rdb.Del(ctx, reportKey)
	envelope.Job.ID = reportID
	envelope.Job.Kind = "report"
	encoded, _ = json.Marshal(envelope)
	disk.rdb.Set(ctx, reportKey, encoded, 0)
	disk.rdb.ZAdd(ctx, queue, redis.Z{Score: float64(now.Add(-time.Second).UnixMilli()), Member: reportID})
	claimed, err := claimJobScript.Run(ctx, disk.rdb, []string{queue}, now.UnixMilli(), "owner", now.Add(time.Minute).UnixMilli(), now.Format(time.RFC3339Nano), "paused").Text()
	if err != nil || !strings.Contains(claimed, reportID) {
		t.Fatal("pause blocked non-LLM report jobs")
	}
	// A pause racing the fourth running attempt returns the job without exhausting it.
	envelope.Job = DurableJob{ID: id, Kind: "analysis", Status: "running", Attempts: 4}
	envelope.Owner = "owner"
	envelope.Lease = now.Add(time.Minute).UnixMilli()
	encoded, _ = json.Marshal(envelope)
	disk.rdb.Set(ctx, jobKey, encoded, 0)
	if err := finishJob(ctx, &envelope, nil, "ai_paused"); err != nil {
		t.Fatal(err)
	}
	defer disk.rdb.ZRem(ctx, jobsDue, id)
	stored, _ = disk.rdb.Get(ctx, jobKey).Bytes()
	_ = json.Unmarshal(stored, &paused)
	if paused.Job.Attempts != 3 || paused.Job.Status != "queued" {
		t.Fatal("pause exhausted last running attempt")
	}
	disk.rdb.Set(ctx, aiControlKey, `{"revision":3}`, 0)
	if _, err := readAIControl(ctx); err == nil {
		t.Fatal("corrupt control state enabled AI")
	}
}

func TestOperationsMutationAndServiceAuth(t *testing.T) {
	setupOperations(t)
	for _, handler := range []http.HandlerFunc{handleAIControl, handleModelProbe} {
		for _, tc := range []struct {
			auth   bool
			origin string
			status int
		}{{false, "http://backend", 401}, {true, "https://foreign.example", 403}, {true, "", 403}} {
			r := operationsRequest("PUT", "/api/v1/operations/ai-control", `{"enabled":false,"revision":0}`, tc.auth)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != tc.status {
				t.Fatalf("unsafe mutation: got %d want %d", w.Code, tc.status)
			}
		}
	}
	r := httptest.NewRequest("GET", "/internal/operations/ai-control", nil)
	w := httptest.NewRecorder()
	handleWorkerAIControl(w, r)
	if w.Code != 401 {
		t.Fatal("service control allowed anonymous access")
	}
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 32))
	r.Header.Set("Origin", "http://backend")
	w = httptest.NewRecorder()
	handleWorkerAIControl(w, r)
	if w.Code != 401 {
		t.Fatal("browser origin allowed on internal control")
	}
}

func TestMonitoringCollectionSurvivesCallerCancelAndProbesLogsConcurrently(t *testing.T) {
	setupOperations(t)
	workerStarted := make(chan struct{})
	logsStarted := make(chan struct{})
	var once sync.Once
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			once.Do(func() { close(workerStarted) })
			select {
			case <-logsStarted:
			case <-r.Context().Done():
				return
			}
		}
		writeJSON(w, 200, map[string]any{"service": "ai-worker", "managed": true})
	}))
	defer worker.Close()
	logs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(logsStarted); w.WriteHeader(200) }))
	defer logs.Close()
	t.Setenv("OPERATIONS_WORKER_URL", worker.URL)
	t.Setenv("LOGS_STORE_URL", logs.URL)
	t.Setenv("LOGS_KIND", "opensearch")
	t.Setenv("OPERATIONS_ENRICHER_URL", "")
	t.Setenv("SANITIZER_URL", "")
	t.Setenv("OPERATIONS_MCP_HEALTH_URL", "")
	ctx, cancel := context.WithCancel(context.Background())
	abandoned := make(chan struct{})
	go func() { collectOperations(ctx); close(abandoned) }()
	select {
	case <-workerStarted:
	case <-time.After(time.Second):
		t.Fatal("collection never started")
	}
	cancel()
	<-abandoned
	snapshot := collectOperations(context.Background())
	statuses := map[string]string{}
	for _, check := range snapshot.Components {
		statuses[check.ID] = check.Status
	}
	if statuses["worker"] != "healthy" || statuses["logs"] != "healthy" {
		t.Fatalf("caller canceled shared collection or logs ran too late: %+v", statuses)
	}
	if statuses["enricher"] != "not_configured" {
		t.Fatalf("missing endpoint was not classified: %+v", statuses)
	}
	if !collectOperations(context.Background()).CheckedAt.Equal(snapshot.CheckedAt) {
		t.Fatal("shared snapshot was not cached")
	}
}
func TestWorkerPauseDefersAnalysisEvenWithoutBackendOperationsFlag(t *testing.T) {
	t.Setenv("OPERATIONS_ENABLED", "false")
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(423) }))
	defer worker.Close()
	t.Setenv("STRUCTURED_WORKER_URL", worker.URL)
	_, err := executeAnalysis(context.Background(), &jobEnvelope{})
	if err == nil || err.Error() != "ai_paused" {
		t.Fatalf("worker pause consumes retry budget: %v", err)
	}
}

func TestProbePreflightFailureDoesNotExhaustLimiterOrReturnBrowser401(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires dedicated Redis")
	}
	setupOperations(t)
	old := store
	defer func() { store = old }()
	disk := NewStorage(addr, "")
	if disk == nil {
		t.Fatal("Redis unavailable")
	}
	defer disk.rdb.Close()
	store = &Store{disk: disk}
	ctx := context.Background()
	keys := []string{aiControlKey, "igpt:operations:model-probe:limit"}
	originals := map[string]string{}
	for _, key := range keys {
		if value, err := disk.rdb.Get(ctx, key).Result(); err == nil {
			originals[key] = value
		}
		disk.rdb.Del(ctx, key)
	}
	defer func() {
		for _, key := range keys {
			if value, ok := originals[key]; ok {
				disk.rdb.Set(ctx, key, value, 0)
			} else {
				disk.rdb.Del(ctx, key)
			}
		}
	}()
	value, _ := json.Marshal(AIControlState{Schema: 1, Enabled: true, Revision: 1, UpdatedAt: time.Now().UTC(), UpdatedBy: "operator"})
	disk.rdb.Set(ctx, aiControlKey, value, 0)
	var status atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status.Load() != 200 {
			w.WriteHeader(int(status.Load()))
			return
		}
		writeJSON(w, 200, map[string]any{"success": true, "latency_ms": 1})
	}))
	defer worker.Close()
	t.Setenv("OPERATIONS_WORKER_URL", worker.URL)
	for _, code := range []int{401, 423, 503} {
		status.Store(int32(code))
		response := httptest.NewRecorder()
		handleModelProbe(response, operationsRequest("POST", "/api/v1/operations/model-probe", "{}", true))
		want := code
		if code == 401 {
			want = 502
		}
		if response.Code != want {
			t.Fatalf("internal status %d became browser %d", code, response.Code)
		}
		if disk.rdb.Exists(ctx, "igpt:operations:model-probe:limit").Val() != 0 {
			t.Fatal("preflight rejection consumed paid probe budget")
		}
	}
	status.Store(200)
	response := httptest.NewRecorder()
	handleModelProbe(response, operationsRequest("POST", "/api/v1/operations/model-probe", "{}", true))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	response = httptest.NewRecorder()
	handleModelProbe(response, operationsRequest("POST", "/api/v1/operations/model-probe", "{}", true))
	if response.Code != 429 {
		t.Fatal("successful probe did not consume limiter")
	}
}

func TestOperationsCannotStartInPermanentMemoryFallback(t *testing.T) {
	t.Setenv("OPERATIONS_ENABLED", "true")
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "false")
	t.Setenv("REDIS_ADDR", "")
	old := store
	store = &Store{}
	defer func() { store = old }()
	if err := awaitDurableStorage(context.Background()); err == nil || !strings.Contains(err.Error(), "operations requires REDIS_ADDR") {
		t.Fatalf("operations silently started without persistent control: %v", err)
	}
}
