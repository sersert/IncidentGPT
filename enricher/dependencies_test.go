package main

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDependencyCorrelationBoundaries(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires Redis")
	}
	old := redisClient
	defer func() { redisClient = old }()
	redisClient = redis.NewClient(&redis.Options{Addr: addr})
	defer redisClient.Close()
	t.Setenv("DURABLE_CORRELATION_ENABLED", "true")
	t.Setenv("DEPENDENCY_CORRELATION_ENABLED", "true")
	cluster := requestID()
	a := EnrichedAlert{ClusterName: cluster, Labels: map[string]string{"namespace": "api", "deployment": "web"}}
	b := EnrichedAlert{ClusterName: cluster, Labels: map[string]string{"namespace": "jobs", "deployment": "worker"}}
	ka, _ := dependencyDecision(a)
	kb, _ := dependencyDecision(b)
	if ka == kb {
		t.Fatal("unrelated merged")
	}
	ctx := context.Background()
	node := GraphNode{ID: topologyNode(cluster, "data", "database", "db"), ClusterID: cluster}
	raw, _ := json.Marshal(node)
	redisClient.Set(ctx, "igpt:graph:node:"+node.ID, raw, time.Minute)
	var edges []GraphEdge
	for _, e := range []EnrichedAlert{a, b} {
		from := topologyNode(cluster, e.Labels["namespace"], "workload", e.Labels["deployment"])
		edge := GraphEdge{ID: requestID(), From: from, To: node.ID, Kind: "application_dependency", EvidenceRef: "fixture", LastSeen: time.Now(), StaleSeconds: 100}
		edges = append(edges, edge)
		raw, _ := json.Marshal(edge)
		redisClient.Set(ctx, "igpt:graph:edge:"+edge.ID, raw, time.Minute)
		redisClient.ZAdd(ctx, "igpt:graph:out:"+from, redis.Z{Score: 1, Member: edge.ID})
	}
	ka, reason := dependencyDecision(a)
	kb, _ = dependencyDecision(b)
	if ka != kb || !strings.Contains(reason, "suspected") {
		t.Fatal("shared dependency not explained")
	}
	b.ClusterName = "other"
	kb, _ = dependencyDecision(b)
	if ka == kb {
		t.Fatal("clusters collided")
	}
	edge := edges[0]
	edge.LastSeen = time.Now().Add(-time.Hour)
	raw, _ = json.Marshal(edge)
	redisClient.Set(ctx, "igpt:graph:edge:"+edge.ID, raw, time.Minute)
	stale, _ := dependencyDecision(a)
	if stale == ka {
		t.Fatal("stale dependency used")
	}
	edge.LastSeen = time.Now()
	edge.Kind = "placement"
	raw, _ = json.Marshal(edge)
	redisClient.Set(ctx, "igpt:graph:edge:"+edge.ID, raw, time.Minute)
	placement, _ := dependencyDecision(a)
	if placement == ka {
		t.Fatal("placement inferred dependency")
	}
}

func TestNodeScopedGroupingDoesNotMergePlacedApplications(t *testing.T) {
	t.Setenv("DURABLE_CORRELATION_ENABLED", "true")
	t.Setenv("DEPENDENCY_CORRELATION_ENABLED", "true")
	old := redisClient
	redisClient = nil
	defer func() { redisClient = old }()
	a := EnrichedAlert{ClusterName: "prod", Labels: map[string]string{"node": "worker-1", "alertname": "NodePressure"}}
	b := EnrichedAlert{ClusterName: "prod", Labels: map[string]string{"node": "worker-1", "alertname": "NodeNotReady"}}
	first, reason := dependencyDecision(a)
	second, _ := dependencyDecision(b)
	if first != second || !strings.Contains(reason, "node-scoped") {
		t.Fatal("same reported node not grouped")
	}
	b.ClusterName = "other"
	second, _ = dependencyDecision(b)
	if first == second {
		t.Fatal("node names collided across clusters")
	}
	b.ClusterName = "prod"
	b.Labels["deployment"] = "independent-app"
	b.Labels["namespace"] = "api"
	second, _ = dependencyDecision(b)
	if first == second {
		t.Fatal("placement inferred application dependency")
	}
}
