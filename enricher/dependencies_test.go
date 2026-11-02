package main

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"io"
	"net"
	"os"
	"strings"
	"sync"
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

func TestDurableCorrelationWaitsForLateRedis(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires dedicated Redis")
	}
	oldClient, oldCfg := redisClient, appCfg
	redisClient = nil
	defer func() {
		if redisClient != nil {
			redisClient.Close()
		}
		redisClient = oldClient
		appCfg = oldCfg
	}()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	appCfg.RedisAddr = address
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- awaitCorrelationRedis(ctx) }()
	select {
	case err := <-done:
		t.Fatalf("returned before Redis started: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	proxy, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	var wg sync.WaitGroup
	go func() {
		for {
			conn, e := proxy.Accept()
			if e != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				up, e := net.Dial("tcp", addr)
				if e != nil {
					return
				}
				defer up.Close()
				go func() {
					io.Copy(up, conn)
					up.Close()
					conn.Close()
				}()
				io.Copy(conn, up)
			}()
		}
	}()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if redisClient == nil || redisClient.Ping(ctx).Err() != nil {
		t.Fatal("late Redis not connected")
	}
	redisClient.Close()
	redisClient = nil
	proxy.Close()
	wg.Wait()
}
func TestDurableCorrelationRedisConfigurationAndCancellation(t *testing.T) {
	oldClient, oldCfg := redisClient, appCfg
	redisClient = nil
	defer func() { redisClient = oldClient; appCfg = oldCfg }()
	appCfg.RedisAddr = ""
	if awaitCorrelationRedis(context.Background()) == nil {
		t.Fatal("missing Redis accepted")
	}
	appCfg.RedisAddr = "127.0.0.1:1"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if awaitCorrelationRedis(ctx) != context.Canceled {
		t.Fatal("cancellation ignored")
	}
}
