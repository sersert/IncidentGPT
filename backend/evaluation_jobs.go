package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
)

var enqueueEvaluationScript = redis.NewScript(`
local prior=redis.call('GET',KEYS[1])
if prior then if cjson.decode(prior).request_hash~=ARGV[2] then return -1 end;return 0 end
if not redis.call('EXISTS',KEYS[5]) then return -4 end
redis.call('ZREMRANGEBYSCORE',KEYS[3],'-inf',ARGV[6])
if redis.call('ZCARD',KEYS[3])>=1000 or redis.call('ZCARD',KEYS[2])+tonumber(ARGV[7])>32 then return -2 end
if not redis.call('SET',KEYS[4],'1','NX','EX',60) then return -3 end
redis.call('SET',KEYS[1],ARGV[1],'EX',ARGV[5]);redis.call('ZADD',KEYS[3],ARGV[3],ARGV[4])
redis.call('ZADD',KEYS[6],ARGV[3],ARGV[4]);redis.call('EXPIRE',KEYS[6],ARGV[5])
for i=7,#KEYS do redis.call('SET',KEYS[i],ARGV[i+1],'EX',ARGV[5]);redis.call('ZADD',KEYS[2],ARGV[3],KEYS[i]) end
return 1`)

func evaluationPaused(ctx context.Context) bool {
	if !operationsEnabled() {
		return false
	}
	state, err := readAIControl(ctx)
	return err != nil || !state.Enabled
}
func handleEvaluationRuns(w http.ResponseWriter, r *http.Request) {
	if !evaluationAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	caseID := r.PathValue("case")
	c, err := loadEvaluationCase(ctx, caseID)
	if err == redis.Nil {
		http.Error(w, "test case not found or expired", 404)
		return
	}
	if err != nil {
		http.Error(w, "case unavailable", 503)
		return
	}
	if r.Method == "GET" {
		ids, err := store.disk.rdb.ZRevRangeByScore(ctx, "igpt:evaluation:case-runs:"+caseID, &redis.ZRangeBy{Min: fmt.Sprint(time.Now().Add(-evaluationTTL).UnixMilli()), Max: "+inf", Count: 25}).Result()
		if err != nil {
			http.Error(w, "runs unavailable", 503)
			return
		}
		items := []map[string]any{}
		for _, id := range ids {
			raw, err := store.disk.rdb.Get(ctx, "igpt:evaluation:run:"+id).Bytes()
			if err == redis.Nil {
				continue
			}
			var run EvaluationRun
			if err != nil || json.Unmarshal(raw, &run) != nil || run.Case.ID != caseID {
				http.Error(w, "run unavailable", 503)
				return
			}
			items = append(items, map[string]any{"run_id": run.ID, "created_at": run.CreatedAt, "created_by": run.Actor, "profiles": run.Profiles, "snapshot_hash": run.Case.SnapshotHash})
		}
		writeJSON(w, 200, map[string]any{"items": items})
		return
	}
	var request struct {
		ProfileIDs []string `json:"profile_ids"`
		RequestID  string   `json:"request_id"`
		Confirm    bool     `json:"confirm_provider_calls"`
	}
	if decodeChange(r, &request) != nil || !request.Confirm || len(request.ProfileIDs) < 1 || len(request.ProfileIDs) > 4 || len(request.RequestID) < 16 || len(request.RequestID) > 100 {
		http.Error(w, "select 1–4 profiles and explicitly confirm provider calls with a request id", 400)
		return
	}
	profiles, err := configuredEvaluationProfiles(ctx)
	if err != nil {
		http.Error(w, "evaluation profiles unavailable", 503)
		return
	}
	selected := []EvaluationProfile{}
	seen := map[string]bool{}
	for _, id := range request.ProfileIDs {
		if seen[id] {
			http.Error(w, "duplicate profile", 400)
			return
		}
		seen[id] = true
		found := false
		for _, p := range profiles {
			if p.ID == id {
				selected = append(selected, p)
				found = true
			}
		}
		if !found {
			http.Error(w, "profile not allowed", 400)
			return
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	actor, _, _ := r.BasicAuth()
	if len(actor) > 200 {
		http.Error(w, "actor exceeds budget", 400)
		return
	}
	run := EvaluationRun{ID: "eval-" + digest(actor, request.RequestID), Case: c, Profiles: selected, CreatedAt: time.Now().UTC(), Actor: actor}
	config, _ := json.Marshal(selected)
	run.RequestHash = digest(caseID, c.SnapshotHash, string(config))
	raw, _ := json.Marshal(run)
	keys := []string{"igpt:evaluation:run:" + run.ID, evaluationDue, evaluationRuns, "igpt:evaluation:rate-limit", "igpt:evaluation:case:" + caseID, "igpt:evaluation:case-runs:" + caseID}
	args := []any{raw, run.RequestHash, run.CreatedAt.UnixMilli(), run.ID, int(evaluationTTL.Seconds()), time.Now().Add(-evaluationTTL).UnixMilli(), len(selected)}
	for _, p := range selected {
		jid := run.ID + ":" + p.ID
		input := c.Input
		input.AnalysisID = jid
		job := EvaluationJob{ID: jid, RunID: run.ID, Profile: p, Input: input, Status: "queued", CreatedAt: run.CreatedAt}
		raw, _ := json.Marshal(job)
		keys = append(keys, "igpt:evaluation:job:"+jid)
		args = append(args, raw)
	}
	n, err := enqueueEvaluationScript.Run(ctx, store.disk.rdb, keys, args...).Int()
	if err != nil {
		http.Error(w, "run persistence unavailable", 503)
		return
	}
	switch n {
	case -1:
		http.Error(w, "request id was already used for a different comparison", 409)
		return
	case -2:
		http.Error(w, "evaluation queue or retention limit reached", 429)
		return
	case -3:
		http.Error(w, "comparisons are limited to one batch per minute", 429)
		return
	case -4:
		http.Error(w, "test case expired", 409)
		return
	}
	writeJSON(w, 202, map[string]any{"run_id": run.ID, "duplicate": n == 0, "status": "queued"})
}
func loadEvaluationRun(ctx context.Context, id string) (EvaluationRun, error) {
	var run EvaluationRun
	raw, err := store.disk.rdb.Get(ctx, "igpt:evaluation:run:"+id).Bytes()
	if err != nil {
		return run, err
	}
	if json.Unmarshal(raw, &run) != nil || run.ID != id || len(run.Profiles) < 1 || len(run.Profiles) > 4 || run.Case.SnapshotHash != evaluationSnapshotHash(run.Case.Input) {
		return run, fmt.Errorf("invalid evaluation run")
	}
	return run, nil
}
func handleEvaluationRun(w http.ResponseWriter, r *http.Request) {
	if !evaluationAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	run, err := loadEvaluationRun(ctx, r.PathValue("run"))
	if err == redis.Nil {
		http.Error(w, "run not found or expired", 404)
		return
	}
	if err != nil {
		http.Error(w, "run unavailable", 503)
		return
	}
	items := []EvaluationJob{}
	for _, profile := range run.Profiles {
		var job EvaluationJob
		raw, err := store.disk.rdb.Get(ctx, "igpt:evaluation:job:"+run.ID+":"+profile.ID).Bytes()
		if err != nil || json.Unmarshal(raw, &job) != nil || job.RunID != run.ID || job.Profile != profile {
			http.Error(w, "evaluation result unavailable", 503)
			return
		}
		job.Input = AnalysisInput{}
		job.Owner = ""
		job.Lease = 0
		items = append(items, job)
	}
	writeJSON(w, 200, map[string]any{"run": run, "results": items, "paused": evaluationPaused(ctx)})
}

// Evaluation has its own queue and a global lease. A lost in-flight attempt is
// recorded as interrupted, never automatically sent to a paid provider again.
var claimEvaluationScript = redis.NewScript(`
if redis.call('EXISTS',KEYS[2])==1 then return nil end
local keys=redis.call('ZRANGEBYSCORE',KEYS[1],'-inf',ARGV[1],'LIMIT',0,1);if #keys==0 then return nil end
local key=keys[1];local raw=redis.call('GET',key);if not raw then redis.call('ZREM',KEYS[1],key);return nil end
local job=cjson.decode(raw)
if job.status=='running' then job.status='failed';job.error_code='interrupted_no_automatic_retry';job.finished_at=ARGV[4];redis.call('SET',key,cjson.encode(job),'KEEPTTL');redis.call('ZREM',KEYS[1],key);return nil end
if job.status~='queued' then redis.call('ZREM',KEYS[1],key);return nil end
job.status='running';job.owner=ARGV[2];job.lease_until=tonumber(ARGV[3]);job.started_at=ARGV[4]
redis.call('SET',KEYS[2],ARGV[2],'PX',90000);raw=cjson.encode(job);redis.call('SET',key,raw,'KEEPTTL');redis.call('ZADD',KEYS[1],ARGV[3],key);return raw`)
var finishEvaluationScript = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw then return 0 end
local job=cjson.decode(raw)
if job.owner~=ARGV[1] or job.status~='running' or job.lease_until<tonumber(ARGV[2]) or redis.call('GET',KEYS[3])~=ARGV[1] then return 0 end
redis.call('SET',KEYS[1],ARGV[3],'KEEPTTL');redis.call('ZREM',KEYS[2],KEYS[1]);redis.call('DEL',KEYS[3]);return 1`)

func claimEvaluation(ctx context.Context) (*EvaluationJob, error) {
	if evaluationPaused(ctx) {
		return nil, nil
	}
	now := time.Now().UTC()
	raw, err := claimEvaluationScript.Run(ctx, store.disk.rdb, []string{evaluationDue, "igpt:evaluation:active"}, now.UnixMilli(), newStableID("eval-owner"), now.Add(90*time.Second).UnixMilli(), now.Format(time.RFC3339Nano)).Text()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var job EvaluationJob
	if json.Unmarshal([]byte(raw), &job) != nil {
		return nil, fmt.Errorf("invalid persisted evaluation job")
	}
	return &job, nil
}
func executeEvaluation(ctx context.Context, job *EvaluationJob) (*AnalysisVersion, string) {
	if validateEvaluationProfile(job.Profile) != nil || validateEvaluationInput(job.Input) != nil {
		return nil, "invalid_evaluation_snapshot"
	}
	raw, status, err := evaluationRequest(ctx, "POST", "analyze", EvaluationRequest{Profile: job.Profile, Input: job.Input})
	if err != nil {
		return nil, "worker_unavailable"
	}
	if status != 200 {
		return nil, fmt.Sprintf("worker_http_%d", status)
	}
	var a AnalysisVersion
	if json.Unmarshal(raw, &a) != nil || validateAnalysis(a, job.Input) != nil || a.Model != job.Profile.Model || a.PromptVersion != job.Profile.PromptVersion || a.Knowledge != nil || a.Notification != "disabled" {
		return nil, "invalid_evaluation_result"
	}
	a.Usage = evaluationUsage(a.Usage)
	a.HumanConfirmation = "unconfirmed"
	a.Composition = job.Input.Composition
	return &a, ""
}
func finishEvaluation(ctx context.Context, job *EvaluationJob, result *AnalysisVersion, code string) error {
	finished := *job
	finished.Status = "complete"
	if result == nil {
		finished.Status = "failed"
	}
	finished.Result = result
	finished.ErrorCode = code
	now := time.Now().UTC()
	finished.FinishedAt = &now
	finished.Owner = ""
	finished.Lease = 0
	raw, _ := json.Marshal(finished)
	n, err := finishEvaluationScript.Run(ctx, store.disk.rdb, []string{"igpt:evaluation:job:" + job.ID, evaluationDue, "igpt:evaluation:active"}, job.Owner, now.UnixMilli(), raw).Int()
	if err != nil || n != 1 {
		return fmt.Errorf("evaluation lease lost")
	}
	return nil
}
func startEvaluationJobs() {
	if !evaluationEnabled() || store.disk == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
			job, err := claimEvaluation(ctx)
			if err == nil && job != nil {
				result, code := executeEvaluation(ctx, job)
				finishCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
				_ = finishEvaluation(finishCtx, job, result, code)
				stop()
			}
			cancel()
		}
	}()
}

var reviewEvaluationScript = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw then return -1 end
local job=cjson.decode(raw);if job.status~='complete' or job.review.revision~=tonumber(ARGV[1]) then return 0 end
redis.call('SET',KEYS[1],ARGV[2],'KEEPTTL');redis.call('HSET',KEYS[2],ARGV[3],ARGV[4]);local ttl=redis.call('TTL',KEYS[1]);if ttl>0 then redis.call('EXPIRE',KEYS[2],ttl) end;return 1`)

func handleEvaluationReview(w http.ResponseWriter, r *http.Request) {
	if !evaluationAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	run, err := loadEvaluationRun(ctx, r.PathValue("run"))
	if err != nil {
		http.Error(w, "run unavailable", 404)
		return
	}
	profile := r.PathValue("profile")
	found := false
	for _, p := range run.Profiles {
		found = found || p.ID == profile
	}
	if !found {
		http.Error(w, "result not found", 404)
		return
	}
	var request struct {
		Revision int    `json:"revision"`
		Verdict  string `json:"verdict"`
		Note     string `json:"note"`
	}
	if decodeChange(r, &request) != nil || request.Revision < 0 || len(request.Note) > 2000 || (request.Verdict != "accurate" && request.Verdict != "partial" && request.Verdict != "incorrect" && request.Verdict != "unrated") {
		http.Error(w, "invalid review", 400)
		return
	}
	key := "igpt:evaluation:job:" + run.ID + ":" + profile
	raw, err := store.disk.rdb.Get(ctx, key).Bytes()
	var job EvaluationJob
	if err != nil || json.Unmarshal(raw, &job) != nil {
		http.Error(w, "result unavailable", 503)
		return
	}
	actor, _, _ := r.BasicAuth()
	review := EvaluationReview{Revision: request.Revision + 1, Verdict: request.Verdict, Note: request.Note, Actor: actor, At: time.Now().UTC()}
	safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", review)
	var clean EvaluationReview
	if err != nil || json.Unmarshal(safe, &clean) != nil {
		http.Error(w, "review sanitization unavailable", 503)
		return
	}
	review.Note = clean.Note
	job.Review = review
	raw, _ = json.Marshal(job)
	n, err := reviewEvaluationScript.Run(ctx, store.disk.rdb, []string{key, "igpt:evaluation:review-history:" + job.ID}, request.Revision, raw, review.Revision, mustReviewJSON(review)).Int()
	if err != nil {
		http.Error(w, "review persistence unavailable", 503)
		return
	}
	if n != 1 {
		http.Error(w, "review changed or result is not complete; refresh", 409)
		return
	}
	writeJSON(w, 200, review)
}

func mustReviewJSON(review EvaluationReview) string {
	raw, _ := json.Marshal(review)
	return string(raw)
}
