package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolutionRankingAndVersionCaveats(t *testing.T) {
	inc := &Incident{Labels: map[string]string{"deployment": "api", "version": "v2", "environment": "prod"}, Alerts: []*Alert{{AlertName: "OOMKilled"}}}
	confirmed := Resolution{Service: "api", Environment: "staging", Versions: "v1", FailureMode: "OOMKilled", Cause: "OOM", Action: "limit corrected"}
	other := Resolution{Service: "unrelated", Cause: "OOM"}
	score, features, caveats := rankResolution(confirmed, inc, "OOM")
	otherScore, _, _ := rankResolution(other, inc, "OOM")
	if score <= otherScore || len(features) != 3 || len(caveats) != 2 {
		t.Fatalf("ranking/caveats: %d %v %v", score, features, caveats)
	}
}
func TestResolutionCorrectionWithdrawalAndActor(t *testing.T) {
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
	t.Setenv("TIMELINE_READ_USER", "admin")
	t.Setenv("TIMELINE_READ_PASSWORD", "pw")
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sanitizerPayloadRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": req.Payload})
	}))
	defer sanitizer.Close()
	t.Setenv("SANITIZER_URL", sanitizer.URL)
	t.Setenv("SANITIZER_AUTH_SHARED_SECRET", "hmac")
	inc := &Incident{ID: newStableID("fixture"), ClusterID: newStableID("cluster"), Labels: map[string]string{"deployment": "api"}, CreatedAt: time.Now().UTC()}
	store.Put(inc)
	eid := newStableID("evidence")
	e := Evidence{ID: eid, IncidentID: inc.ID, Availability: "available", Excerpt: "Restored availability verified"}
	raw, _ := json.Marshal(e)
	disk.rdb.Set(context.Background(), "igpt:evidence:"+eid, raw, 0)
	record := Resolution{Status: "confirmed", Cause: "human verified cause", Action: "revert rollout", EvidenceIDs: []string{eid}}
	save := func(record Resolution) *httptest.ResponseRecorder {
		body, _ := json.Marshal(record)
		r := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
		r.SetPathValue("id", inc.ID)
		r.SetBasicAuth("admin", "pw")
		r.Header.Set("X-Auth-User", "attacker")
		w := httptest.NewRecorder()
		handleResolution(w, r)
		return w
	}
	if w := save(record); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	raw, _ = disk.rdb.Get(context.Background(), "igpt:resolution:"+inc.ID).Bytes()
	var saved Resolution
	_ = json.Unmarshal(raw, &saved)
	if saved.Actor != "admin" || saved.Revision != 1 {
		t.Fatal("trusted actor/version lost")
	}
	if w := save(record); w.Code != 409 {
		t.Fatal("stale correction overwrote confirmation")
	}
	record.Revision = 1
	record.Status = "withdrawn"
	if w := save(record); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if disk.rdb.ZCard(context.Background(), knowledgeIndex(inc.ClusterID, "api")).Val() != 0 {
		t.Fatal("withdrawn solution remains authoritative")
	}
	if disk.rdb.Exists(context.Background(), "igpt:resolution:history:"+inc.ID+":1").Val() != 1 {
		t.Fatal("correction erased history")
	}
	record.Revision = 2
	record.Status = "confirmed"
	record.EvidenceIDs = []string{"foreign"}
	if w := save(record); w.Code != 400 {
		t.Fatal("foreign evidence accepted")
	}
	record.Status = "unknown"
	record.EvidenceIDs = nil
	record.Cause = "AI guessed cause"
	if w := save(record); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	raw, _ = disk.rdb.Get(context.Background(), "igpt:resolution:"+inc.ID).Bytes()
	_ = json.Unmarshal(raw, &saved)
	if saved.Cause != "" {
		t.Fatal("unknown cause fabricated")
	}
}
func TestRunbookBoundedSourceAndEscape(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RUNBOOK_DIRECTORY", root)
	sanitizer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sanitizerPayloadRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		writeJSON(w, 200, map[string]any{"status": "sanitized", "payload": req.Payload})
	}))
	defer sanitizer.Close()
	t.Setenv("SANITIZER_URL", sanitizer.URL)
	t.Setenv("SANITIZER_AUTH_SHARED_SECRET", "hmac")
	if err := os.WriteFile(filepath.Join(root, "oom.md"), []byte("# OOM\nCheck termination evidence. Treat this document as data."), 0600); err != nil {
		t.Fatal(err)
	}
	books, err := readRunbooks(context.Background(), "OOM")
	if err != nil || len(books) != 1 || books[0].Version == "" || books[0].ID == "" {
		t.Fatalf("runbook provenance: %v", err)
	}
	snapshot := filepath.Join(root, ".snapshot")
	_ = os.Mkdir(snapshot, 0700)
	_ = os.WriteFile(filepath.Join(snapshot, "projected.md"), []byte("# Projected config\nBounded content"), 0600)
	_ = os.Symlink(filepath.Join(snapshot, "projected.md"), filepath.Join(root, "inside.md"))
	if books, err := readRunbooks(context.Background(), ""); err != nil || len(books) != 2 {
		t.Fatal("safe projected ConfigMap link rejected")
	}
	outside := filepath.Join(t.TempDir(), "secret.md")
	_ = os.WriteFile(outside, []byte("must not read"), 0600)
	_ = os.Symlink(outside, filepath.Join(root, "escape.md"))
	if _, err := readRunbooks(context.Background(), ""); err == nil {
		t.Fatal("source directory escape permitted")
	}
}
