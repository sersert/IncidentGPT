package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func regressionMemory(t *testing.T) *Incident {
	t.Helper()
	old := store
	t.Cleanup(func() { store = old })
	inc := &Incident{ID: newStableID("review"), Status: "active", CreatedAt: time.Now().UTC()}
	store = &Store{incidents: map[string]*Incident{inc.ID: inc}, alerts: map[string]*Alert{}}
	t.Setenv("REPORTS_ENABLED", "false")
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "false")
	t.Setenv("LEGACY_API_AUTH_REQUIRED", "false")
	return inc
}
func regressionRequest(method, path, body string, auth bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://demo.example"+path, strings.NewReader(body))
	if auth {
		r.SetBasicAuth("admin", "pw")
	}
	w := httptest.NewRecorder()
	backendHandler().ServeHTTP(w, r)
	return w
}
func regressionRedis(t *testing.T) *Storage {
	t.Helper()
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("dedicated test Redis required")
	}
	old := store
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 13, ContextTimeoutEnabled: true})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	disk := &Storage{rdb: rdb, ctx: context.Background()}
	store = &Store{disk: disk}
	t.Cleanup(func() { rdb.Close(); store = old })
	return disk
}
func regressionIncident(t *testing.T, disk *Storage, inc *Incident) {
	t.Helper()
	if err := disk.SaveIncident(inc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		disk.rdb.Del(ctx, keyIncident+inc.ID, "igpt:lifecycle:"+inc.ID, "igpt:incident:postmortem:"+inc.ID)
		disk.rdb.ZRem(ctx, keyIncidentIdx, inc.ID)
		keys, _ := disk.rdb.Keys(ctx, "igpt:lifecycle:history:"+inc.ID+":*").Result()
		if len(keys) > 0 {
			disk.rdb.Del(ctx, keys...)
		}
	})
}
func TestRegressionLegacyAPIsAndMemoryMetrics(t *testing.T) {
	inc := regressionMemory(t)
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "")
	paths := []string{"/api/v1/dashboard/stats", "/api/v1/analytics/summary", "/api/v1/analytics/mttr", "/api/v1/analytics/quality"}
	for _, path := range paths {
		w := regressionRequest("GET", path, "", false)
		if w.Code != 200 {
			t.Fatalf("compatibility %s: %d %s", path, w.Code, w.Body.String())
		}
		var result map[string]any
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result["persistent"] != false {
			t.Fatal(w.Body.String())
		}
	}
	w := regressionRequest("PATCH", "/api/v1/incidents/"+inc.ID, `{"status":"acknowledged"}`, false)
	if w.Code != 200 || inc.Status != "acknowledged" {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, suffix := range []string{"postmortem", "lifecycle", "analyses"} {
		if w := regressionRequest("GET", "/api/v1/incidents/"+inc.ID+"/"+suffix, "", false); w.Code != 401 {
			t.Fatal(suffix, w.Code)
		}
	}
	t.Setenv("LEGACY_API_AUTH_REQUIRED", "true")
	for _, path := range paths {
		if w := regressionRequest("GET", path, "", false); w.Code != 401 {
			t.Fatal(path, w.Code)
		}
	}
	if w := regressionRequest("PATCH", "/api/v1/incidents/"+inc.ID, `{}`, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	for _, path := range paths {
		if w := regressionRequest("GET", path, "", true); w.Code != 200 {
			t.Fatal(path, w.Code)
		}
	}
	// Legacy GeneratedAt and resolved status are not lifecycle measurements.
	inc.Analysis = &AIAnalysis{GeneratedAt: time.Now().Format(time.RFC3339)}
	inc.Status = "resolved"
	w = regressionRequest("GET", "/api/v1/analytics/summary", "", true)
	var result struct {
		Analysis struct {
			Samples int
			Mean    *float64 `json:"mean_seconds"`
		}
		Recovery struct{ Samples int } `json:"verified_recovery"`
	}
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Analysis.Samples != 0 || result.Analysis.Mean != nil || result.Recovery.Samples != 0 {
		t.Fatal(w.Body.String())
	}
}
func TestRegressionMemoryPeriodAndBound(t *testing.T) {
	inc := regressionMemory(t)
	inc.CreatedAt = time.Now().Add(-8 * 24 * time.Hour)
	r := httptest.NewRequest("GET", "/?period=7d", nil)
	rows, bounded, err := metricSnapshot(r)
	if err != nil || bounded || len(rows) != 0 {
		t.Fatal(rows, bounded, err)
	}
	r = httptest.NewRequest("GET", "/?period=30d", nil)
	for i := 0; i < 1001; i++ {
		id := fmt.Sprint(i)
		store.incidents[id] = &Incident{ID: id, CreatedAt: time.Now()}
	}
	rows, bounded, err = metricSnapshot(r)
	if err != nil || !bounded || len(rows) != 1000 {
		t.Fatal(len(rows), bounded, err)
	}
}
func TestRegressionLifecycleNoOpWrites(t *testing.T) {
	disk := regressionRedis(t)
	t.Setenv("REPORTS_ENABLED", "false")
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "false")
	now := time.Now().UTC()
	inc := &Incident{ID: newStableID("review-life"), Status: "active", CreatedAt: now, Alerts: []*Alert{{ID: newStableID("alert"), StartsAt: now.Add(-time.Minute).Format(time.RFC3339Nano), ReceivedAt: now}}}
	regressionIncident(t, disk, inc)
	t.Setenv("LIFECYCLE_ENABLED", "false")
	recordIngestLifecycle(inc)
	if disk.rdb.Exists(context.Background(), "igpt:lifecycle:"+inc.ID).Val() != 0 {
		t.Fatal("default ingestion wrote lifecycle")
	}
	t.Setenv("LIFECYCLE_ENABLED", "true")
	for i := 0; i < 3; i++ {
		recordIngestLifecycle(inc)
	}
	l, err := readLifecycle(context.Background(), inc)
	if err != nil || l.Revision != 1 {
		t.Fatal(l, err)
	}
	keys, err := disk.rdb.Keys(context.Background(), "igpt:lifecycle:history:"+inc.ID+":*").Result()
	if err != nil || len(keys) != 1 {
		t.Fatal(keys, err)
	}
	if ttl := disk.rdb.TTL(context.Background(), keys[0]).Val(); ttl != -1 {
		t.Fatal(ttl)
	}
	t.Setenv("LIFECYCLE_HISTORY_RETENTION_DAYS", "7")
	inc.Alerts[0].StartsAt = now.Add(-2 * time.Minute).Format(time.RFC3339Nano)
	recordIngestLifecycle(inc)
	l, err = readLifecycle(context.Background(), inc)
	if err != nil || l.Revision != 2 {
		t.Fatal(l, err)
	}
	if ttl := disk.rdb.TTL(context.Background(), fmt.Sprintf("igpt:lifecycle:history:%s:2", inc.ID)).Val(); ttl < 6*24*time.Hour {
		t.Fatal(ttl)
	}
	if ttl := disk.rdb.TTL(context.Background(), "igpt:lifecycle:"+inc.ID).Val(); ttl != -1 {
		t.Fatal(ttl)
	}
	// A duplicate delivery must not invalidate an editor's revision.
	recordIngestLifecycle(inc)
	if err := mutateLifecycle(context.Background(), inc, func(current *Lifecycle) error {
		if current.Revision != l.Revision {
			return redis.TxFailedErr
		}
		current.LastActor = "operator"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRegressionReportJSONCorruption(t *testing.T) {
	disk := regressionRedis(t)
	ctx := context.Background()
	now := time.Now().UTC()
	inc := &Incident{ID: newStableID("review-cjson"), Status: "active", CreatedAt: now, Analysis: &AIAnalysis{Recommendations: []Recommendation{}, Similar: []SimilarInc{}}}
	regressionIncident(t, disk, inc)
	if _, err := disk.GetIncident(inc.ID); err != nil {
		t.Fatal("valid record rejected before report", err)
	}
	jid, rid := newStableID("review-job"), newStableID("review-report")
	e := jobEnvelope{Job: DurableJob{ID: jid, IncidentID: inc.ID, Kind: "report", Status: "running"}, Owner: "review-owner", Lease: now.Add(time.Minute).UnixMilli(), Input: AnalysisInput{Report: &ReportSource{ReportID: rid, IncidentID: inc.ID, Kind: "postmortem"}}}
	raw, _ := json.Marshal(e)
	disk.rdb.Set(ctx, "igpt:job:"+jid, raw, time.Minute)
	report, _ := json.Marshal(ReportVersion{Version: 1, ID: rid, IncidentID: inc.ID, Kind: "postmortem", Revision: 1})
	t.Cleanup(func() {
		disk.rdb.Del(ctx, "igpt:job:"+jid, "igpt:report:"+rid, "igpt:report:"+rid+":revision:1", reportIndex(inc.ID, "postmortem"))
	})
	n, err := finishReportScript.Run(ctx, disk.rdb, []string{"igpt:job:" + jid, jobsDue, "igpt:report:" + rid, reportIndex(inc.ID, "postmortem")}, e.Owner, now.UnixMilli(), "complete", "", now.Add(time.Second).UnixMilli(), now.Format(time.RFC3339Nano), report, 3600).Int()
	if err != nil || n != 1 {
		t.Fatal(n, err)
	}
	// A stale lease cannot publish or create the postmortem marker.
	disk.rdb.Del(ctx, "igpt:incident:postmortem:"+inc.ID)
	if n, err := finishReportScript.Run(ctx, disk.rdb, []string{"igpt:job:" + jid, jobsDue, "igpt:report:" + rid, reportIndex(inc.ID, "postmortem")}, "stale-owner", now.UnixMilli(), "complete", "", now.UnixMilli(), now.Format(time.RFC3339Nano), report, 3600).Int(); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if disk.rdb.Exists(ctx, "igpt:incident:postmortem:"+inc.ID).Val() != 0 {
		t.Fatal("stale owner published metadata")
	}
	disk.rdb.Set(ctx, "igpt:incident:postmortem:"+inc.ID, "1", 0)

	after, err := disk.GetIncident(inc.ID)
	if err != nil || !after.HasPostmortem {
		t.Fatal(after, err)
	}
	rawIncident := disk.rdb.Get(ctx, keyIncident+inc.ID).Val()
	if !strings.Contains(rawIncident, `"recommendations":[]`) || !strings.Contains(rawIncident, `"similar_incidents":[]`) {
		t.Fatal(rawIncident)
	}
	if _, err := disk.ListIncidents(); err != nil {
		t.Fatal(err)
	}
	if w := regressionRequest("GET", "/api/v1/dashboard/active", "", false); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Compatibility for data already rewritten by the older Lua implementation.
	legacy := strings.ReplaceAll(strings.ReplaceAll(rawIncident, `"recommendations":[]`, `"recommendations":{}`), `"similar_incidents":[]`, `"similar_incidents":{}`)
	disk.rdb.Set(ctx, keyIncident+inc.ID, legacy, 0)
	if got, err := disk.GetIncident(inc.ID); err != nil || !got.HasPostmortem || len(got.Analysis.Similar) != 0 {
		t.Fatal(got, err)
	}
	if _, err := disk.ListIncidents(); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeIncident([]byte(strings.Replace(legacy, `"similar_incidents":{}`, `"similar_incidents":{"bad":1}`, 1))); err == nil {
		t.Fatal("unexpected malformed record accepted")
	}
	// Publishing metadata does not overwrite concurrent incident changes.
	inc.Status = "acknowledged"
	if err := disk.SaveIncident(inc); err != nil {
		t.Fatal(err)
	}
	if got, err := disk.GetIncident(inc.ID); err != nil || !got.HasPostmortem || got.Status != "acknowledged" {
		t.Fatal(got, err)
	}
}

type regressionCountHook struct{ count atomic.Int64 }

func (h *regressionCountHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *regressionCountHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *regressionCountHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, c redis.Cmder) error {
		if c.Name() == "zrevrange" && fmt.Sprint(c.Args()[1]) == keyIncidentIdx {
			h.count.Add(1)
		}
		return next(ctx, c)
	}
}
func TestRegressionDoubleIncidentScan(t *testing.T) {
	disk := regressionRedis(t)
	inc := &Incident{ID: newStableID("review-scan"), Status: "active", CreatedAt: time.Now().UTC()}
	regressionIncident(t, disk, inc)
	hook := &regressionCountHook{}
	disk.rdb.AddHook(hook)
	w := regressionRequest("GET", "/api/v1/dashboard/active", "", false)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if n := hook.count.Load(); n != 1 {
		t.Fatal("expected one index read", n)
	}

}
func TestRegressionRewrittenHostOrigin(t *testing.T) {
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	t.Setenv("ALLOWED_ORIGINS", "http://127.0.0.1:8088,https://demo.example")
	for _, tc := range []struct {
		origin  string
		allowed bool
	}{{"http://127.0.0.1:8088", true}, {"https://demo.example:443", true}, {"http://incidentgpt-pr20.test:80", true}, {"https://evil.example", false}, {"null", false}, {"http://127.0.0.1:8088#fragment", false}, {"http://127.0.0.1:8088?", false}} {
		r := httptest.NewRequest("POST", "http://incidentgpt-pr20.test/api/v1/incidents/fixture/postmortem", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-Forwarded-Host", "evil.example")
		r.SetBasicAuth("admin", "pw")
		w := httptest.NewRecorder()
		if got := changeReadAuth(w, r); got != tc.allowed {
			t.Fatal(tc.origin, got, w.Code)
		}
	}
	for _, invalid := range []string{"*", "https://*.example", "https://demo.example/", "https://demo.example,"} {
		t.Setenv("ALLOWED_ORIGINS", invalid)
		if _, err := allowedBrowserOrigins(); err == nil {
			t.Fatal("invalid allowlist accepted", invalid)
		}
	}
}
func TestRegressionNilReportJobStaysRunning(t *testing.T) {
	disk := regressionRedis(t)
	ctx := context.Background()
	jid := newStableID("review-nil-job")
	e := &jobEnvelope{Job: DurableJob{ID: jid, IncidentID: newStableID("review-incident"), Kind: "report", Status: "running"}, Owner: "owner", Lease: time.Now().Add(time.Minute).UnixMilli()}
	raw, _ := json.Marshal(e)
	key := "igpt:job:" + jid
	disk.rdb.Set(ctx, key, raw, time.Minute)
	defer disk.rdb.Del(ctx, key)
	runReportJob(ctx, e)
	stored, err := disk.rdb.Get(ctx, key).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	var after jobEnvelope
	if json.Unmarshal(stored, &after) != nil || after.Job.Status != "failed" || after.Job.ErrorCode != "invalid_report_snapshot" {
		t.Fatal(string(stored))
	}

}

func TestRegressionIncidentStorageErrorIsVisible(t *testing.T) {
	disk := regressionRedis(t)
	ctx := context.Background()
	id := newStableID("bad-storage")
	key := keyIncident + id
	defer disk.rdb.Del(ctx, key)
	disk.rdb.HSet(ctx, key, "unexpected", "hash")
	if _, err := disk.GetIncident(id); err == nil || err == redis.Nil {
		t.Fatal("wrong Redis type treated as missing", err)
	}
}
