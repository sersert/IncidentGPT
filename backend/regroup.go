package main

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"net/http"
	"time"
)

// Move one alert, or split it into a new incident. Source records and their
// immutable evidence/analysis remain accessible through history_sources.
var regroupScript = redis.NewScript(`
if redis.call('GET',KEYS[1])~=ARGV[1] then return 0 end
local target=redis.call('GET',KEYS[2]);if ARGV[2]=='' then if target then return 0 end else if target~=ARGV[2] then return 0 end end
if redis.call('GET',KEYS[3])~=ARGV[3] then return 0 end
redis.call('DEL','igpt:analysis:requested:'..ARGV[10],'igpt:analysis:requested:'..ARGV[7],'igpt:analysis:latest:'..ARGV[10],'igpt:analysis:latest:'..ARGV[7]);redis.call('SET',KEYS[1],ARGV[4]);redis.call('SET',KEYS[2],ARGV[5]);redis.call('SET',KEYS[3],ARGV[6]);redis.call('SET',KEYS[4],ARGV[7]);redis.call('SET','igpt:regroup:binding:'..ARGV[11],ARGV[7]);redis.call('ZADD',KEYS[5],ARGV[8],ARGV[7]);redis.call('SET',KEYS[6],ARGV[9]);return 1`)

func handleRegroup(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil || !durableEnabled() {
		http.Error(w, "regrouping requires durable Redis mode", 503)
		return
	}
	var body struct {
		AlertID string `json:"alert_id"`
		Target  string `json:"target_incident_id"`
	}
	if decodeChange(r, &body) != nil || body.AlertID == "" {
		http.Error(w, "one alert and optional target required", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	owner := newStableID("regroup-owner")
	locked, err := store.disk.rdb.SetNX(ctx, "igpt:composition:lock", owner, 30*time.Second).Result()
	if err != nil || !locked {
		http.Error(w, "composition busy; retry", 409)
		return
	}
	defer redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`).Run(context.Background(), store.disk.rdb, []string{"igpt:composition:lock"}, owner)
	sourceID := r.PathValue("id")
	sourceRaw, err := store.disk.rdb.Get(ctx, keyIncident+sourceID).Bytes()
	var source Incident
	if err != nil || json.Unmarshal(sourceRaw, &source) != nil {
		http.Error(w, "source unavailable", 404)
		return
	}
	if len(source.Alerts) > 50 || incidentCluster(&source) == "unknown" || incidentCluster(&source) == "" {
		http.Error(w, "bounded known cluster required", 400)
		return
	}
	var moved *Alert
	remaining := []*Alert(nil)
	for _, a := range source.Alerts {
		if a.ID == body.AlertID {
			moved = a
		} else {
			remaining = append(remaining, a)
		}
	}
	if moved == nil {
		http.Error(w, "alert is not in source", 400)
		return
	}
	alertRaw, err := store.disk.rdb.Get(ctx, keyAlert+moved.ID).Bytes()
	if err != nil {
		http.Error(w, "alert unavailable", 503)
		return
	}
	var stored Alert
	if json.Unmarshal(alertRaw, &stored) != nil || stored.IncidentID == nil || *stored.IncidentID != source.ID {
		http.Error(w, "alert binding changed", 409)
		return
	}
	var target Incident
	var targetRaw []byte
	now := time.Now().UTC()
	if body.Target == "" {
		target = Incident{ID: newStableID("incident"), ClusterID: source.ClusterID, Title: moved.AlertName, Namespace: moved.Namespace, Source: moved.Source, Status: "active", Severity: moved.Severity, CreatedAt: now, UpdatedAt: now, Labels: moved.Labels, AnalysisMode: "structured"}
	} else {
		targetRaw, err = store.disk.rdb.Get(ctx, keyIncident+body.Target).Bytes()
		if err != nil || json.Unmarshal(targetRaw, &target) != nil {
			http.Error(w, "target unavailable", 404)
			return
		}
		if target.ID == source.ID || incidentCluster(&target) != incidentCluster(&source) || target.Status == "resolved" || len(target.Alerts) >= 50 {
			http.Error(w, "invalid target cluster, size or state", 400)
			return
		}
	}
	actor, _, _ := r.BasicAuth()
	auditID := newStableID("regroup")
	moved.IncidentID = &target.ID
	moved.CorrelationReason = "human move by " + actor + "; audit " + auditID
	source.Alerts = remaining
	source.AlertCount = len(remaining)
	source.UpdatedAt = now
	target.Alerts = append(target.Alerts, moved)
	target.AlertCount = len(target.Alerts)
	target.UpdatedAt = now
	target.HistorySources = appendUnique(target.HistorySources, source.ID)
	source.HistorySources = appendUnique(source.HistorySources, target.ID)
	for _, inc := range []*Incident{&source, &target} {
		inc.AnalysisSummary = nil
		inc.Analysis = nil
		inc.Confidence = nil
		inc.History = append(inc.History, StatusChange{Status: "composition_changed:" + auditID, At: now.Format(time.RFC3339Nano), By: actor})
	}
	src, _ := json.Marshal(source)
	dst, _ := json.Marshal(target)
	alert, _ := json.Marshal(moved)
	audit, _ := json.Marshal(map[string]any{"schema_version": 1, "audit_id": auditID, "actor": actor, "at": now, "alert_id": moved.ID, "source_incident_id": source.ID, "target_incident_id": target.ID, "source_composition": incidentComposition(&source), "target_composition": incidentComposition(&target)})
	n, err := regroupScript.Run(ctx, store.disk.rdb, []string{keyIncident + source.ID, keyIncident + target.ID, keyAlert + moved.ID, keyFingerprint + moved.Fingerprint, keyIncidentIdx, "igpt:regroup:" + auditID}, sourceRaw, targetRaw, alertRaw, src, dst, alert, target.ID, target.CreatedAt.Unix(), audit, source.ID, moved.Fingerprint).Int()
	if err != nil {
		http.Error(w, "regroup persistence unavailable", 503)
		return
	}
	if n != 1 {
		http.Error(w, "composition changed; retry", 409)
		return
	}
	writeJSON(w, 200, map[string]any{"target_incident_id": target.ID, "audit_id": auditID, "analysis_status": "previous composition; request refresh for new analysis"})
}
func appendUnique(ids []string, id string) []string {
	for _, v := range ids {
		if v == id {
			return ids
		}
	}
	if len(ids) < 50 {
		ids = append(ids, id)
	}
	return ids
}
