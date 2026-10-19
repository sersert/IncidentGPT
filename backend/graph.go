package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func graphNodeID(cluster, ns, kind, name string) string {
	return "node-" + digest(cluster, ns, kind, name)
}
func normalizeGraph(d *GraphDocument, observed bool) error {
	copy := *d
	copy.Nodes = append([]GraphNode(nil), d.Nodes...)
	copy.Edges = append([]GraphEdge(nil), d.Edges...)
	if err := normalizeGraphValue(&copy, observed); err != nil {
		return err
	}
	*d = copy
	return nil
}
func normalizeGraphValue(d *GraphDocument, observed bool) error {
	if d.Version != 1 || !metadataName.MatchString(d.ClusterID) || d.ClusterID == "unknown" || len(d.Nodes) == 0 || len(d.Nodes) > 200 || len(d.Edges) > 400 {
		return fmt.Errorf("invalid bounded graph")
	}
	aliases := map[string]string{}
	now := time.Now().UTC()
	for i := range d.Nodes {
		n := &d.Nodes[i]
		if !metadataName.MatchString(n.Name) || (n.Kind != "node" && !metadataName.MatchString(n.Namespace)) || (n.Namespace != "" && !metadataName.MatchString(n.Namespace)) || n.ClusterID != d.ClusterID {
			return fmt.Errorf("invalid cluster-aware node")
		}
		switch n.Kind {
		case "workload", "service", "database", "pod", "replicaset", "node":
		default:
			return fmt.Errorf("invalid node type")
		}
		if n.ID == "" || aliases[n.ID] != "" {
			return fmt.Errorf("unique input aliases required")
		}
		id := graphNodeID(n.ClusterID, n.Namespace, n.Kind, n.Name)
		aliases[n.ID] = id
		n.ID = id
		n.LastSeen = now
		n.Status = "unknown"
	}
	if observed && len(d.Edges) == 0 {
		return fmt.Errorf("observation needs a metadata relationship")
	}
	source := "declared_configuration"
	if observed {
		source = "kubernetes_metadata"
	}
	for i := range d.Edges {
		e := &d.Edges[i]
		from, to := aliases[e.From], aliases[e.To]
		if from == "" || to == "" || from == to {
			return fmt.Errorf("unknown or self relationship")
		}
		if observed {
			if e.Kind != "ownership" && e.Kind != "placement" {
				return fmt.Errorf("observations cannot invent application dependencies")
			}
		} else if e.Kind != "application_dependency" {
			return fmt.Errorf("declared configuration accepts application dependencies only")
		}
		e.From = from
		e.To = to
		e.Source = source
		e.Observed = observed
		e.LastSeen = now
		if observed {
			e.StaleSeconds = 900
		} else {
			e.StaleSeconds = 7 * 86400
		}
		e.ID = "edge-" + digest(from, to, e.Kind, source)
	}
	raw, _ := json.Marshal(d)
	reference := "graph-observation-" + digest(string(raw))
	for i := range d.Edges {
		d.Edges[i].EvidenceRef = reference
	}
	return nil
}
func persistGraph(ctx context.Context, d GraphDocument) error {
	safe, err := newSanitizerClient(env("SANITIZER_URL", ""), env("SANITIZER_AUTH_SHARED_SECRET", ""), 3*time.Second).SanitizePayload(ctx, "backend", "webhook", d)
	if err != nil {
		return fmt.Errorf("graph sanitization failed")
	}
	var clean GraphDocument
	if json.Unmarshal(safe, &clean) != nil || clean.ClusterID != d.ClusterID || len(clean.Nodes) != len(d.Nodes) || len(clean.Edges) != len(d.Edges) {
		return fmt.Errorf("sanitized graph invalid")
	}
	for i, n := range clean.Nodes {
		if n.ID != d.Nodes[i].ID || n.ClusterID != d.ClusterID || n.Name != d.Nodes[i].Name || n.Namespace != d.Nodes[i].Namespace || n.Kind != d.Nodes[i].Kind {
			return fmt.Errorf("graph routing identity changed")
		}
	}
	// Serialize replacement declarations; stale or withdrawn declared edges must
	// stop affecting grouping immediately rather than waiting seven days.
	if len(d.Edges) == 0 || !d.Edges[0].Observed {
		owner := newStableID("graph-owner")
		lock := "igpt:graph:lock:" + digest(d.ClusterID)
		ok, err := store.disk.rdb.SetNX(ctx, lock, owner, 30*time.Second).Result()
		if err != nil || !ok {
			return fmt.Errorf("graph update busy")
		}
		defer redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`).Run(context.Background(), store.disk.rdb, []string{lock}, owner)
	}
	pipe := store.disk.rdb.TxPipeline()
	if len(d.Edges) == 0 || !d.Edges[0].Observed {
		prior, err := store.disk.rdb.Get(ctx, "igpt:graph:declared:"+digest(d.ClusterID)).Bytes()
		if err != nil && err != redis.Nil {
			return err
		}
		if err == nil {
			var old GraphDocument
			if json.Unmarshal(prior, &old) != nil {
				return fmt.Errorf("invalid prior graph")
			}
			for _, e := range old.Edges {
				pipe.Del(ctx, "igpt:graph:edge:"+e.ID)
				pipe.ZRem(ctx, "igpt:graph:edges:"+digest(d.ClusterID), e.ID)
				pipe.ZRem(ctx, "igpt:graph:out:"+e.From, e.ID)
			}
		}
		pipe.Set(ctx, "igpt:graph:declared:"+digest(d.ClusterID), []byte(safe), 30*24*time.Hour)
	}
	now := time.Now().UTC()
	ttl := 30 * 24 * time.Hour
	for _, n := range clean.Nodes {
		raw, _ := json.Marshal(n)
		pipe.Set(ctx, "igpt:graph:node:"+n.ID, raw, ttl)
		idx := "igpt:graph:nodes:" + digest(d.ClusterID)
		pipe.ZAdd(ctx, idx, redis.Z{Score: float64(now.Unix()), Member: n.ID})
		pipe.ZRemRangeByRank(ctx, idx, 0, -2001)
		pipe.Expire(ctx, idx, ttl)
	}
	for i, e := range clean.Edges {
		if e.ID != d.Edges[i].ID || e.From != d.Edges[i].From || e.To != d.Edges[i].To || e.Kind != d.Edges[i].Kind {
			return fmt.Errorf("edge identity changed")
		}
		raw, _ := json.Marshal(e)
		pipe.Set(ctx, "igpt:graph:edge:"+e.ID, raw, ttl)
		idx := "igpt:graph:edges:" + digest(d.ClusterID)
		pipe.ZAdd(ctx, idx, redis.Z{Score: float64(now.Unix()), Member: e.ID})
		pipe.ZRemRangeByRank(ctx, idx, 0, -4001)
		pipe.Expire(ctx, idx, ttl)
		out := "igpt:graph:out:" + e.From
		pipe.ZAdd(ctx, out, redis.Z{Score: float64(now.Unix()), Member: e.ID})
		pipe.ZRemRangeByRank(ctx, out, 0, -17)
		pipe.Expire(ctx, out, ttl)
		pipe.SetNX(ctx, "igpt:graph:evidence:"+e.EvidenceRef, []byte(safe), ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}
func handleGraphWrite(w http.ResponseWriter, r *http.Request) {
	if store.disk == nil {
		http.Error(w, "graph storage unavailable", 503)
		return
	}
	observed := r.Method == "POST"
	if observed {
		if !trustedBearer(r, env("ANALYSIS_SERVICE_TOKEN", "")) {
			http.Error(w, "unauthorized", 401)
			return
		}
	} else if !changeReadAuth(w, r) {
		return
	}
	var d GraphDocument
	if decodeChange(r, &d) != nil {
		http.Error(w, "invalid graph document", 400)
		return
	}
	if err := normalizeGraph(&d, observed); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := persistGraph(ctx, d); err != nil {
		http.Error(w, "graph persistence or sanitization unavailable", 503)
		return
	}
	writeJSON(w, 200, d)
}
func serviceGraph(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "graph storage unavailable", 503)
		return
	}
	cluster := r.URL.Query().Get("cluster_id")
	if id := r.URL.Query().Get("incident_id"); id != "" {
		inc := store.Get(id)
		if inc == nil {
			http.Error(w, "not found", 404)
			return
		}
		cluster = incidentCluster(inc)
	}
	if cluster == "" {
		writeJSON(w, 200, map[string]any{"nodes": []GraphNode{}, "edges": []GraphEdge{}, "status": "cluster_unknown"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	nids, err := store.disk.rdb.ZRevRange(ctx, "igpt:graph:nodes:"+digest(cluster), 0, 199).Result()
	if err != nil {
		http.Error(w, "graph query failed", 503)
		return
	}
	eids, err := store.disk.rdb.ZRevRange(ctx, "igpt:graph:edges:"+digest(cluster), 0, 399).Result()
	if err != nil {
		http.Error(w, "graph query failed", 503)
		return
	}
	nodes := []GraphNode{}
	edges := []GraphEdge{}
	known := map[string]bool{}
	for _, id := range nids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:graph:node:"+id).Bytes()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			http.Error(w, "graph node unavailable", 503)
			return
		}
		var n GraphNode
		if json.Unmarshal(raw, &n) != nil || n.ClusterID != cluster {
			http.Error(w, "graph integrity failure", 503)
			return
		}
		nodes = append(nodes, n)
		known[n.ID] = true
	}
	stale := 0
	for _, id := range eids {
		raw, err := store.disk.rdb.Get(ctx, "igpt:graph:edge:"+id).Bytes()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			http.Error(w, "graph edge unavailable", 503)
			return
		}
		var e GraphEdge
		if json.Unmarshal(raw, &e) != nil {
			http.Error(w, "graph integrity failure", 503)
			return
		}
		if time.Since(e.LastSeen) > time.Duration(e.StaleSeconds)*time.Second {
			stale++
			continue
		}
		if known[e.From] && known[e.To] {
			edges = append(edges, e)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	writeJSON(w, 200, map[string]any{"nodes": nodes, "edges": edges, "stale_edges_omitted": stale, "bounded": len(nids) == 200 || len(eids) == 400, "status": "available"})
}
func serviceDetail(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "graph storage unavailable", 503)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	raw, err := store.disk.rdb.Get(ctx, "igpt:graph:node:"+id).Bytes()
	if err == redis.Nil {
		http.Error(w, "not found", 404)
		return
	}
	if err != nil {
		http.Error(w, "node unavailable", 503)
		return
	}
	var n GraphNode
	if json.Unmarshal(raw, &n) != nil {
		http.Error(w, "node invalid", 503)
		return
	}
	writeJSON(w, 200, n)
}
func startGraphConfiguration() {
	path := env("SERVICE_GRAPH_FILE", "")
	if path == "" || store.disk == nil {
		return
	}
	load := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		f, err := os.Open(path)
		if err != nil {
			return
		}
		raw, err := io.ReadAll(io.LimitReader(f, (32<<10)+1))
		f.Close()
		if err != nil || len(raw) > 32<<10 {
			return
		}
		var d GraphDocument
		if json.Unmarshal(raw, &d) != nil || normalizeGraph(&d, false) != nil {
			return
		}
		_ = persistGraph(ctx, d)
	}
	load()
	go func() {
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for range t.C {
			load()
		}
	}()
}
