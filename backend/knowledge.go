package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Resolution struct {
	Composition    string    `json:"composition_hash,omitempty"`
	Schema         int       `json:"schema_version"`
	IncidentID     string    `json:"incident_id"`
	Revision       int       `json:"revision"`
	Status         string    `json:"status"` // confirmed, unknown, withdrawn
	Cause          string    `json:"confirmed_cause"`
	Action         string    `json:"restoring_action"`
	Failed         []string  `json:"unsuccessful_attempts"`
	EvidenceIDs    []string  `json:"verification_evidence_ids"`
	Environment    string    `json:"environment"`
	Versions       string    `json:"versions"`
	FailureMode    string    `json:"failure_mode"`
	ErrorSignature string    `json:"error_signature"`
	Links          []string  `json:"links"`
	Actor          string    `json:"confirming_actor"`
	At             time.Time `json:"confirmed_at"`
	ClusterID      string    `json:"cluster_id"`
	Service        string    `json:"service"`
}
type Runbook struct {
	ID          string    `json:"document_id"`
	Version     string    `json:"version_hash"`
	Title       string    `json:"title"`
	Excerpt     string    `json:"excerpt"`
	CollectedAt time.Time `json:"collected_at"`
	ModifiedAt  time.Time `json:"modified_at"`
}

func knowledgeIndex(cluster, service string) string {
	return "igpt:knowledge:index:" + digest(cluster, service)
}

var saveResolutionScript = redis.NewScript(`
local prior=redis.call('GET',KEYS[1]);local rev=0
if prior then rev=cjson.decode(prior).revision end
if rev ~= tonumber(ARGV[1]) then return 0 end
redis.call('SET',KEYS[1],ARGV[2]);redis.call('SET',KEYS[2],ARGV[2])
if ARGV[4]=='confirmed' then redis.call('ZADD',KEYS[3],ARGV[3],ARGV[5]);redis.call('ZADD',KEYS[4],ARGV[3],ARGV[5])
else redis.call('ZREM',KEYS[3],ARGV[5]);redis.call('ZREM',KEYS[4],ARGV[5]) end
return 1`)

func validateEvidenceOwnership(ctx context.Context, id string, evidence []string) error {
	if len(evidence) > 100 {
		return fmt.Errorf("too many evidence references")
	}
	for _, eid := range evidence {
		raw, err := store.disk.rdb.Get(ctx, "igpt:evidence:"+eid).Bytes()
		if err != nil {
			return fmt.Errorf("evidence unavailable")
		}
		var e Evidence
		if json.Unmarshal(raw, &e) != nil || e.IncidentID != id || e.Availability != "available" {
			return fmt.Errorf("verification requires available incident evidence")
		}
	}
	return nil
}
func handleResolution(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "knowledge storage unavailable", 503)
		return
	}
	id := r.PathValue("id")
	inc := store.Get(id)
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	key := "igpt:resolution:" + id
	if r.Method == "GET" {
		raw, err := store.disk.rdb.Get(ctx, key).Bytes()
		if err == redis.Nil {
			writeJSON(w, 200, map[string]any{"resolution": nil})
			return
		}
		if err != nil {
			http.Error(w, "resolution unavailable", 503)
			return
		}
		var record Resolution
		if json.Unmarshal(raw, &record) != nil {
			http.Error(w, "invalid resolution", 503)
			return
		}
		writeJSON(w, 200, map[string]any{"resolution": record})
		return
	}
	var record Resolution
	if decodeChange(r, &record) != nil || record.Revision < 0 || len(record.Cause) > 4000 || len(record.Action) > 4000 || len(record.Failed) > 20 || len(record.Links) > 10 || len(record.Environment) > 200 || len(record.Versions) > 200 || len(record.FailureMode) > 200 || len(record.ErrorSignature) > 200 {
		http.Error(w, "invalid bounded resolution", 400)
		return
	}
	if record.Status != "confirmed" && record.Status != "unknown" && record.Status != "withdrawn" {
		http.Error(w, "invalid confirmation state", 400)
		return
	}
	if record.Status == "confirmed" && (strings.TrimSpace(record.Cause) == "" || strings.TrimSpace(record.Action) == "" || len(record.EvidenceIDs) == 0) {
		http.Error(w, "confirmation requires cause, restoring action and verification evidence", 400)
		return
	}
	if record.Status == "unknown" {
		record.Cause = ""
	}
	for _, attempt := range record.Failed {
		if len(attempt) > 2000 {
			http.Error(w, "attempt exceeds budget", 400)
			return
		}
	}
	for _, link := range record.Links {
		check := ChangeEvent{SchemaVersion: 1, ClusterID: "check", Namespace: "check", Workload: "check", Source: "ci", Type: "deployment", SourceURL: link}
		if normalizeChange(&check, time.Now()) != nil {
			http.Error(w, "unsafe link", 400)
			return
		}
	}
	if err := validateEvidenceOwnership(ctx, id, record.EvidenceIDs); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	oldRevision := record.Revision
	record.Revision++
	record.Schema = 1
	record.IncidentID = id
	record.Actor, _, _ = r.BasicAuth()
	record.At = time.Now().UTC()
	record.ClusterID = incidentCluster(inc)
	record.Composition = incidentComposition(inc)
	record.Service = incidentWorkload(inc)
	if record.Service == "" {
		record.Service = inc.Labels["service"]
	}
	safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", record)
	if err != nil {
		http.Error(w, "sanitization failed; resolution not saved", 503)
		return
	}
	var clean Resolution
	if json.Unmarshal(safe, &clean) != nil || clean.IncidentID != id || clean.Revision != record.Revision || clean.ClusterID != record.ClusterID || clean.Service != record.Service || clean.Status != record.Status {
		http.Error(w, "sanitized identity invalid", 503)
		return
	}
	if validateEvidenceOwnership(ctx, id, clean.EvidenceIDs) != nil {
		http.Error(w, "sanitized verification references invalid", 503)
		return
	}
	clean.Actor = record.Actor
	clean.At = record.At
	data, _ := json.Marshal(clean)
	n, err := saveResolutionScript.Run(ctx, store.disk.rdb, []string{key, fmt.Sprintf("igpt:resolution:history:%s:%d", id, clean.Revision), knowledgeIndex(clean.ClusterID, clean.Service), knowledgeIndex(clean.ClusterID, "")}, oldRevision, data, clean.At.UnixMilli(), clean.Status, id).Int()
	if err != nil {
		http.Error(w, "resolution persistence failed", 503)
		return
	}
	if n != 1 {
		http.Error(w, "resolution changed; reload before editing", 409)
		return
	}
	writeJSON(w, 200, clean)
}

type KnowledgeMatch struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Date        string     `json:"date"`
	Resolution  Resolution `json:"resolution"`
	Score       int        `json:"score"`
	Matches     []string   `json:"matching_features"`
	Differences []string   `json:"differences"`
}

func rankResolution(r Resolution, inc *Incident, query string) (int, []string, []string) {
	score := 0
	matches := []string{}
	differences := []string{}
	if inc != nil {
		service := incidentWorkload(inc)
		if service == "" {
			service = inc.Labels["service"]
		}
		if r.Service != "" && r.Service == service {
			score += 100
			matches = append(matches, "same service/workload")
		}
		if r.Environment == "" || inc.Labels["environment"] == "" {
			differences = append(differences, "environment unknown")
		} else if r.Environment != inc.Labels["environment"] {
			differences = append(differences, "environment differs or is unknown")
		}
		if r.Versions == "" || inc.Labels["version"] == "" {
			differences = append(differences, "version unknown")
		} else if r.Versions != inc.Labels["version"] {
			differences = append(differences, "version differs or is unknown")
		}
		for _, a := range inc.Alerts {
			if r.FailureMode != "" && strings.EqualFold(r.FailureMode, a.AlertName) {
				score += 30
				matches = append(matches, "same alert/failure mode")
				break
			}
		}
	}
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized != "" && strings.Contains(strings.ToLower(r.Cause+" "+r.Action+" "+r.ErrorSignature+" "+r.FailureMode), normalized) {
		score += 50
		matches = append(matches, "matching text/error signature")
	}
	return score, matches, differences
}
func knowledgeSearch(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "knowledge storage unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var inc *Incident
	cluster, service := r.URL.Query().Get("cluster_id"), r.URL.Query().Get("service")
	if id := r.URL.Query().Get("incident_id"); id != "" {
		inc = store.Get(id)
		if inc == nil {
			http.Error(w, "not found", 404)
			return
		}
		cluster = incidentCluster(inc)
		service = incidentWorkload(inc)
		if service == "" {
			service = inc.Labels["service"]
		}
	}
	query := r.URL.Query().Get("q")
	if len(query) > 200 || len(cluster) > 200 || len(service) > 200 {
		http.Error(w, "query budget exceeded", 400)
		return
	}
	if cluster == "" || cluster == "unknown" {
		writeJSON(w, 200, map[string]any{"results": []KnowledgeMatch{}, "total": 0, "reason": "cluster identity required", "runbooks": []Runbook{}})
		return
	}
	ids, err := store.disk.rdb.ZRevRange(ctx, knowledgeIndex(cluster, service), 0, 199).Result()
	if err != nil {
		http.Error(w, "knowledge query failed", 503)
		return
	}
	matches := []KnowledgeMatch{}
	for _, id := range ids {
		if inc != nil && id == inc.ID {
			continue
		}
		raw, err := store.disk.rdb.Get(ctx, "igpt:resolution:"+id).Bytes()
		if err != nil {
			http.Error(w, "resolution query failed", 503)
			return
		}
		var record Resolution
		if json.Unmarshal(raw, &record) != nil {
			http.Error(w, "invalid resolution", 503)
			return
		}
		if record.Status != "confirmed" || record.ClusterID != cluster {
			continue
		}
		score, features, caveats := rankResolution(record, inc, query)
		if (query != "" || inc != nil) && score == 0 {
			continue
		}
		matches = append(matches, KnowledgeMatch{ID: id, Title: record.Cause, Date: record.At.Format(time.RFC3339), Resolution: record, Score: score, Matches: features, Differences: caveats})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	offset, limit := intQuery(r, "offset", 0), intQuery(r, "limit", 10)
	if offset < 0 || offset > 200 || limit < 1 || limit > 50 {
		http.Error(w, "invalid pagination", 400)
		return
	}
	end := min(len(matches), offset+limit)
	start := min(offset, len(matches))
	books, bookErr := readRunbooks(ctx, query)
	bookStatus := "available"
	if bookErr != nil {
		bookStatus = "unavailable"
		if len(books) > 0 {
			bookStatus = "partial"
		}
	}
	writeJSON(w, 200, map[string]any{"results": matches[start:end], "total": len(matches), "candidate_limit_applied": len(ids) == 200, "runbooks": books, "runbook_status": bookStatus})
}
func readRunbooks(ctx context.Context, query string) ([]Runbook, error) {
	books := []Runbook{}
	root := env("RUNBOOK_DIRECTORY", "")
	if root == "" {
		return books, nil
	}
	files, err := os.ReadDir(root)
	if err != nil {
		return books, err
	}
	if len(files) > 100 {
		return books, fmt.Errorf("runbook directory exceeds budget")
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		canonicalRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return books, err
		}
		canonicalRoot, err = filepath.Abs(canonicalRoot)
		if err != nil {
			return books, err
		}
		path, err := filepath.EvalSymlinks(filepath.Join(root, file.Name()))
		if err != nil {
			return books, err
		}
		path, err = filepath.Abs(path)
		if err != nil || !strings.HasPrefix(path, canonicalRoot+string(filepath.Separator)) {
			return books, fmt.Errorf("runbook escapes configured directory")
		}
		stat, err := os.Stat(path)
		if err != nil || !stat.Mode().IsRegular() || stat.Size() > 64<<10 {
			return books, fmt.Errorf("invalid bounded runbook")
		}
		f, err := os.Open(path)
		if err != nil {
			return books, err
		}
		raw, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
		f.Close()
		if err != nil || len(raw) > 64<<10 {
			return books, fmt.Errorf("runbook read exceeds budget")
		}
		text := string(raw)
		if query != "" && !strings.Contains(strings.ToLower(text), strings.ToLower(query)) {
			continue
		}
		title := strings.Trim(strings.SplitN(text, "\n", 2)[0], "# ")
		book := Runbook{ID: "runbook-" + digest(file.Name()), Version: digest(text), Title: boundedExcerpt(title, 200), Excerpt: boundedExcerpt(text, 4096), CollectedAt: time.Now().UTC(), ModifiedAt: stat.ModTime().UTC()}
		safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", book)
		if err != nil {
			return books, err
		}
		var clean Runbook
		if json.Unmarshal(safe, &clean) != nil {
			return books, fmt.Errorf("runbook sanitization invalid")
		}
		if store.disk != nil {
			data, _ := json.Marshal(clean)
			if store.disk.rdb.SetNX(ctx, "igpt:runbook:version:"+clean.ID+":"+clean.Version, data, 30*24*time.Hour).Err() != nil {
				return books, fmt.Errorf("runbook persistence unavailable")
			}
		}
		books = append(books, clean)
		if len(books) == 5 {
			break
		}
	}
	return books, nil
}
func knowledgePatterns(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "knowledge storage unavailable", 503)
		return
	}
	cluster, service := r.URL.Query().Get("cluster_id"), r.URL.Query().Get("service")
	if cluster == "" {
		writeJSON(w, 200, []any{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	ids, err := store.disk.rdb.ZRevRange(ctx, knowledgeIndex(cluster, service), 0, 199).Result()
	if err != nil {
		http.Error(w, "patterns unavailable", 503)
		return
	}
	counts := map[string]int{}
	for _, id := range ids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:resolution:"+id).Bytes()
		if err != nil {
			http.Error(w, "patterns records unavailable", 503)
			return
		}
		var record Resolution
		if json.Unmarshal(raw, &record) != nil {
			http.Error(w, "invalid resolution", 503)
			return
		}
		if record.Status == "confirmed" && record.ClusterID == cluster && record.FailureMode != "" {
			counts[record.FailureMode]++
		}
	}
	type patternCount struct {
		Pattern string `json:"pattern"`
		Count   int    `json:"count"`
	}
	result := []patternCount{}
	for pattern, count := range counts {
		result = append(result, patternCount{pattern, count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Pattern < result[j].Pattern })
	w.Header().Set("X-Candidate-Limit", "200")
	writeJSON(w, 200, result)
}
