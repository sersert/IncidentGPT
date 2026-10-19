package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

func topologyDigest(parts ...string) string {
	raw, _ := json.Marshal(parts)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func topologyNode(cluster, ns, kind, name string) string {
	return "node-" + topologyDigest(cluster, ns, kind, name)
}
func dependencyCorrelation() bool {
	return durableCorrelation() && os.Getenv("DEPENDENCY_CORRELATION_ENABLED") == "true"
}
func dependencyDecision(e EnrichedAlert) (string, string) {
	cluster, ns := e.ClusterName, e.Labels["namespace"]
	workload := e.K8sContext["deployment"]
	if workload == "" {
		workload = e.Labels["deployment"]
	}
	if workload == "" {
		workload = e.Labels["service"]
	}
	resource := workload
	if resource == "" {
		resource = e.Labels["pod"]
	}
	if resource == "" {
		resource = e.Fingerprint
	}
	fallback := "resource-" + topologyDigest(cluster, ns, resource)
	reason := "same cluster and workload within the correlation settle window; topology unavailable"
	if cluster == "" || cluster == "unknown" {
		return "isolated-" + topologyDigest(e.Fingerprint, e.StartsAt.String()), "cluster unknown; no cross-resource grouping"
	}
	if workload == "" && e.Labels["pod"] == "" && e.Labels["node"] != "" {
		return "node-resource-" + topologyDigest(cluster, e.Labels["node"]), "same node-scoped reported resource within the settle window; no application dependency inferred"
	}
	if workload == "" || redisClient == nil {
		return fallback, reason
	}
	kind := "workload"
	if e.Labels["deployment"] == "" && e.K8sContext["deployment"] == "" {
		kind = "service"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	ids, err := redisClient.ZRevRange(ctx, "igpt:graph:out:"+topologyNode(cluster, ns, kind, workload), 0, 15).Result()
	if err != nil || len(ids) >= 16 {
		return fallback, "degraded: topology unavailable or candidate limit; workload grouping"
	}
	target, ref := "", ""
	for _, id := range ids {
		raw, err := redisClient.Get(ctx, "igpt:graph:edge:"+id).Bytes()
		if err != nil {
			return fallback, "degraded: missing topology record; workload grouping"
		}
		var edge GraphEdge
		if json.Unmarshal(raw, &edge) != nil {
			return fallback, "degraded: invalid topology; workload grouping"
		}
		if edge.Kind != "application_dependency" || edge.Observed || edge.StaleSeconds <= 0 || time.Since(edge.LastSeen) > time.Duration(edge.StaleSeconds)*time.Second {
			continue
		}
		nodeRaw, err := redisClient.Get(ctx, "igpt:graph:node:"+edge.To).Bytes()
		var node GraphNode
		if err != nil || json.Unmarshal(nodeRaw, &node) != nil || node.ClusterID != cluster {
			return fallback, "degraded: dependency identity unavailable; workload grouping"
		}
		if target != "" && target != edge.To {
			return fallback, "degraded: conflicting dependencies; workload grouping"
		}
		target, ref = edge.To, edge.EvidenceRef
	}
	if target == "" {
		return fallback, reason
	}
	return "dependency-" + topologyDigest(cluster, target), "shared dependency suspected; declared relationship " + ref + "; traversal depth 1, no causal conclusion"
}

var topologySlots = make(chan struct{}, 4)

func observeTopology(ctx context.Context, e EnrichedAlert) {
	if os.Getenv("TOPOLOGY_OBSERVATIONS_ENABLED") != "true" || e.ClusterName == "" || e.ClusterName == "unknown" || e.K8sContext["observed_pod"] == "" {
		return
	}
	select {
	case topologySlots <- struct{}{}:
		defer func() { <-topologySlots }()
	default:
		return
	}
	d := GraphDocument{Version: 1, ClusterID: e.ClusterName}
	ns := e.Labels["namespace"]
	add := func(alias, kind, name, namespace string) {
		d.Nodes = append(d.Nodes, GraphNode{ID: alias, ClusterID: e.ClusterName, Namespace: namespace, Kind: kind, Name: name})
	}
	add("pod", "pod", e.K8sContext["observed_pod"], ns)
	if rs := e.K8sContext["observed_replicaset"]; rs != "" {
		add("rs", "replicaset", rs, ns)
		d.Edges = append(d.Edges, GraphEdge{From: "rs", To: "pod", Kind: "ownership"})
		if dep := e.K8sContext["deployment"]; dep != "" {
			add("workload", "workload", dep, ns)
			d.Edges = append(d.Edges, GraphEdge{From: "workload", To: "rs", Kind: "ownership"})
		}
	}
	if node := e.K8sContext["observed_node"]; node != "" {
		add("node", "node", node, "")
		d.Edges = append(d.Edges, GraphEdge{From: "pod", To: "node", Kind: "placement"})
	}
	raw, _ := json.Marshal(d)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", os.Getenv("TOPOLOGY_BACKEND_URL"), bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("ANALYSIS_SERVICE_TOKEN"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err == nil {
		resp.Body.Close()
	}
}
