package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const jobsDue = "igpt:jobs:due"

type jobEnvelope struct {
	Job   DurableJob    `json:"job"`
	Input AnalysisInput `json:"input"`
	Owner string        `json:"owner"`
	Lease int64         `json:"lease_until"`
}

func durableEnabled() bool            { return env("DURABLE_ANALYSIS_ENABLED", "false") == "true" }
func analysisKey(id string) string    { return "igpt:analysis:" + id }
func analysisLatest(id string) string { return "igpt:analysis:latest:" + id }
func makeEvidence(inc *Incident, alerts []EnrichedAlert) []Evidence {
	out := make([]Evidence, 0, len(alerts)*4)
	now := time.Now().UTC()
	for i, a := range alerts {
		resource := a.Labels["pod"]
		if resource == "" {
			resource = incidentWorkload(inc)
		}
		var source *time.Time

		if t, err := time.Parse(time.RFC3339Nano, a.StartsAt); err == nil {
			t = t.UTC()
			source = &t
		}
		observations := []struct {
			kind      string
			data      any
			available bool
		}{{"alert", map[string]any{"labels": a.Labels, "annotations": a.Annotations, "status": a.Status}, true}, {"kubernetes", a.K8sContext, len(a.K8sContext) > 0}, {"metrics", a.PromSample, len(a.PromSample) > 0}, {"logs", a.RecentLogs, len(a.RecentLogs) > 0}}
		for _, o := range observations {
			raw, _ := json.Marshal(o.data)
			excerpt := string(raw)
			availability := "available"
			if !o.available {
				availability = "unavailable"
				excerpt = "Source availability was not recorded by the legacy collector; no successful empty query is implied."
			}
			if len(excerpt) > 2048 {
				excerpt = boundedExcerpt(excerpt, 2048)
			}
			from := source
			if o.kind != "alert" {
				from = nil
			}
			out = append(out, Evidence{Version: 1, ID: "evidence-" + digest(inc.ID, strconv.Itoa(i), o.kind, a.EnrichedAt, a.StartsAt, string(raw)), IncidentID: inc.ID, ClusterID: incidentCluster(inc), Source: o.kind, Resource: resource, ObservedFrom: from, CollectedAt: nil, IngestedAt: now, Excerpt: excerpt, CollectorVersion: "enricher-provided/v1", Availability: availability})
		}
	}
	return out
}

var enqueueAnalysisScript = redis.NewScript(`
if redis.call('EXISTS',KEYS[#KEYS]) == 1 then return -2 end
if redis.call('EXISTS',KEYS[1]) == 1 then return 0 end
local inc=redis.call('GET',KEYS[3]); if not inc then return -1 end
local candidate=cjson.decode(ARGV[1])
if candidate.job.kind=='diagnostics' then
local activeKey='igpt:diagnostics:active:'..candidate.job.incident_id
local active=redis.call('GET',activeKey)
if active then local oldraw=redis.call('GET','igpt:job:'..active);if oldraw then local old=cjson.decode(oldraw);if old.job.status=='queued' or old.job.status=='running' then return -3 end end end
redis.call('SET',activeKey,candidate.job.job_id)
end
local data=cjson.decode(inc); data.analysis_request_id=ARGV[3]
redis.call('SET',KEYS[3],cjson.encode(data))
redis.call('SET',KEYS[1],ARGV[1]); redis.call('ZADD',KEYS[2],ARGV[2],ARGV[4]);redis.call('ZADD','igpt:jobs:pending:created',ARGV[2],ARGV[4])
redis.call('SET',KEYS[4],ARGV[3]); redis.call('ZADD',KEYS[5],ARGV[2],ARGV[4])
for i=6,#KEYS-1 do redis.call('SET',KEYS[i],ARGV[i-1],'NX') end
redis.call('SET',KEYS[#KEYS],ARGV[4])
return 1`)

func enqueueAnalysis(ctx context.Context, inc *Incident, alerts []EnrichedAlert, batchID string) (string, error) {
	if store.disk == nil {
		return "", fmt.Errorf("persistent storage required")
	}
	aid := "analysis-" + digest(inc.ID, batchID)
	jid := "job-" + digest(aid)
	input := AnalysisInput{Version: 1, IncidentID: inc.ID, AnalysisID: aid, ClusterID: incidentCluster(inc), Evidence: makeEvidence(inc, alerts), Composition: incidentComposition(inc)}
	input.IngestKey = "igpt:ingest:batch:" + digest(incidentCluster(inc), batchID)
	input.PreviousID, _ = store.disk.rdb.Get(ctx, analysisLatest(inc.ID)).Result()
	return enqueueInput(ctx, inc, input, jid)
}
func incidentComposition(inc *Incident) string {
	ids := make([]string, 0, len(inc.Alerts))
	for _, a := range inc.Alerts {
		ids = append(ids, a.ID)
	}
	return digest(ids...)
}
func enqueueInput(ctx context.Context, inc *Incident, input AnalysisInput, jid string) (string, error) {
	now := time.Now().UTC()
	job := DurableJob{Version: 1, ID: jid, IncidentID: inc.ID, AnalysisID: input.AnalysisID, Kind: "analysis", Status: "queued", CreatedAt: now}
	if input.JobKind != "" {
		job.Kind = input.JobKind
	}
	envelope := jobEnvelope{Job: job, Input: input}
	raw, _ := json.Marshal(envelope)
	keys := []string{"igpt:job:" + jid, jobsDue, keyIncident + inc.ID, "igpt:analysis:requested:" + inc.ID, "igpt:jobs:incident:" + inc.ID}
	args := []any{raw, now.UnixMilli(), input.AnalysisID, jid}
	for _, e := range input.Evidence {
		keys = append(keys, "igpt:evidence:"+e.ID)
		data, _ := json.Marshal(e)
		args = append(args, data)
	}
	if input.IngestKey == "" {
		input.IngestKey = "igpt:ingest:batch:" + digest(jid)
	}
	keys = append(keys, input.IngestKey)
	n, err := enqueueAnalysisScript.Run(ctx, store.disk.rdb, keys, args...).Int()
	if n == -3 && err == nil {
		return "", fmt.Errorf("investigation already active")
	}
	if n == -2 && err == nil {
		return store.disk.rdb.Get(ctx, input.IngestKey).Result()
	}
	if err != nil || n < 0 {
		return "", fmt.Errorf("analysis scheduling unavailable")
	}
	return jid, nil
}

var claimJobScript = redis.NewScript(`
local ids=redis.call('ZRANGEBYSCORE',KEYS[1],'-inf',ARGV[1],'LIMIT',0,1)
if #ids == 0 then return nil end
local key='igpt:job:'..ids[1];local raw=redis.call('GET',key)
if not raw then redis.call('ZREM',KEYS[1],ids[1]);return nil end
local e=cjson.decode(raw)
if e.job.status == 'canceled' or e.job.status == 'complete' or e.job.status == 'failed' then redis.call('ZREM',KEYS[1],ids[1]);return nil end
if e.job.attempts >= 4 then if e.job.kind=='diagnostics' and redis.call('GET','igpt:diagnostics:active:'..e.job.incident_id)==e.job.job_id then redis.call('DEL','igpt:diagnostics:active:'..e.job.incident_id) end;e.job.status='failed';e.job.error_code='retry_budget_exhausted';e.job.finished_at=ARGV[4];redis.call('SET',key,cjson.encode(e));redis.call('ZREM',KEYS[1],ids[1]);redis.call('ZREM','igpt:jobs:pending:created',ids[1]);redis.call('INCR','igpt:jobs:failures');return nil end
e.owner=ARGV[2];e.lease_until=tonumber(ARGV[3]);e.job.status='running';e.job.attempts=e.job.attempts+1; if e.job.attempts>1 then redis.call('INCR','igpt:jobs:retries') end
raw=cjson.encode(e);redis.call('SET',key,raw);redis.call('ZADD',KEYS[1],ARGV[3],ids[1]);return raw`)

func claimJob(ctx context.Context) (*jobEnvelope, error) {
	now := time.Now().UTC()
	raw, err := claimJobScript.Run(ctx, store.disk.rdb, []string{jobsDue}, now.UnixMilli(), newStableID("owner"), now.Add(90*time.Second).UnixMilli(), now.Format(time.RFC3339Nano)).Text()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var e jobEnvelope
	err = json.Unmarshal([]byte(raw), &e)
	return &e, err
}

var finishJobScript = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw then return 0 end
local e=cjson.decode(raw);if e.owner ~= ARGV[1] or e.job.status ~= 'running' or e.lease_until < tonumber(ARGV[2]) then return 0 end
e.job.status=ARGV[3];e.job.error_code=ARGV[4]; if ARGV[3]=='failed' then redis.call('INCR','igpt:jobs:failures') end
if ARGV[3] == 'queued' then redis.call('ZADD',KEYS[2],ARGV[5],e.job.job_id)
else if e.job.kind=='diagnostics' and redis.call('GET','igpt:diagnostics:active:'..e.job.incident_id)==e.job.job_id then redis.call('DEL','igpt:diagnostics:active:'..e.job.incident_id) end;e.job.finished_at=ARGV[6];redis.call('ZREM',KEYS[2],e.job.job_id);redis.call('ZREM','igpt:jobs:pending:created',e.job.job_id) end
redis.call('SET',KEYS[1],cjson.encode(e))
if ARGV[3] == 'complete' then
redis.call('INCR','igpt:jobs:completed');if tonumber(ARGV[8])>=0 then redis.call('INCRBYFLOAT','igpt:jobs:latency:sum',ARGV[8]);redis.call('INCR','igpt:jobs:latency:samples') end; redis.call('SET',KEYS[3],ARGV[7],'NX');redis.call('ZADD',KEYS[4],ARGV[2],e.job.analysis_id)
if redis.call('GET',KEYS[5]) == e.job.analysis_id then
redis.call('SET',KEYS[6],e.job.analysis_id)
local rawinc=redis.call('GET','igpt:incident:'..e.job.incident_id)
if rawinc then local inc=cjson.decode(rawinc);local a=cjson.decode(ARGV[7]);inc.analysis_summary=a.summary;redis.call('SET','igpt:incident:'..e.job.incident_id,cjson.encode(inc)) end
end
end
return 1`)

func finishJob(ctx context.Context, e *jobEnvelope, a *AnalysisVersion, code string) error {
	status := "complete"
	data := []byte("{}")
	if a != nil {
		data, _ = json.Marshal(a)
	} else if e.Job.Attempts >= 4 {
		status = "failed"
	} else {
		status = "queued"
	}
	latency := -1.0
	if a != nil && !a.StartedAt.IsZero() && !a.CompletedAt.Before(a.StartedAt) {
		latency = a.CompletedAt.Sub(a.StartedAt).Seconds()
	}
	now := time.Now().UTC()
	n, err := finishJobScript.Run(ctx, store.disk.rdb, []string{"igpt:job:" + e.Job.ID, jobsDue, analysisKey(e.Job.AnalysisID), "igpt:analysis:history:" + e.Job.IncidentID, "igpt:analysis:requested:" + e.Job.IncidentID, analysisLatest(e.Job.IncidentID)}, e.Owner, now.UnixMilli(), status, code, now.Add(time.Duration(1<<e.Job.Attempts)*time.Second).UnixMilli(), now.Format(time.RFC3339Nano), data, latency).Int()
	if err != nil || n != 1 {
		return fmt.Errorf("lease lost or completion unavailable")
	}
	return nil
}

var durableHTTP = &http.Client{Timeout: 55 * time.Second}

func executeAnalysis(ctx context.Context, e *jobEnvelope) (*AnalysisVersion, error) {
	raw, _ := json.Marshal(e.Input)
	req, err := http.NewRequestWithContext(ctx, "POST", env("STRUCTURED_WORKER_URL", "http://ai-worker.incidentgpt.svc:8080/v1/analyze"), bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid_worker_configuration")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env("ANALYSIS_SERVICE_TOKEN", ""))
	resp, err := durableHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider_or_worker_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("worker_http_%d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return nil, fmt.Errorf("worker_read_failed")
	}
	var a AnalysisVersion
	if json.Unmarshal(body, &a) != nil {
		return nil, fmt.Errorf("invalid_structured_output")
	}
	if err = validateAnalysis(a, e.Input); err != nil {
		return nil, err
	}
	a.HumanConfirmation = "unconfirmed"
	a.PreviousID = e.Input.PreviousID
	a.Composition = e.Input.Composition
	return &a, nil
}
func startAnalysisJobs() {
	if !durableEnabled() || store.disk == nil {
		return
	}
	for i := 0; i < 4; i++ {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for range ticker.C {
				ctx, cancel := context.WithTimeout(context.Background(), 85*time.Second)
				e, err := claimJob(ctx)
				if err == nil && e != nil {
					var a *AnalysisVersion
					var runErr error
					if e.Job.Kind == "diagnostics" {
						runErr = collectDiagnosticEvidence(ctx, e)
					}
					if runErr == nil {
						a, runErr = executeAnalysis(ctx, e)
					}
					code := ""
					if runErr != nil {
						code = runErr.Error()
						a = nil
					}
					_ = finishJob(ctx, e, a, code)
				}
				cancel()
			}
		}()
	}
}
func handleAnalysisVersions(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	id := r.PathValue("id")
	inc := store.Get(id)
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}
	if store.disk == nil {
		http.Error(w, "history storage unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	limit := intQuery(r, "limit", 25)
	offset := intQuery(r, "offset", 0)
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		http.Error(w, "invalid pagination", 400)
		return
	}
	ids, err := store.disk.rdb.ZRevRange(ctx, "igpt:analysis:history:"+id, int64(offset), int64(offset+limit-1)).Result()
	if err != nil {
		http.Error(w, "history unavailable", 503)
		return
	}
	latest, err := store.disk.rdb.Get(ctx, analysisLatest(id)).Result()
	if err != nil && err != redis.Nil {
		http.Error(w, "latest unavailable", 503)
		return
	}
	versions := []AnalysisVersion{}
	for _, aid := range ids {
		raw, err := store.disk.rdb.Get(ctx, analysisKey(aid)).Bytes()
		if err != nil {
			http.Error(w, "analysis unavailable", 503)
			return
		}
		var a AnalysisVersion
		if json.Unmarshal(raw, &a) != nil || a.IncidentID != id {
			http.Error(w, "analysis integrity failure", 503)
			return
		}
		versions = append(versions, a)
	}
	jids, err := store.disk.rdb.ZRevRange(ctx, "igpt:jobs:incident:"+id, 0, 24).Result()
	if err != nil {
		http.Error(w, "job status unavailable", 503)
		return
	}
	jobs := []DurableJob{}
	operations := []DiagnosticOperation{}
	for _, jid := range jids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:job:"+jid).Bytes()
		if err != nil {
			http.Error(w, "job unavailable", 503)
			return
		}
		var e jobEnvelope
		if json.Unmarshal(raw, &e) != nil {
			http.Error(w, "invalid job", 503)
			return
		}
		jobs = append(jobs, e.Job)
		if e.Input.Operation != nil {
			operations = append(operations, *e.Input.Operation)
		}
	}
	writeJSON(w, 200, map[string]any{"items": versions, "latest_analysis_id": latest, "jobs": jobs, "operations": operations, "composition_hash": incidentComposition(inc), "history_sources": inc.HistorySources, "enabled": durableEnabled()})
}
func handleEvidence(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "evidence storage unavailable", 503)
		return
	}
	id, eid := r.PathValue("id"), r.PathValue("evidence")
	if store.Get(id) == nil {
		http.Error(w, "not found", 404)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	raw, err := store.disk.rdb.Get(ctx, "igpt:evidence:"+eid).Bytes()
	if err == redis.Nil {
		http.Error(w, "not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "evidence unavailable", 503)
		return
	}
	var e Evidence
	if json.Unmarshal(raw, &e) != nil || e.IncidentID != id {
		http.Error(w, "evidence does not belong to incident", 403)
		return
	}
	writeJSON(w, 200, e)
}
func handleDurableIngest(w http.ResponseWriter, r *http.Request) {
	if store.disk == nil {
		http.Error(w, "durable scheduling requires Redis", 503)
		return
	}
	token := env("ANALYSIS_SERVICE_TOKEN", "")
	if !trustedBearer(r, token) {
		http.Error(w, "unauthorized", 401)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 512<<10))
	defer r.Body.Close()
	if err != nil {
		http.Error(w, "bad payload", 400)
		return
	}
	alerts, gk, err := parseIngestPayload(body)
	if err != nil || len(alerts) == 0 || len(alerts) > 50 {
		http.Error(w, "invalid bounded alert batch", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", alerts)
	if err != nil || json.Unmarshal(safe, &alerts) != nil {
		http.Error(w, "sanitization failed; not ingested", 503)
		return
	}
	for _, a := range alerts {
		if a.ClusterName != alerts[0].ClusterName {
			http.Error(w, "mixed clusters", 400)
			return
		}
	}
	batchID := r.Header.Get("X-IncidentGPT-Batch-ID")
	if len(batchID) > 200 {
		http.Error(w, "invalid batch id", 400)
		return
	}
	if batchID == "" {
		batchID = digest(string(body))
	}
	cluster := alerts[0].ClusterName
	if cluster == "" || cluster == "unknown" {
		cluster = alerts[0].Labels["cluster"]
	}
	lockKey := "igpt:composition:lock"
	owner := newStableID("ingest-owner")
	claimed, err := store.disk.rdb.SetNX(ctx, lockKey, owner, 30*time.Second).Result()
	if err != nil || !claimed {
		http.Error(w, "ingest busy; retry same batch", 503)
		return
	}
	defer func() {
		_ = redis.NewScript("if redis.call('GET',KEYS[1]) == ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0").Run(context.Background(), store.disk.rdb, []string{lockKey}, owner).Err()
	}()
	prior, err := store.disk.rdb.Get(ctx, "igpt:ingest:batch:"+digest(cluster, batchID)).Result()
	if err == nil {
		writeJSON(w, 202, map[string]any{"job_id": prior, "scheduled": true, "duplicate": true})
		return
	}
	if err != redis.Nil {
		http.Error(w, "batch lookup unavailable", 503)
		return
	}
	// Keep a human-moved fingerprint apart from its former automatic batch.
	partitions := map[string][]EnrichedAlert{}
	for _, alert := range alerts {
		fp := alert.Fingerprint
		if cluster != "" && cluster != "unknown" {
			fp = "cluster-" + digest(cluster, fp)
		}
		target, lookupErr := store.disk.rdb.Get(ctx, "igpt:regroup:binding:"+fp).Result()
		if lookupErr != nil && lookupErr != redis.Nil {
			http.Error(w, "manual binding unavailable", 503)
			return
		}
		partitions[target] = append(partitions[target], alert)
	}
	targets := make([]string, 0, len(partitions))
	for target := range partitions {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	jobIDs := []string{}
	incidentIDs := []string{}
	for _, target := range targets {
		part := partitions[target]
		fps := []string{}
		for _, alert := range part {
			fp := alert.Fingerprint
			if cluster != "" && cluster != "unknown" {
				fp = "cluster-" + digest(cluster, fp)
			}
			fps = append(fps, fp)
		}
		if existing := store.FindIncidentForGroup(fps); existing != nil {
			count := len(existing.Alerts)
			known := map[string]bool{}
			for _, a := range existing.Alerts {
				known[a.Fingerprint] = true
			}
			for i, a := range part {
				if a.Status != "resolved" && !known[fps[i]] {
					count++
					known[fps[i]] = true
				}
			}
			if count > 50 {
				http.Error(w, "incident composition limit reached; manually separate or close the group", 503)
				return
			}
		}
		inc := ingestGroup(part, gk)
		if inc == nil || inc.Status == "resolved" {
			continue
		}
		childBatch := batchID
		if len(targets) > 1 {
			childBatch = batchID + "/composition/" + target
		}
		jid, err := enqueueAnalysis(ctx, inc, part, childBatch)
		if err != nil {
			http.Error(w, "durable persistence or scheduling unavailable; retry same batch", 503)
			return
		}
		jobIDs = append(jobIDs, jid)
		incidentIDs = append(incidentIDs, inc.ID)
	}
	if len(jobIDs) == 0 {
		writeJSON(w, 200, map[string]any{"status": "resolved", "scheduled": false})
		return
	}
	if len(targets) > 1 {
		if err := store.disk.rdb.Set(ctx, "igpt:ingest:batch:"+digest(cluster, batchID), jobIDs[0], 0).Err(); err != nil {
			http.Error(w, "batch acknowledgement unavailable; retry", 503)
			return
		}
	}
	writeJSON(w, 202, map[string]any{"incident_id": incidentIDs[0], "incident_ids": incidentIDs, "job_id": jobIDs[0], "job_ids": jobIDs, "scheduled": true})
}
func trustedBearer(r *http.Request, token string) bool {
	return token != "" && constantToken(r.Header.Get("Authorization"), "Bearer "+token)
}
func handleJobMetrics(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "job storage unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	n, err := store.disk.rdb.ZCard(ctx, jobsDue).Result()
	if err != nil {
		http.Error(w, "queue unavailable", 503)
		return
	}
	oldest, err := store.disk.rdb.ZRangeWithScores(ctx, jobsDue, 0, 0).Result()
	if err != nil {
		http.Error(w, "queue unavailable", 503)
		return
	}
	age := 0.0
	if len(oldest) > 0 {
		age = max(0, float64(time.Now().UnixMilli())-oldest[0].Score) / 1000
	}
	counts, err := store.disk.rdb.MGet(ctx, "igpt:jobs:retries", "igpt:jobs:failures", "igpt:jobs:completed", "igpt:jobs:latency:sum", "igpt:jobs:latency:samples").Result()
	if err != nil {
		http.Error(w, "metrics unavailable", 503)
		return
	}
	created, err := store.disk.rdb.ZRangeWithScores(ctx, "igpt:jobs:pending:created", 0, 0).Result()
	if err != nil {
		http.Error(w, "queue age unavailable", 503)
		return
	}
	oldestAge := 0.0
	if len(created) > 0 {
		oldestAge = max(0, float64(time.Now().UnixMilli())-created[0].Score) / 1000
	}
	writeJSON(w, 200, map[string]any{"oldest_pending_age_seconds": oldestAge, "analysis_latency_seconds_sum": counts[3], "analysis_latency_samples": counts[4], "retries": counts[0], "failures": counts[1], "completed": counts[2], "pending_jobs": n, "overdue_seconds": age, "provider_usage": "unknown unless present on analysis"})
}

// Legacy callbacks never route durable analyses by reusable group key.
func rejectDurableCallback(w http.ResponseWriter, r *http.Request) bool {
	if !durableEnabled() {
		return false
	}
	http.Error(w, "durable results require leased incident_id+analysis_id job completion", 409)
	return true
}

func boundedExcerpt(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	runes := []rune(s)
	n := 0
	size := 0
	for n < len(runes) && size+len(string(runes[n])) <= limit-16 {
		size += len(string(runes[n]))
		n++
	}
	return string(runes[:n]) + " [truncated]"
}
