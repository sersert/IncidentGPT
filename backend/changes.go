package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ChangeEvent v1 stores allowlisted metadata only. Neither configuration bodies,
// environment variables nor arbitrary upstream summaries are accepted.
type ChangeEvent struct {
	SchemaVersion  int        `json:"schema_version"`
	ID             string     `json:"event_id"`
	ClusterID      string     `json:"cluster_id"`
	Namespace      string     `json:"namespace"`
	Workload       string     `json:"workload"`
	Service        string     `json:"service,omitempty"`
	ResourceUID    string     `json:"resource_uid,omitempty"`
	Source         string     `json:"source"`
	Type           string     `json:"event_type"`
	SourceAt       *time.Time `json:"source_timestamp,omitempty"`
	ObservedAt     time.Time  `json:"observed_timestamp"`
	Revision       string     `json:"revision,omitempty"`
	Images         []string   `json:"images,omitempty"`
	PreviousImages []string   `json:"previous_images,omitempty"`
	Summary        string     `json:"summary"`
	SourceURL      string     `json:"source_url,omitempty"`
	EvidenceIDs    []string   `json:"evidence_ids"`
	Reason         string     `json:"relevance_reason,omitempty"`
}

type ChangeCoverage struct {
	ClusterID   string     `json:"cluster_id"`
	Namespace   string     `json:"namespace"`
	StartedAt   time.Time  `json:"started_at"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	LastAttempt time.Time  `json:"last_attempt"`
	Status      string     `json:"status"`
	// Every process restart opens a new observation interval. Gaps cannot be
	// reconstructed; this is a coverage statement, not a completeness promise.
	IntervalStartedAt time.Time `json:"interval_started_at"`
}

var metadataName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,199}$`)
var revisionName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
var imageName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/@-]{0,255}$`)

func newStableID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("system random unavailable")
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
func digest(parts ...string) string {
	b, _ := json.Marshal(parts)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func changeIndex(cluster, ns, workload string) string {
	return "igpt:changes:index:" + digest(cluster, ns, workload)
}
func changeRetention() time.Duration {
	days, err := strconv.Atoi(env("CHANGE_RETENTION_DAYS", "30"))
	if err != nil || days < 1 || days > 365 {
		days = 30
	}
	return time.Duration(days) * 24 * time.Hour
}
func changesEnabled() bool { return env("CHANGES_ENABLED", "false") == "true" }
func changeServiceAuth(w http.ResponseWriter, r *http.Request) bool {
	if !changesEnabled() {
		http.Error(w, "changes disabled", 503)
		return false
	}
	expected := env("CHANGE_INGEST_TOKEN", "")
	supplied := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(supplied)) != 1 {
		http.Error(w, "unauthorized", 401)
		return false
	}
	if store.disk == nil {
		http.Error(w, "persistent storage unavailable", 503)
		return false
	}
	return true
}

// Timeline readers use real HTTP Basic credentials. X-Auth-User alone grants
// no access. All authenticated readers have the existing single-admin scope.
func changeReadAuth(w http.ResponseWriter, r *http.Request) bool {
	// Browser mutations must originate from the UI host. Service/CLI callers
	// authenticate independently and do not send a browser Origin header.
	if r.Method != "GET" && r.Method != "HEAD" {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, r.Host) || u.User != nil || u.Path != "" || u.RawQuery != "" {
				http.Error(w, "browser origin denied", 403)
				return false
			}
		}
	}
	user, pass, ok := r.BasicAuth()
	expectedUser, expectedPass := env("TIMELINE_READ_USER", ""), env("TIMELINE_READ_PASSWORD", "")
	if !ok || expectedUser == "" || expectedPass == "" || subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(expectedPass)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="IncidentGPT"`)
		http.Error(w, "timeline permission denied", 401)
		return false
	}
	return true
}
func decodeChange(r *http.Request, v any) error {
	defer r.Body.Close()
	d := json.NewDecoder(io.LimitReader(r.Body, 32<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("extra payload")
	}
	return nil
}
func normalizeChange(e *ChangeEvent, now time.Time) error {
	if e.SchemaVersion != 1 || !metadataName.MatchString(e.ClusterID) || e.ClusterID == "unknown" || !metadataName.MatchString(e.Namespace) || !metadataName.MatchString(e.Workload) {
		return fmt.Errorf("invalid identity or schema")
	}
	if e.Service != "" && !metadataName.MatchString(e.Service) {
		return fmt.Errorf("invalid service")
	}
	if e.ResourceUID != "" && !metadataName.MatchString(e.ResourceUID) {
		return fmt.Errorf("invalid uid")
	}
	if e.Revision != "" && !revisionName.MatchString(e.Revision) {
		return fmt.Errorf("invalid revision")
	}
	if len(e.Images) > 20 || len(e.PreviousImages) > 20 {
		return fmt.Errorf("too many images")
	}
	for _, images := range [][]string{e.Images, e.PreviousImages} {
		for _, image := range images {
			if !imageName.MatchString(image) {
				return fmt.Errorf("invalid image")
			}
		}
	}
	if e.Source != "kubernetes" && e.Source != "argocd" && e.Source != "ci" {
		return fmt.Errorf("unsupported source")
	}
	switch e.Type {
	case "rollout_observed", "rollout_complete", "deployment", "configuration_metadata":
	default:
		return fmt.Errorf("unsupported event type")
	}
	if e.SourceAt != nil {
		if e.SourceAt.After(now.Add(5*time.Minute)) || e.SourceAt.Before(now.Add(-365*24*time.Hour)) {
			return fmt.Errorf("source timestamp outside budget")
		}
		t := e.SourceAt.UTC()
		e.SourceAt = &t
	}
	if e.SourceURL != "" {
		u, err := url.Parse(e.SourceURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(e.SourceURL) > 1024 {
			return fmt.Errorf("unsafe source URL")
		}
	}
	upstream := e.ID
	if len(upstream) > 200 {
		return fmt.Errorf("event id too long")
	}
	if e.Source == "kubernetes" && e.ResourceUID == "" {
		return fmt.Errorf("resource uid required")
	}
	if upstream == "" {
		raw, _ := json.Marshal(struct {
			Revision string
			Images   []string
			SourceAt *time.Time
		}{e.Revision, e.Images, e.SourceAt})
		upstream = string(raw)
	}
	e.ID = "change-" + digest(e.ClusterID, e.Namespace, e.Workload, e.Source, e.Type, e.ResourceUID, upstream)
	e.ObservedAt = now.UTC()
	// Caller text is deliberately discarded. Do not turn webhook payloads into
	// a mechanism for persisting raw diffs or instructions.
	e.Summary = "Observed " + e.Type + " metadata; temporal proximity is not proof of causation."
	e.EvidenceIDs = []string{}
	e.Reason = ""
	return nil
}

var changeIngestSlots = make(chan struct{}, 8)

var changeSaveScript = redis.NewScript(`
if redis.call('EXISTS',KEYS[1]) == 1 then return 0 end
redis.call('SET',KEYS[1],ARGV[1],'EX',ARGV[3])
redis.call('ZADD',KEYS[2],ARGV[2],ARGV[4])
redis.call('ZREMRANGEBYSCORE',KEYS[2],'-inf',ARGV[5])
redis.call('EXPIRE',KEYS[2],ARGV[3])
return 1`)

func persistChange(ctx context.Context, e ChangeEvent) (bool, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return false, err
	}
	at := e.ObservedAt
	if e.SourceAt != nil {
		at = *e.SourceAt
	}
	// Older upstream events outside retention are not accepted as recent data.
	if at.Before(time.Now().Add(-changeRetention())) {
		return false, fmt.Errorf("event outside retention")
	}
	result, err := changeSaveScript.Run(ctx, store.disk.rdb, []string{"igpt:change:" + e.ID, changeIndex(e.ClusterID, e.Namespace, e.Workload)}, data, at.UnixMilli(), int(changeRetention().Seconds()), e.ID, time.Now().Add(-changeRetention()).UnixMilli()).Int()
	return result == 1, err
}
func handleChangeIngest(w http.ResponseWriter, r *http.Request) {
	select {
	case changeIngestSlots <- struct{}{}:
		defer func() { <-changeIngestSlots }()
	default:
		http.Error(w, "change ingestion busy; retry later", 503)
		return
	}
	if !changeServiceAuth(w, r) {
		return
	}
	var e ChangeEvent
	if err := decodeChange(r, &e); err != nil {
		http.Error(w, "invalid normalized event", 400)
		return
	}
	if err := normalizeChange(&e, time.Now()); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	// Sanitization precedes persistence for both the collector and webhook path.
	c := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second)
	safe, err := c.SanitizePayload(ctx, "backend", "webhook", e)
	if err != nil {
		http.Error(w, "sanitization unavailable; event not stored", 503)
		return
	}
	var clean ChangeEvent
	if json.Unmarshal(safe, &clean) != nil || clean.ID != e.ID || clean.ClusterID != e.ClusterID || clean.Namespace != e.Namespace || clean.Workload != e.Workload {
		http.Error(w, "sanitizer changed routing identity; event not stored", 503)
		return
	}
	if clean.SourceURL != "" {
		u, err := url.Parse(clean.SourceURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			clean.SourceURL = ""
		}
	}
	created, err := persistChange(ctx, clean)
	if err != nil {
		http.Error(w, "change persistence unavailable", 503)
		return
	}
	writeJSON(w, 200, map[string]any{"event_id": e.ID, "created": created})
}
func handleChangeCoverage(w http.ResponseWriter, r *http.Request) {
	if !changeServiceAuth(w, r) {
		return
	}
	var c ChangeCoverage
	if decodeChange(r, &c) != nil || !metadataName.MatchString(c.ClusterID) || !metadataName.MatchString(c.Namespace) || c.ClusterID == "unknown" || (c.Status != "collecting" && c.Status != "partial" && c.Status != "unavailable") {
		http.Error(w, "invalid coverage", 400)
		return
	}
	now := time.Now().UTC()
	c.LastAttempt = now
	if c.Status == "collecting" {
		c.LastSuccess = &now
	} else {
		c.LastSuccess = nil
	}
	if c.IntervalStartedAt.IsZero() || c.IntervalStartedAt.After(now) {
		http.Error(w, "invalid coverage interval", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	key := "igpt:change:coverage:" + digest(c.ClusterID, c.Namespace)
	previous, err := store.disk.rdb.Get(ctx, key).Bytes()
	if err != nil && err != redis.Nil {
		http.Error(w, "coverage storage unavailable", 503)
		return
	}
	if err == nil {
		var old ChangeCoverage
		if json.Unmarshal(previous, &old) == nil {
			c.StartedAt = old.StartedAt
			if c.LastSuccess == nil {
				c.LastSuccess = old.LastSuccess
			}
		}
	}
	if c.StartedAt.IsZero() {
		c.StartedAt = now
	}
	data, _ := json.Marshal(c)
	if store.disk.rdb.Set(ctx, key, data, changeRetention()).Err() != nil {
		http.Error(w, "coverage persistence unavailable", 503)
		return
	}
	writeJSON(w, 200, c)
}
func incidentWorkload(inc *Incident) string {
	if inc.EnrichedData != nil {
		if ctx, ok := inc.EnrichedData["k8s_context"].(map[string]any); ok {
			if name, ok := ctx["deployment"].(string); ok && name != "" {
				return name
			}
		}
	}
	for _, key := range []string{"deployment", "workload"} {
		if inc.Labels[key] != "" {
			return inc.Labels[key]
		}
	}
	return ""
}
func incidentCluster(inc *Incident) string {
	if inc.ClusterID != "" && inc.ClusterID != "unknown" {
		return inc.ClusterID
	}
	if inc.Labels["cluster"] != "unknown" {
		return inc.Labels["cluster"]
	}
	return ""
}
func incidentSymptom(inc *Incident) *time.Time {
	var earliest *time.Time
	for _, a := range inc.Alerts {
		t, err := time.Parse(time.RFC3339Nano, a.StartsAt)
		if err == nil && !t.IsZero() && (earliest == nil || t.Before(*earliest)) {
			t = t.UTC()
			earliest = &t
		}
	}
	return earliest
}
func handleIncidentChanges(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/changes")
	inc := store.Get(id)
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}
	result := map[string]any{"items": []ChangeEvent{}, "coverage": nil, "status": "disabled", "next_offset": nil, "correlation_only": true}
	if !changesEnabled() {
		writeJSON(w, 200, result)
		return
	}
	if store.disk == nil {
		http.Error(w, "change storage unavailable", 503)
		return
	}
	cluster, workload, symptom := incidentCluster(inc), incidentWorkload(inc), incidentSymptom(inc)
	if cluster == "" || workload == "" || symptom == nil {
		result["status"] = "unknown_incident_identity_or_symptom"
		writeJSON(w, 200, result)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	coverage, err := store.disk.rdb.Get(ctx, "igpt:change:coverage:"+digest(cluster, inc.Namespace)).Bytes()
	if err != nil && err != redis.Nil {
		http.Error(w, "coverage unavailable", 503)
		return
	}
	if err == nil {
		var c ChangeCoverage
		if json.Unmarshal(coverage, &c) != nil {
			http.Error(w, "invalid coverage", 503)
			return
		}
		if time.Since(c.LastAttempt) > 3*time.Minute {
			c.Status = "unavailable"
		}
		result["coverage"] = c
	}
	window, err := time.ParseDuration(env("CHANGE_LOOKBACK", "2h"))
	if err != nil || window < time.Minute || window > 7*24*time.Hour {
		window = 2 * time.Hour
	}
	offset := intQuery(r, "offset", 0)
	limit := intQuery(r, "limit", 25)
	if offset < 0 || offset > 10000 || limit < 1 || limit > 100 {
		http.Error(w, "invalid pagination", 400)
		return
	}
	ids, err := store.disk.rdb.ZRangeByScore(ctx, changeIndex(cluster, inc.Namespace, workload), &redis.ZRangeBy{Min: strconv.FormatInt(symptom.Add(-window).UnixMilli(), 10), Max: strconv.FormatInt(symptom.Add(15*time.Minute).UnixMilli(), 10), Offset: int64(offset), Count: int64(limit + 1)}).Result()
	if err != nil {
		http.Error(w, "change lookup unavailable", 503)
		return
	}
	if len(ids) > limit {
		result["next_offset"] = offset + limit
		ids = ids[:limit]
	}
	items := make([]ChangeEvent, 0, len(ids))
	for _, eventID := range ids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:change:"+eventID).Bytes()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			http.Error(w, "change records unavailable", 503)
			return
		}
		var e ChangeEvent
		if json.Unmarshal(raw, &e) != nil {
			http.Error(w, "invalid change record", 503)
			return
		}
		e.Reason = "Same cluster, namespace and workload; within symptom time window"
		items = append(items, e)
	}
	result["items"] = items
	result["status"] = "available"
	result["first_symptom_at"] = symptom
	writeJSON(w, 200, result)
}
