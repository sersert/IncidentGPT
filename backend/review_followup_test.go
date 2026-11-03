package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func followupRedis(t *testing.T) *Storage {
	t.Helper()
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires disposable Redis, logical DB 12")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 12, ContextTimeoutEnabled: true})
	ctx := context.Background()
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	old := store
	disk := &Storage{rdb: rdb, ctx: ctx}
	store = &Store{disk: disk}
	t.Cleanup(func() { rdb.FlushDB(ctx); rdb.Close(); store = old })
	return disk
}
func followupMemory(t *testing.T) {
	old := store
	t.Cleanup(func() { store = old })
	store = &Store{incidents: map[string]*Incident{}, alerts: map[string]*Alert{}}
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "false")
	t.Setenv("MANUAL_REGROUP_ENABLED", "false")
}
func TestResolveDeliveryUsesAllAffectedIncidents(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(fmt.Sprint(persistent), func(t *testing.T) {
			followupMemory(t)
			if persistent {
				followupRedis(t)
			}
			a := EnrichedAlert{Fingerprint: "a", Status: "firing", Labels: map[string]string{"namespace": "api", "alertname": "A"}}
			b := EnrichedAlert{Fingerprint: "b", Status: "firing", Labels: map[string]string{"namespace": "jobs", "alertname": "B"}}
			first := ingestGroup([]EnrichedAlert{a}, "a")
			second := ingestGroup([]EnrichedAlert{b}, "b")
			ingestGroup([]EnrichedAlert{a, b}, "combined")
			a.Status = "resolved"
			b.Status = "resolved"
			ingestGroup([]EnrichedAlert{a, b}, "combined")
			for _, id := range []string{first.ID, second.ID} {
				if got := store.Get(id); got.Status != "resolved" || got.Alerts[0].Status != "resolved" {
					t.Fatal("combined resolve left active incident", got)
				}
			}
			a.Status = "firing"
			recurred := ingestGroup([]EnrichedAlert{a}, "a")
			if recurred == nil || recurred.ID == first.ID || len(recurred.Alerts) != 1 {
				t.Fatal("closed fingerprint could not recur", recurred)
			}
		})
	}
}
func TestPartialAndMixedResolvePreserveFiringMembers(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(fmt.Sprint(persistent), func(t *testing.T) {
			followupMemory(t)
			if persistent {
				followupRedis(t)
			}
			a := EnrichedAlert{Fingerprint: "a", Status: "firing", Labels: map[string]string{"namespace": "api", "alertname": "A"}}
			b := a
			b.Fingerprint = "b"
			c := a
			c.Fingerprint = "c"
			first := ingestGroup([]EnrichedAlert{a, b}, "first")
			second := ingestGroup([]EnrichedAlert{c}, "second")
			a.Status = "resolved"
			ingestGroup([]EnrichedAlert{a, c}, "mixed")
			got := store.Get(first.ID)
			if got.Status == "resolved" || got.Alerts[0].Status != "resolved" || got.Alerts[1].Status != "firing" {
				t.Fatal("partial resolve closed unrelated member", got)
			}
			if got := store.Get(second.ID); got.Status == "resolved" || len(got.Alerts) != 1 {
				t.Fatal("mixed batch lost firing member", got)
			}
			b.Status = "resolved"
			ingestGroup([]EnrichedAlert{b}, "first")
			if store.Get(first.ID).Status != "resolved" {
				t.Fatal("fully resolved composition remained active")
			}
		})
	}
}
func TestMissingIncidentStartsNonEmptyEpisode(t *testing.T) {
	followupMemory(t)
	gone := "deleted"
	a := &Alert{ID: "old", Fingerprint: "orphan", Status: "firing", IncidentID: &gone}
	store.PutAlert(a)
	inc := ingestGroup([]EnrichedAlert{{Fingerprint: "orphan", Status: "firing", Labels: map[string]string{"namespace": "api"}}}, "group")
	if inc == nil || len(inc.Alerts) != 1 || inc.Alerts[0].ID == a.ID {
		t.Fatal("orphan produced empty incident", inc)
	}
}
func TestRepresentativeScopeNeverMixesNamespaces(t *testing.T) {
	inc := &Incident{ClusterID: "prod", Namespace: "stale", Alerts: []*Alert{
		{Namespace: "jobs", Labels: map[string]string{"namespace": "jobs", "deployment": "job"}},
		{Namespace: "api", Labels: map[string]string{"namespace": "api", "deployment": "web", "pod": "web-7f", "node": "node-api"}},
	}}
	refreshCompositionContext(inc)
	scope := incidentDiagnosticScope(inc)
	if scope.Namespace != "api" || scope.Workload != "web" || scope.Pod != "web-7f" || scope.Node != "node-api" {
		t.Fatal(scope)
	}
	inc.Labels["pod"] = "changed"
	if inc.Alerts[1].Labels["pod"] != "web-7f" || inc.Context.Labels["pod"] != "web-7f" {
		t.Fatal("composition context aliases alert labels")
	}
	inc.Alerts = inc.Alerts[:1]
	refreshCompositionContext(inc)
	scope = incidentDiagnosticScope(inc)
	if scope.Namespace != "jobs" || scope.Pod != "" || scope.Node != "" {
		t.Fatal("removed pod remained in another namespace", scope)
	}
}
func TestTopologyMutationAuthAndDisabledReads(t *testing.T) {
	followupMemory(t)
	t.Setenv("SERVICE_GRAPH_ENABLED", "false")
	t.Setenv("SERVICE_GRAPH_FILE", "")
	t.Setenv("TIMELINE_READ_USER", "")
	t.Setenv("TIMELINE_READ_PASSWORD", "")
	r := httptest.NewRequest("GET", "/api/v1/services/graph?incident_id=unknown", nil)
	w := httptest.NewRecorder()
	serviceGraph(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"enabled":false`) || w.Header().Get("WWW-Authenticate") != "" {
		t.Fatal(w.Code, w.Body.String())
	}
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	t.Setenv("ALLOWED_ORIGINS", "http://127.0.0.1:8088")
	for _, tc := range []struct {
		origin, contentType string
		status              int
	}{
		{"https://evil.example", "text/plain", 403}, {"https://evil.example", "application/json", 403},
		{"http://127.0.0.1:8088", "text/plain", 415}, {"http://127.0.0.1:8088", "application/json", 204},
		{"", "application/json; charset=utf-8", 204},
	} {
		r := httptest.NewRequest("POST", "http://internal.example/regroup", strings.NewReader(`{}`))
		r.SetBasicAuth("admin", "pw")
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", tc.contentType)
		r.Header.Set("X-Auth-User", "forged")
		w := httptest.NewRecorder()
		corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sharedAdminMutationAuth(w, r) {
				w.WriteHeader(204)
			}
		})).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(tc, w.Code, w.Body.String())
		}
		if w.Header().Get("Access-Control-Allow-Origin") == "*" {
			t.Fatal("wildcard CORS for authenticated mutation")
		}
	}
}
func TestFileGraphRejectsAPIReplacement(t *testing.T) {
	followupRedis(t)
	t.Setenv("SERVICE_GRAPH_FILE", "/configured/dependencies.json")
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	r := httptest.NewRequest("PUT", "/api/v1/services/graph", strings.NewReader(`{}`))
	r.SetBasicAuth("admin", "pw")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleGraphWrite(w, r)
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := loadGraphConfiguration(context.Background(), "/missing-file"); err == nil {
		t.Fatal("file load error hidden")
	}
}
func TestAlertFingerprintIndexBackfillsAndKeepsNewEpisode(t *testing.T) {
	disk := followupRedis(t)
	ctx := context.Background()
	old := &Alert{ID: "legacy-old", Fingerprint: "fp", Status: "firing", ReceivedAt: time.Now().Add(-time.Hour)}
	newer := *old
	newer.ID = "legacy-new"
	newer.ReceivedAt = time.Now()
	for _, a := range []*Alert{old, &newer} {
		raw, _ := json.Marshal(a)
		disk.rdb.Set(ctx, keyAlert+a.ID, raw, 0)
		disk.rdb.ZAdd(ctx, keyAlertIdx, redis.Z{Score: float64(a.ReceivedAt.Unix()), Member: a.ID})
	}
	if err := disk.ensureAlertIndex(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := disk.FindAlertByFingerprint("fp")
	if err != nil || got.ID != newer.ID {
		t.Fatal(got, err)
	}
	old.Status = "resolved"
	if err := disk.SaveAlert(old); err != nil {
		t.Fatal(err)
	}
	got, err = disk.FindAlertByFingerprint("fp")
	if err != nil || got.ID != newer.ID {
		t.Fatal("old resolution removed new episode", got, err)
	}
	// Even a broken history key cannot affect a ready indexed lookup.
	disk.rdb.Del(ctx, keyAlertIdx)
	disk.rdb.Set(ctx, keyAlertIdx, "not a zset", 0)
	got, err = disk.FindAlertByFingerprint("fp")
	if err != nil || got.ID != newer.ID {
		t.Fatal("lookup scanned history", got, err)
	}
	if _, err := disk.FindAlertByFingerprint("absent"); err != redis.Nil {
		t.Fatal("new fingerprint scanned history", err)
	}
}
func TestRegroupBindingExpiryAndSharedAdminActor(t *testing.T) {
	disk := followupRedis(t)
	ctx := context.Background()
	t.Setenv("MANUAL_REGROUP_ENABLED", "true")
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "true")
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	id := "source"
	a := &Alert{ID: "a", Fingerprint: "a", Status: "firing", IncidentID: &id, Namespace: "api", Labels: map[string]string{"namespace": "api"}}
	inc := &Incident{ID: id, ClusterID: "prod", Status: "active", CreatedAt: time.Now(), Alerts: []*Alert{a}, Fingerprints: map[string]bool{"a": true}}
	store.Put(inc)
	store.PutAlert(a)
	r := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"alert_id":"a"}`))
	r.SetPathValue("id", id)
	r.SetBasicAuth("admin", "pw")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Auth-User", "forged")
	w := httptest.NewRecorder()
	handleRegroup(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if store.Get(id).Status != "resolved" {
		t.Fatal("empty source remains active")
	}
	ttl := disk.rdb.TTL(ctx, "igpt:regroup:binding:a").Val()
	if ttl <= 0 || ttl > manualBindingTTL {
		t.Fatal(ttl)
	}
	target := disk.rdb.Get(ctx, "igpt:regroup:binding:a").Val()
	moved := store.Get(target)
	if !strings.HasPrefix(moved.ID, "INC-") || moved.History[len(moved.History)-1].By != "admin" {
		t.Fatal("forged actor or inconsistent ID", moved)
	}
	t.Setenv("MANUAL_REGROUP_ENABLED", "false")
	if binding, err := disk.manualBinding(ctx, "a"); err != nil || binding != "" {
		t.Fatal("disabled flag still separates fingerprints", binding, err)
	}
	t.Setenv("MANUAL_REGROUP_ENABLED", "true")
	moved.Status = "resolved"
	store.Put(moved)
	if binding, err := disk.manualBinding(ctx, "a"); err != nil || binding != "" || disk.rdb.Exists(ctx, "igpt:regroup:binding:a").Val() != 0 {
		t.Fatal("closed target kept manual pin", binding, err)
	}
}

func TestDurableBatchSchedulesEachActualOwnerOnce(t *testing.T) {
	for _, converge := range []bool{false, true} {
		t.Run(fmt.Sprint(converge), func(t *testing.T) {
			disk := followupRedis(t)
			ctx := context.Background()
			t.Setenv("DURABLE_ANALYSIS_ENABLED", "true")
			t.Setenv("MANUAL_REGROUP_ENABLED", "true")
			t.Setenv("ANALYSIS_SERVICE_TOKEN", "service")
			sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req sanitizerPayloadRequest
				json.NewDecoder(r.Body).Decode(&req)
				writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": req.Payload})
			}))
			defer sanitizer.Close()
			t.Setenv("SANITIZER_URL", sanitizer.URL)
			t.Setenv("SANITIZER_AUTH_SHARED_SECRET", "hmac")
			cluster := newStableID("cluster")
			a := EnrichedAlert{ClusterName: cluster, Fingerprint: "a", Status: "firing", Labels: map[string]string{"namespace": "api", "alertname": "OwnedA"}}
			b := a
			b.Fingerprint = "b"
			b.Labels = map[string]string{"namespace": "jobs", "alertname": "OwnedB"}
			first := ingestGroup([]EnrichedAlert{a}, "a")
			second := first
			if converge {
				ingestGroup([]EnrichedAlert{a, b}, "combined")
				disk.rdb.Set(ctx, "igpt:regroup:binding:cluster-"+digest(cluster, "a"), first.ID, manualBindingTTL)
			} else {
				second = ingestGroup([]EnrichedAlert{b}, "b")
			}
			payload, _ := json.Marshal(map[string]any{"group_key": "combined", "alerts": []EnrichedAlert{a, b}})
			send := func() *httptest.ResponseRecorder {
				r := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
				r.Header.Set("Authorization", "Bearer service")
				r.Header.Set("X-IncidentGPT-Batch-ID", "owned-batch")
				w := httptest.NewRecorder()
				handleDurableIngest(w, r)
				return w
			}
			w := send()
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			var accepted struct {
				Jobs []string `json:"job_ids"`
			}
			json.Unmarshal(w.Body.Bytes(), &accepted)
			expected := 2
			if converge {
				expected = 1
			}
			if len(accepted.Jobs) != expected {
				t.Fatal("one job per final owner violated", w.Body.String())
			}
			for _, jid := range accepted.Jobs {
				raw, err := disk.rdb.Get(ctx, "igpt:job:"+jid).Bytes()
				if err != nil {
					t.Fatal(err)
				}
				var job jobEnvelope
				if json.Unmarshal(raw, &job) != nil {
					t.Fatal(string(raw))
				}
				alerts := 0
				for _, e := range job.Input.Evidence {
					if e.Source != "alert" {
						continue
					}
					alerts++
					if !converge && ((job.Job.IncidentID == first.ID && strings.Contains(e.Excerpt, "OwnedB")) || (job.Job.IncidentID == second.ID && strings.Contains(e.Excerpt, "OwnedA"))) {
						t.Fatal("foreign incident evidence entered job", e)
					}
				}
				if alerts != 3-expected {
					t.Fatal("lost/duplicated incident evidence", alerts)
				}
			}
			if w := send(); w.Code != 202 || disk.rdb.ZCard(ctx, jobsDue).Val() != int64(expected) {
				t.Fatal("batch retry duplicated jobs", w.Code, w.Body.String())
			}
		})
	}
}

func TestAlertIndexRebuildAfterOlderWriterRollback(t *testing.T) {
	disk := followupRedis(t)
	ctx := context.Background()
	if err := disk.rebuildAlertIndex(ctx); err != nil {
		t.Fatal(err)
	}
	// An older binary knows nothing about the new ready marker or index.
	legacy := &Alert{ID: "from-older-binary", Fingerprint: "legacy-after-ready", Status: "firing", ReceivedAt: time.Now()}
	raw, _ := json.Marshal(legacy)
	disk.rdb.Set(ctx, keyAlert+legacy.ID, raw, 0)
	disk.rdb.ZAdd(ctx, keyAlertIdx, redis.Z{Member: legacy.ID, Score: float64(legacy.ReceivedAt.Unix())})
	if err := disk.rebuildAlertIndex(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := disk.FindAlertByFingerprint(legacy.Fingerprint)
	if err != nil || got.ID != legacy.ID {
		t.Fatal("persistent marker hid old-writer alert", got, err)
	}
}

func TestLegacyStorageDoesNotEnableNewIndexByDefault(t *testing.T) {
	disk := followupRedis(t)
	ctx := context.Background()
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "false")
	t.Setenv("MANUAL_REGROUP_ENABLED", "false")
	alert := &Alert{ID: "legacy-default", Fingerprint: "legacy-default", Status: "firing"}
	if err := disk.SaveAlert(alert); err != nil {
		t.Fatal(err)
	}
	if disk.rdb.Exists(ctx, alertFingerprintKey(alert.Fingerprint), alertIndexReady).Val() != 0 {
		t.Fatal("default storage created opt-in metadata")
	}
}

func TestRepeatedGraphObservationHasStableProvenance(t *testing.T) {
	d := GraphDocument{Version: 1, ClusterID: "prod", Nodes: []GraphNode{{ID: "pod", ClusterID: "prod", Namespace: "api", Kind: "pod", Name: "web-1"}, {ID: "node", ClusterID: "prod", Kind: "node", Name: "node-1"}}, Edges: []GraphEdge{{From: "pod", To: "node", Kind: "placement"}}}
	if err := normalizeGraph(&d, true); err != nil {
		t.Fatal(err)
	}
	reference := d.Edges[0].EvidenceRef
	if err := normalizeGraph(&d, true); err != nil {
		t.Fatal(err)
	}
	if d.Edges[0].EvidenceRef != reference {
		t.Fatal("repeated timestamp created a new evidence key")
	}
}
