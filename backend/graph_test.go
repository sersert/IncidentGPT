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
)

func TestGraphTypesAndClusterIdentity(t *testing.T) {
	d := GraphDocument{Version: 1, ClusterID: "east", Nodes: []GraphNode{{ID: "a", ClusterID: "east", Namespace: "api", Name: "web", Kind: "workload"}, {ID: "b", ClusterID: "east", Namespace: "data", Name: "db", Kind: "database"}}, Edges: []GraphEdge{{From: "a", To: "b", Kind: "application_dependency"}}}
	if normalizeGraph(&d, true) == nil {
		t.Fatal("metadata allowed invented dependency")
	}
	if normalizeGraph(&d, false) != nil {
		t.Fatal("valid declaration rejected")
	}
	if d.Nodes[0].ID == graphNodeID("west", "api", "workload", "web") || d.Edges[0].Observed || d.Edges[0].EvidenceRef == "" {
		t.Fatal("identity/provenance invalid")
	}
}
func TestRegroupPreservesHistoryAndFencesCompletion(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
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
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	id := newStableID("fixture")
	cluster := newStableID("cluster")
	movedFingerprint := newStableID("fp")
	otherFingerprint := newStableID("fp")
	a := &Alert{ID: newStableID("alert"), Fingerprint: "cluster-" + digest(cluster, movedFingerprint), IncidentID: &id, Status: "firing"}
	inc := &Incident{ID: id, ClusterID: cluster, Alerts: []*Alert{a, {ID: newStableID("alert")}}, CreatedAt: time.Now().UTC(), Fingerprints: map[string]bool{a.Fingerprint: true}}
	store.Put(inc)
	store.PutAlert(a)
	ctx := context.Background()
	aid := newStableID("analysis")
	disk.rdb.Set(ctx, analysisKey(aid), `{"original":true}`, 0)
	disk.rdb.Set(ctx, "igpt:analysis:requested:"+id, aid, 0)
	raw, _ := json.Marshal(map[string]string{"alert_id": a.ID})
	r := httptest.NewRequest("POST", "/", bytes.NewReader(raw))
	r.SetPathValue("id", id)
	r.SetBasicAuth("admin", "pw")
	w := httptest.NewRecorder()
	handleRegroup(w, r)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	source := store.Get(id)
	target := store.Get(result["target_incident_id"])
	if len(source.Alerts) != 1 || len(target.Alerts) != 1 || *target.Alerts[0].IncidentID != target.ID || target.HistorySources[0] != id {
		t.Fatal("composition/history lost")
	}
	if disk.rdb.Exists(ctx, analysisKey(aid)).Val() != 1 || disk.rdb.Exists(ctx, "igpt:analysis:requested:"+id).Val() != 0 || disk.rdb.Get(ctx, keyFingerprint+a.Fingerprint).Val() != target.ID {
		t.Fatal("old result not fenced or binding lost")
	}
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sanitizerPayloadRequest
		json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": req.Payload})
	}))
	defer sanitizer.Close()
	t.Setenv("SANITIZER_URL", sanitizer.URL)
	t.Setenv("SANITIZER_AUTH_SHARED_SECRET", "hmac")
	t.Setenv("ANALYSIS_SERVICE_TOKEN", "service")
	batch, _ := json.Marshal(map[string]any{"group_key": "same-original-group", "alerts": []EnrichedAlert{{ClusterName: cluster, Fingerprint: movedFingerprint, Labels: map[string]string{"namespace": "api", "alertname": "Moved"}}, {ClusterName: cluster, Fingerprint: otherFingerprint, Labels: map[string]string{"namespace": "api", "alertname": "Other"}}}})
	ingest := httptest.NewRequest("POST", "/", bytes.NewReader(batch))
	ingest.Header.Set("Authorization", "Bearer service")
	ingest.Header.Set("X-IncidentGPT-Batch-ID", newStableID("batch"))
	response := httptest.NewRecorder()
	handleDurableIngest(response, ingest)
	if response.Code != 202 {
		t.Fatal("partition ingestion", response.Code, response.Body.String())
	}
	var accepted struct {
		Incidents []string `json:"incident_ids"`
	}
	json.Unmarshal(response.Body.Bytes(), &accepted)
	if len(accepted.Incidents) != 2 {
		t.Fatal("manual split recombined on ingest", response.Body.String())
	}
	if len(store.Get(target.ID).Alerts) != 1 {
		t.Fatal("unmoved alert attached to manually split target")
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", bytes.NewReader(raw))
	r.SetPathValue("id", id)
	r.SetBasicAuth("admin", "pw")
	handleRegroup(w, r)
	if w.Code != 400 {
		t.Fatal("duplicate move accepted")
	}
}

func TestGraphAPIPersistenceReplacementAndSanitizerBoundary(t *testing.T) {
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
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
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
	d := GraphDocument{Version: 1, ClusterID: cluster, Nodes: []GraphNode{{ID: "app", ClusterID: cluster, Namespace: "api", Name: "web", Kind: "workload"}, {ID: "db", ClusterID: cluster, Namespace: "data", Name: "orders", Kind: "database"}}, Edges: []GraphEdge{{From: "app", To: "db", Kind: "application_dependency"}}}
	write := func(method string, d GraphDocument) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(d)
		r := httptest.NewRequest(method, "/api/v1/services/graph", bytes.NewReader(raw))
		if method == "POST" {
			r.Header.Set("Authorization", "Bearer service")
		} else {
			r.SetBasicAuth("admin", "pw")
		}
		w := httptest.NewRecorder()
		handleGraphWrite(w, r)
		return w
	}
	if w := write("PUT", d); w.Code != 200 {
		t.Fatal("graph declaration", w.Code, w.Body.String())
	}
	read := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/v1/services/graph?cluster_id="+cluster, nil)
		r.SetBasicAuth("admin", "pw")
		w := httptest.NewRecorder()
		serviceGraph(w, r)
		return w
	}
	w := read()
	var graph GraphDocument
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &graph) != nil || len(graph.Edges) != 1 || graph.Nodes[0].Status != "unknown" {
		t.Fatal("persisted graph unavailable", w.Body.String())
	}
	edge := graph.Edges[0]
	if disk.rdb.Exists(context.Background(), "igpt:graph:evidence:"+edge.EvidenceRef).Val() != 1 {
		t.Fatal("graph provenance not persisted")
	}
	observed := GraphDocument{Version: 1, ClusterID: cluster, Nodes: []GraphNode{{ID: "pod", ClusterID: cluster, Namespace: "api", Name: "web-1", Kind: "pod"}, {ID: "node", ClusterID: cluster, Name: "node-1", Kind: "node"}}, Edges: []GraphEdge{{From: "pod", To: "node", Kind: "placement"}}}
	if w := write("POST", observed); w.Code != 200 {
		t.Fatal("metadata observation", w.Code, w.Body.String())
	}
	d.Edges = nil
	if w := write("PUT", d); w.Code != 200 {
		t.Fatal("declaration withdrawal", w.Code, w.Body.String())
	}
	w = read()
	json.Unmarshal(w.Body.Bytes(), &graph)
	if len(graph.Edges) != 1 || graph.Edges[0].Kind != "placement" || disk.rdb.Exists(context.Background(), "igpt:graph:edge:"+edge.ID).Val() != 0 {
		t.Fatal("withdrawal left dependency or removed metadata", w.Body.String())
	}
	sanitizer.Close()
	if w := write("PUT", d); w.Code != 503 {
		t.Fatal("sanitizer outage accepted graph", w.Code)
	}
}

func TestDurableCompositionLimitAndClosedAlertRecurrence(t *testing.T) {
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
	t.Setenv("DURABLE_ANALYSIS_ENABLED", "true")
	t.Setenv("ANALYSIS_SERVICE_TOKEN", "service")
	t.Setenv("SANITIZER_AUTH_SHARED_SECRET", "hmac")
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sanitizerPayloadRequest
		json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": req.Payload})
	}))
	defer sanitizer.Close()
	t.Setenv("SANITIZER_URL", sanitizer.URL)
	cluster, id := newStableID("cluster"), newStableID("fixture")
	upstream := newStableID("fp")
	inc := &Incident{ID: id, ClusterID: cluster, Status: "active", Namespace: "api", CreatedAt: time.Now().UTC(), Fingerprints: map[string]bool{}}
	for i := 0; i < 50; i++ {
		fp := "cluster-" + digest(cluster, newStableID("fp"))
		if i == 0 {
			fp = "cluster-" + digest(cluster, upstream)
		}
		a := &Alert{ID: newStableID("alert"), Status: "firing", Fingerprint: fp, IncidentID: &id}
		inc.Alerts = append(inc.Alerts, a)
		inc.Fingerprints[fp] = true
		store.PutAlert(a)
	}
	store.Put(inc)
	payload, _ := json.Marshal(map[string]any{"group_key": cluster + "/api", "alerts": []EnrichedAlert{{Status: "firing", ClusterName: cluster, Fingerprint: upstream, Labels: map[string]string{"namespace": "api", "alertname": "Repeat"}}, {Status: "firing", ClusterName: cluster, Fingerprint: newStableID("fp"), Labels: map[string]string{"namespace": "api", "alertname": "New"}}}})
	ingest := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewReader(payload))
		r.Header.Set("Authorization", "Bearer service")
		r.Header.Set("X-IncidentGPT-Batch-ID", newStableID("batch"))
		w := httptest.NewRecorder()
		handleDurableIngest(w, r)
		return w
	}
	if w := ingest(); w.Code != 503 || !strings.Contains(w.Body.String(), "composition limit") || len(store.Get(id).Alerts) != 50 {
		t.Fatal("storm limit did not preserve composition", w.Code, w.Body.String())
	}
	inc.Status = "resolved"
	store.Put(inc)
	w := ingest()
	if w.Code != 202 {
		t.Fatal("closed incident recurrence rejected", w.Code, w.Body.String())
	}
	var accepted map[string]any
	json.Unmarshal(w.Body.Bytes(), &accepted)
	next := store.Get(accepted["incident_id"].(string))
	if next.ID == id || len(next.Alerts) != 2 {
		t.Fatal("closed firing alert did not create a new episode")
	}
	for _, a := range next.Alerts {
		stored, err := disk.GetAlert(a.ID)
		if err != nil || stored.IncidentID == nil || *stored.IncidentID != next.ID {
			t.Fatal("new standalone alert binding missing")
		}
	}
}
