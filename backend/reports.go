package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"
)

const reportRetention = 90 * 24 * time.Hour

type ReportAction struct {
	Text    string `json:"text"`
	Owner   string `json:"owner"`
	Status  string `json:"status"`
	DueDate string `json:"due_date,omitempty"`
}
type ReportVersion struct {
	Version      int            `json:"schema_version"`
	ID           string         `json:"report_id"`
	IncidentID   string         `json:"incident_id"`
	Kind         string         `json:"kind"`
	Revision     int            `json:"revision"`
	Markdown     string         `json:"markdown"`
	GeneratedAt  time.Time      `json:"generated_at"`
	ReviewStatus string         `json:"review_status"`
	Status       string         `json:"status"`
	LastEditedBy string         `json:"last_edited_by"`
	LastEditedAt *time.Time     `json:"last_edited_at,omitempty"`
	Source       ReportSource   `json:"source"`
	Actions      []ReportAction `json:"action_items,omitempty"`
}

func reportIndex(id, kind string) string { return "igpt:reports:" + id + ":" + kind }
func markdownFact(s string) string {
	s = boundedExcerpt(s, 2000)
	r := strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "#", "\\#", "\n", " ", "\r", " ")
	return r.Replace(s)
}
func knownTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "Unknown"
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func buildReportSource(ctx context.Context, inc *Incident, kind, actor string) (*ReportSource, error) {
	if len(inc.Alerts) > 50 {
		return nil, fmt.Errorf("incident exceeds report snapshot budget")
	}
	source := &ReportSource{Version: 1, ReportID: newStableID("report"), Kind: kind, IncidentID: inc.ID, Composition: incidentComposition(inc), CapturedAt: time.Now().UTC(), Actor: actor}
	l, err := readLifecycle(ctx, inc)
	if err != nil {
		return nil, err
	}
	source.LifecycleRevision = l.Revision
	var b strings.Builder
	fmt.Fprintf(&b, "# %s draft: %s\n\nIncident: %s. Composition: %s. Snapshot: %s.\n\nHuman review required. Unknown fields are not inferred.\n\n## Impact and ownership\n\nCustomer impact, customer counts and financial impact: Unknown.\nStatus: %s. Owner: %s.\n\n## Detection and timeline\n\n", kind, markdownFact(inc.Title), inc.ID, source.Composition, source.CapturedAt.Format(time.RFC3339Nano), markdownFact(inc.Status), func() string {
		if inc.Assignee == nil {
			return "Unknown"
		}
		return markdownFact(*inc.Assignee)
	}())
	for _, event := range []struct {
		Name string
		At   *time.Time
	}{{"First known symptom", l.FirstSymptom}, {"Alert receipt", l.AlertReceipt}, {"Acknowledgement", l.Acknowledged}, {"Verified recovery", l.Recovery}, {"Administrative closure", l.Closed}} {
		fmt.Fprintf(&b, "- %s: %s (lifecycle revision %d).\n", event.Name, knownTime(event.At), l.Revision)
	}
	b.WriteString("\nAlert resolution alone is not verification of full service recovery.\n")
	latest, err := store.disk.rdb.Get(ctx, analysisLatest(inc.ID)).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	evidenceIDs := map[string]bool{}
	if latest != "" {
		raw, err := store.disk.rdb.Get(ctx, analysisKey(latest)).Bytes()
		var a AnalysisVersion
		if err != nil || json.Unmarshal(raw, &a) != nil || a.IncidentID != inc.ID {
			return nil, fmt.Errorf("analysis snapshot unavailable")
		}
		source.AnalysisID = a.ID
		fmt.Fprintf(&b, "\n## Current analysis and unresolved hypotheses\n\nAnalysis %s, processing %s to %s. Human confirmation: %s.\n\n%s\n", a.ID, knownTime(&a.StartedAt), knownTime(&a.CompletedAt), markdownFact(a.HumanConfirmation), markdownFact(a.Summary))
		if a.Composition != source.Composition {
			b.WriteString("\nWarning: analysis uses a previous incident composition.\n")
		}
		for _, h := range a.Hypotheses {
			fmt.Fprintf(&b, "\n- Unresolved hypothesis: %s. Supporting evidence: %s. Contradicting: %s.\n", markdownFact(h.Text), strings.Join(h.Supporting, ", "), strings.Join(h.Contradicting, ", "))
		}
		fmt.Fprintf(&b, "\nMissing data: %s. Next checks: %s.\n", markdownFact(strings.Join(a.Missing, "; ")), markdownFact(strings.Join(a.NextChecks, "; ")))
		for _, id := range a.EvidenceIDs {
			evidenceIDs[id] = true
		}
	} else {
		b.WriteString("\n## Current analysis and unresolved hypotheses\n\nAnalysis: Unknown or invalidated by composition change. No cause inferred.\n")
	}
	raw, err := store.disk.rdb.Get(ctx, "igpt:resolution:"+inc.ID).Bytes()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	b.WriteString("\n## Cause and actions\n\n")
	if err == nil {
		var resolution Resolution
		if json.Unmarshal(raw, &resolution) != nil {
			return nil, fmt.Errorf("resolution invalid")
		}
		source.ResolutionRevision = resolution.Revision
		if resolution.Status == "confirmed" {
			if resolution.Composition != source.Composition {
				b.WriteString("Warning: resolution confirmation belongs to a previous or unknown composition; applicability requires human review.\n")
			}
			fmt.Fprintf(&b, "Human-confirmed cause: %s. Restoring action: %s. Confirmed by %s at %s; resolution revision %d.\n", markdownFact(resolution.Cause), markdownFact(resolution.Action), markdownFact(resolution.Actor), resolution.At.Format(time.RFC3339Nano), resolution.Revision)
			for _, id := range resolution.EvidenceIDs {
				evidenceIDs[id] = true
			}
		} else {
			b.WriteString("Cause unresolved; no current confirmed resolution.\n")
		}
		for _, attempt := range resolution.Failed {
			fmt.Fprintf(&b, "- Human-recorded unsuccessful attempt: %s (resolution revision %d).\n", markdownFact(attempt), resolution.Revision)
		}
	} else {
		b.WriteString("Confirmed cause and restoring action: Unknown. Failed attempts: Unknown.\n")
	}
	for _, id := range l.RecoveryEvidence {
		evidenceIDs[id] = true
	}
	fmt.Fprintf(&b, "\nRecovery verification actor: %s; evidence: %s.\n", func() string {
		if l.RecoveryActor == "" {
			return "Unknown"
		}
		return markdownFact(l.RecoveryActor)
	}(), strings.Join(l.RecoveryEvidence, ", "))
	b.WriteString("\n## Relevant changes\n\nTemporal association is a lead, not causation.\n")
	if cluster := incidentCluster(inc); cluster != "" && inc.Namespace != "" && incidentWorkload(inc) != "" && l.FirstSymptom != nil {
		ids, err := store.disk.rdb.ZRevRangeByScore(ctx, changeIndex(cluster, inc.Namespace, incidentWorkload(inc)), &redis.ZRangeBy{Min: fmt.Sprint(l.FirstSymptom.Add(-24 * time.Hour).UnixMilli()), Max: fmt.Sprint(l.FirstSymptom.Add(time.Hour).UnixMilli()), Count: 25}).Result()
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			raw, err := store.disk.rdb.Get(ctx, "igpt:change:"+id).Bytes()
			var e ChangeEvent
			if err != nil || json.Unmarshal(raw, &e) != nil || e.ClusterID != cluster || e.Namespace != inc.Namespace || e.Workload != incidentWorkload(inc) {
				return nil, fmt.Errorf("change snapshot invalid")
			}
			source.ChangeIDs = append(source.ChangeIDs, id)
			fmt.Fprintf(&b, "- %s: %s / %s, revision %s, source time %s; source %s.\n", e.ID, markdownFact(e.Namespace), markdownFact(e.Workload), markdownFact(e.Revision), knownTime(e.SourceAt), markdownFact(e.Source))
		}
	}
	if len(source.ChangeIDs) == 0 {
		b.WriteString("Change coverage unknown or no stored associated changes in the bounded window.\n")
	}
	b.WriteString("\n## Investigation handoff\n\n")
	jobs, err := store.disk.rdb.ZRevRange(ctx, "igpt:jobs:incident:"+inc.ID, 0, 24).Result()
	if err != nil {
		return nil, err
	}
	for _, id := range jobs {
		raw, err := store.disk.rdb.Get(ctx, "igpt:job:"+id).Bytes()
		var job jobEnvelope
		if err != nil || json.Unmarshal(raw, &job) != nil {
			return nil, fmt.Errorf("job snapshot unavailable")
		}
		if job.Input.Operation != nil {
			op := job.Input.Operation
			fmt.Fprintf(&b, "- Check %s (%s): job %s; collection %s; results %s; evidence %s.\n", op.ID, markdownFact(op.Action), job.Job.Status, knownTime(op.CollectedAt), markdownFact(op.Status), strings.Join(op.EvidenceIDs, ", "))
			for _, id := range op.EvidenceIDs {
				evidenceIDs[id] = true
			}
		}
	}
	if len(jobs) == 0 {
		b.WriteString("No recorded investigation jobs.\n")
	}
	b.WriteString("\n## Stored evidence\n\n")
	ids := []string{}
	for id := range evidenceIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) > 80 {
		return nil, fmt.Errorf("evidence exceeds snapshot budget")
	}
	for _, id := range ids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:evidence:"+id).Bytes()
		var e Evidence
		if err != nil || json.Unmarshal(raw, &e) != nil || e.IncidentID != inc.ID {
			return nil, fmt.Errorf("authorized evidence snapshot unavailable")
		}
		source.EvidenceIDs = append(source.EvidenceIDs, id)
		fmt.Fprintf(&b, "- [%s](/api/v1/incidents/%s/evidence/%s): %s, %s, collected %s. %s\n", e.ID, inc.ID, e.ID, markdownFact(e.Source), markdownFact(e.Availability), knownTime(e.CollectedAt), markdownFact(boundedExcerpt(e.Excerpt, 256)))
	}
	if len(ids) == 0 {
		b.WriteString("No cited evidence selected. Historical source pages retain their original evidence.\n")
	}
	fmt.Fprintf(&b, "\n## Composition and audit\n\nHistorical source incidents: %s.\n", strings.Join(inc.HistorySources, ", "))
	b.WriteString("\n## Contributing factors and follow-up actions\n\nContributing factors: Unknown until reviewed.\nFollow-up actions: add reviewed action items with owner, status and optional due date.\n")
	source.Markdown = b.String()
	if kind == "handoff" {
		var brief strings.Builder
		fmt.Fprintf(&brief, "# Handoff draft: %s\n\nIncident: %s. Snapshot: %s. Human review required.\n\n", markdownFact(inc.Title), inc.ID, source.CapturedAt.Format(time.RFC3339Nano))
		sections := strings.Split(source.Markdown, "\n## ")
		for _, section := range sections[1:] {
			if strings.HasPrefix(section, "Impact and ownership") || strings.HasPrefix(section, "Current analysis") || strings.HasPrefix(section, "Cause and actions") || strings.HasPrefix(section, "Investigation handoff") {
				brief.WriteString("## ")
				brief.WriteString(section)
				brief.WriteString("\n")
			}
		}
		fmt.Fprintf(&brief, "\nCited evidence IDs: %s.\n\nNext owner: Unknown until assigned. Pending work: review missing data and next checks above.\n", strings.Join(source.EvidenceIDs, ", "))
		source.Markdown = boundedExcerpt(brief.String(), 8<<10)
	}
	if len(source.Markdown) > 32<<10 {
		return nil, fmt.Errorf("report exceeds 32 KiB budget")
	}
	return source, nil
}
func handleReports(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "report storage unavailable", 503)
		return
	}
	inc := store.Get(r.PathValue("id"))
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}
	kind := "postmortem"
	if strings.Contains(r.URL.Path, "/handoff") {
		kind = "handoff"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if r.Method == "POST" {
		if env("REPORTS_ENABLED", "false") != "true" || !durableEnabled() {
			http.Error(w, "report generation disabled", 503)
			return
		}
		actor, _, _ := r.BasicAuth()
		source, err := buildReportSource(ctx, inc, kind, actor)
		if err != nil {
			http.Error(w, "report sources unavailable or exceed snapshot budget", 503)
			return
		}
		safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", source)
		var clean ReportSource
		if err != nil || json.Unmarshal(safe, &clean) != nil || clean.IncidentID != inc.ID || clean.ReportID != source.ReportID || clean.Composition != source.Composition || clean.Kind != kind || !sameReportSourceIdentity(clean, *source) {
			http.Error(w, "report snapshot sanitization failed", 503)
			return
		}
		input := AnalysisInput{Version: 1, IncidentID: inc.ID, ClusterID: incidentCluster(inc), JobKind: "report", Report: &clean, Composition: source.Composition}
		jid, err := enqueueInput(ctx, inc, input, newStableID("job"))
		if err != nil {
			http.Error(w, "report queue busy or unavailable", 409)
			return
		}
		writeJSON(w, 202, map[string]any{"status": "generating", "job_id": jid, "report_id": source.ReportID})
		return
	}
	if r.Method == "PUT" {
		editReport(ctx, w, r, inc, kind)
		return
	}
	ids, err := store.disk.rdb.ZRevRange(ctx, reportIndex(inc.ID, kind), 0, 49).Result()
	if err != nil {
		http.Error(w, "reports unavailable", 503)
		return
	}
	reports := []ReportVersion{}
	for _, id := range ids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:report:"+id).Bytes()
		if err == redis.Nil {
			continue
		}
		var report ReportVersion
		if err != nil || json.Unmarshal(raw, &report) != nil || report.IncidentID != inc.ID || report.Kind != kind {
			http.Error(w, "report integrity unavailable", 503)
			return
		}
		reports = append(reports, report)
	}
	selected := r.URL.Query().Get("report_id")
	var current *ReportVersion
	for i := range reports {
		if selected == "" || reports[i].ID == selected {
			current = &reports[i]
			break
		}
	}
	if strings.HasSuffix(r.URL.Path, "/export") {
		if current == nil {
			http.Error(w, "report not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="incident-report.md"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write([]byte(current.Markdown))
		return
	}
	jobs := []DurableJob{}
	jids, err := store.disk.rdb.ZRevRange(ctx, "igpt:jobs:incident:"+inc.ID, 0, 49).Result()
	if err != nil {
		http.Error(w, "report jobs unavailable", 503)
		return
	}
	for _, id := range jids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:job:"+id).Bytes()
		var e jobEnvelope
		if err != nil || json.Unmarshal(raw, &e) != nil {
			http.Error(w, "report job unavailable", 503)
			return
		}
		if e.Job.Kind == "report" && e.Input.Report != nil && e.Input.Report.Kind == kind {
			jobs = append(jobs, e.Job)
		}
	}
	response := map[string]any{"items": reports, "jobs": jobs, "enabled": env("REPORTS_ENABLED", "false") == "true", "markdown": "", "status": "empty", "review_status": "draft"}
	if current != nil {
		raw, _ := json.Marshal(current)
		json.Unmarshal(raw, &response)
	}
	response["items"] = reports
	response["jobs"] = jobs
	writeJSON(w, 200, response)
}

var finishReportScript = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw then return 0 end
local e=cjson.decode(raw);if e.owner~=ARGV[1] or e.job.status~='running' or e.lease_until<tonumber(ARGV[2]) then return 0 end
e.job.status=ARGV[3];e.job.error_code=ARGV[4]
if ARGV[3]=='queued' then redis.call('ZADD',KEYS[2],ARGV[5],e.job.job_id)
else e.job.finished_at=ARGV[6];redis.call('ZREM',KEYS[2],e.job.job_id);redis.call('ZREM','igpt:jobs:pending:created',e.job.job_id);if redis.call('GET','igpt:report:active:'..e.job.incident_id)==e.job.job_id then redis.call('DEL','igpt:report:active:'..e.job.incident_id) end end
redis.call('SET',KEYS[1],cjson.encode(e))
if ARGV[3]=='complete' then
redis.call('SET',KEYS[3],ARGV[7],'EX',ARGV[8],'NX');redis.call('SET',KEYS[3]..':revision:1',ARGV[7],'EX',ARGV[8],'NX');redis.call('ZADD',KEYS[4],ARGV[2],e.input.report_source.report_id);redis.call('ZREMRANGEBYRANK',KEYS[4],0,-51);redis.call('EXPIRE',KEYS[4],ARGV[8]);redis.call('INCR','igpt:jobs:completed')
local incRaw=redis.call('GET','igpt:incident:'..e.job.incident_id);if incRaw and e.input.report_source.kind=='postmortem' then local inc=cjson.decode(incRaw);inc.has_postmortem=true;redis.call('SET','igpt:incident:'..e.job.incident_id,cjson.encode(inc)) end
elseif ARGV[3]=='failed' then redis.call('INCR','igpt:jobs:failures') end
return 1`)

func runReportJob(ctx context.Context, e *jobEnvelope) {
	source := e.Input.Report
	status, code := "complete", ""
	var report ReportVersion
	if source == nil || source.IncidentID != e.Job.IncidentID || len(source.Markdown) > 32<<10 {
		status, code = "failed", "invalid_report_snapshot"
	} else {
		report = ReportVersion{Version: 1, ID: source.ReportID, IncidentID: source.IncidentID, Kind: source.Kind, Revision: 1, Markdown: source.Markdown, GeneratedAt: time.Now().UTC(), ReviewStatus: "draft", Status: "ready", Source: *source}
		safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", report)
		var clean ReportVersion
		if err != nil || json.Unmarshal(safe, &clean) != nil || clean.ID != report.ID || clean.IncidentID != report.IncidentID || clean.Version != 1 || clean.Revision != 1 || clean.Kind != report.Kind || !sameReportSourceIdentity(clean.Source, report.Source) {
			status, code = "queued", "report_sanitizer_unavailable"
			if e.Job.Attempts >= 4 {
				status = "failed"
			}
		} else {
			report = clean
		}
	}
	if source == nil {
		return
	}
	raw, _ := json.Marshal(report)
	now := time.Now().UTC()
	_ = finishReportScript.Run(ctx, store.disk.rdb, []string{"igpt:job:" + e.Job.ID, jobsDue, "igpt:report:" + source.ReportID, reportIndex(e.Job.IncidentID, source.Kind)}, e.Owner, now.UnixMilli(), status, code, now.Add(time.Duration(1<<e.Job.Attempts)*time.Second).UnixMilli(), now.Format(time.RFC3339Nano), raw, int(reportRetention.Seconds())).Err()
}

var editReportScript = redis.NewScript(`
local raw=redis.call('GET',KEYS[1]);if not raw then return -1 end
local report=cjson.decode(raw);if report.revision~=tonumber(ARGV[1]) then return 0 end
redis.call('SET',KEYS[1],ARGV[2],'EX',ARGV[3]);redis.call('SET',KEYS[2],ARGV[2],'EX',ARGV[3]);return 1`)

func editReport(ctx context.Context, w http.ResponseWriter, r *http.Request, inc *Incident, kind string) {
	var edit struct {
		ID       string         `json:"report_id"`
		Revision int            `json:"revision"`
		Markdown string         `json:"markdown"`
		Review   string         `json:"review_status"`
		Actions  []ReportAction `json:"action_items"`
	}
	if decodeReport(r, &edit) != nil || edit.ID == "" || edit.Revision < 1 || edit.Revision >= 1000 || len(edit.Markdown) > 32<<10 || len(edit.Actions) > 30 || (edit.Review != "draft" && edit.Review != "reviewed") {
		http.Error(w, "report ID, revision, bounded markdown and review status required", 400)
		return
	}
	for _, a := range edit.Actions {
		if len(a.Text) == 0 || len(a.Text) > 1000 || len(a.Owner) > 200 || (a.Status != "open" && a.Status != "in_progress" && a.Status != "done") {
			http.Error(w, "invalid action item", 400)
			return
		}
		if a.DueDate != "" {
			if _, err := time.Parse("2006-01-02", a.DueDate); err != nil {
				http.Error(w, "invalid due date", 400)
				return
			}
		}
	}
	raw, err := store.disk.rdb.Get(ctx, "igpt:report:"+edit.ID).Bytes()
	var report ReportVersion
	if err != nil || json.Unmarshal(raw, &report) != nil || report.IncidentID != inc.ID || report.Kind != kind {
		http.Error(w, "report not found", 404)
		return
	}
	if report.Revision != edit.Revision {
		http.Error(w, "report changed; reload before saving", 409)
		return
	}
	actor, _, _ := r.BasicAuth()
	now := time.Now().UTC()
	report.Markdown = edit.Markdown
	if cut := strings.Index(report.Markdown, "\n## Reviewed action items\n"); cut >= 0 {
		report.Markdown = report.Markdown[:cut]
	}
	if len(edit.Actions) > 0 {
		report.Markdown += "\n## Reviewed action items\n\n"
		for _, action := range edit.Actions {
			report.Markdown += fmt.Sprintf("- %s; owner: %s; status: %s; due: %s\n", markdownFact(action.Text), markdownFact(action.Owner), action.Status, markdownFact(action.DueDate))
		}
	}
	if len(report.Markdown) > 32<<10 {
		http.Error(w, "report including actions exceeds budget", 400)
		return
	}
	report.ReviewStatus = edit.Review
	report.Actions = edit.Actions
	report.Revision++
	report.LastEditedBy = actor
	report.LastEditedAt = &now
	safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", report)
	var clean ReportVersion
	if err != nil || json.Unmarshal(safe, &clean) != nil || clean.ID != report.ID || clean.IncidentID != inc.ID || clean.Revision != report.Revision || clean.Kind != report.Kind || !sameReportSourceIdentity(clean.Source, report.Source) {
		http.Error(w, "report sanitization failed", 503)
		return
	}
	n, err := editReportScript.Run(ctx, store.disk.rdb, []string{"igpt:report:" + edit.ID, fmt.Sprintf("igpt:report:%s:revision:%d", edit.ID, report.Revision)}, edit.Revision, []byte(safe), int(reportRetention.Seconds())).Int()
	if err != nil {
		http.Error(w, "report persistence unavailable", 503)
		return
	}
	if n != 1 {
		http.Error(w, "report changed; reload", 409)
		return
	}
	writeJSON(w, 200, clean)
}
func decodeReport(r *http.Request, v any) error {
	defer r.Body.Close()
	d := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 48<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var trailing any
	if d.Decode(&trailing) != io.EOF {
		return fmt.Errorf("trailing payload")
	}
	return nil
}

func sameReportSourceIdentity(a, b ReportSource) bool {
	a.Markdown = ""
	b.Markdown = ""
	return reflect.DeepEqual(a, b)
}
