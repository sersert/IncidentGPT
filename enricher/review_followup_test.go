package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func reviewCorrelationRedis(t *testing.T) {
	t.Helper()
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires disposable Redis, logical DB 13")
	}
	old, oldCfg := redisClient, appCfg
	redisClient = redis.NewClient(&redis.Options{Addr: addr, DB: 13, ContextTimeoutEnabled: true})
	if err := redisClient.FlushDB(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		redisClient.FlushDB(context.Background())
		redisClient.Close()
		redisClient = old
		appCfg = oldCfg
	})
	t.Setenv("DURABLE_CORRELATION_ENABLED", "true")
	t.Setenv("DEPENDENCY_CORRELATION_ENABLED", "true")
	appCfg.CorrWindow = time.Minute
	appCfg.CorrSettle = time.Second
	// Raw delivery is separately tested; suppress its external fixture path here.
	appCfg.RawDedupTTL = time.Minute
}
func seedReviewDeclaration(t *testing.T, cluster, namespace, workload string) (string, string) {
	t.Helper()
	from := topologyNode(cluster, namespace, "workload", workload)
	target := topologyNode(cluster, "data", "database", "db")
	d := GraphDocument{Version: 1, ClusterID: cluster, Nodes: []GraphNode{{ID: from, ClusterID: cluster, Namespace: namespace, Kind: "workload", Name: workload}, {ID: target, ClusterID: cluster, Namespace: "data", Kind: "database", Name: "db"}}, Edges: []GraphEdge{{ID: "declared", From: from, To: target, Kind: "application_dependency", LastSeen: time.Now(), StaleSeconds: 3600, EvidenceRef: "declared-fixture"}}}
	raw, _ := json.Marshal(d)
	if err := redisClient.Set(context.Background(), "igpt:graph:declared:"+topologyDigest(cluster), raw, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	return from, target
}
func TestDeclaredDependencySurvivesObservedIndexChurn(t *testing.T) {
	reviewCorrelationRedis(t)
	ctx := context.Background()
	cluster := requestID()
	from, _ := seedReviewDeclaration(t, cluster, "api", "web")
	e := EnrichedAlert{ClusterName: cluster, Labels: map[string]string{"namespace": "api", "deployment": "web"}, Fingerprint: "a"}
	expected, _ := dependencyDecision(e)
	for i := 0; i < 100; i++ {
		id := fmt.Sprint("observed-", i)
		edge := GraphEdge{ID: id, From: from, To: "rs", Kind: "ownership", Observed: true, LastSeen: time.Now(), StaleSeconds: 900}
		raw, _ := json.Marshal(edge)
		redisClient.Set(ctx, "igpt:graph:edge:"+id, raw, time.Minute)
		redisClient.ZAdd(ctx, "igpt:graph:out:"+from, redis.Z{Score: float64(i), Member: id})
	}
	redisClient.ZRemRangeByRank(ctx, "igpt:graph:out:"+from, 0, -17)
	got, reason := dependencyDecision(e)
	if got != expected || len(got) < 11 || got[:11] != "dependency-" {
		t.Fatal("observed edges displaced declaration", got, reason)
	}
}
func TestResolveRemovesOriginalPendingGroupAfterGraphOutage(t *testing.T) {
	reviewCorrelationRedis(t)
	ctx := context.Background()
	cluster := requestID()
	seedReviewDeclaration(t, cluster, "api", "web")
	e := EnrichedAlert{ClusterName: cluster, Fingerprint: "fp", Status: "firing", StartsAt: time.Now().UTC(), Labels: map[string]string{"namespace": "api", "deployment": "web"}}
	group, _ := dependencyDecision(e)
	raw, _ := json.Marshal(e)
	if _, err := bufferDurableScript.Run(ctx, redisClient, []string{"igpt:corr:pending:" + group, correlationDue, pendingFingerprintGroup(e)}, e.Fingerprint, raw, time.Now().UnixMilli(), group, 60000).Result(); err != nil {
		t.Fatal(err)
	}
	redisClient.Del(ctx, "igpt:graph:declared:"+topologyDigest(cluster))
	fallback, _ := dependencyDecision(e)
	if group == fallback {
		t.Fatal("test did not change grouping decision")
	}
	e.Status = "resolved"
	if err := bufferDurable(e); err != errSkipResolved {
		t.Fatal(err)
	}
	if redisClient.HExists(ctx, "igpt:corr:pending:"+group, e.Fingerprint).Val() || redisClient.Exists(ctx, pendingFingerprintGroup(e)).Val() != 0 || redisClient.ZScore(ctx, correlationDue, group).Err() != redis.Nil {
		t.Fatal("resolution missed original pending group")
	}
}
func TestResolvedOlderEpisodeCannotRemoveNewPendingEpisode(t *testing.T) {
	reviewCorrelationRedis(t)
	ctx := context.Background()
	group := "same-group"
	old := EnrichedAlert{ClusterName: "prod", Fingerprint: "fp", Status: "firing", StartsAt: time.Now().UTC().Add(-time.Hour)}
	newer := old
	newer.StartsAt = time.Now().UTC()
	for _, e := range []EnrichedAlert{old, newer} {
		raw, _ := json.Marshal(e)
		if _, err := bufferDurableScript.Run(ctx, redisClient, []string{"igpt:corr:pending:" + group, correlationDue, pendingFingerprintGroup(e)}, e.Fingerprint, raw, time.Now().UnixMilli(), group, 60000).Result(); err != nil {
			t.Fatal(err)
		}
	}
	old.Status = "resolved"
	if err := bufferDurable(old); err != errSkipResolved {
		t.Fatal(err)
	}
	if !redisClient.HExists(ctx, "igpt:corr:pending:"+group, "fp").Val() {
		t.Fatal("late resolution removed a newer episode")
	}
	if ttl := redisClient.PTTL(ctx, pendingFingerprintGroup(newer)).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatal(ttl)
	}
}
func TestTopologySubmissionIsBoundedAndDoesNotBlockEnrichment(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-release; w.WriteHeader(503) }))
	defer server.Close()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	t.Setenv("TOPOLOGY_OBSERVATIONS_ENABLED", "true")
	t.Setenv("TOPOLOGY_BACKEND_URL", server.URL)
	e := EnrichedAlert{ClusterName: "prod", Labels: map[string]string{"namespace": "api"}, K8sContext: map[string]string{"observed_pod": "web-1", "observed_node": "node-1"}}
	failures, dropped := topologyObservationFailures.Load(), topologyObservationDropped.Load()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 20; i++ {
			submitTopologyObservation(e)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("topology blocked submission path")
	}
	for i := 0; i < 4; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("bounded slots were not submitted")
		}
	}
	if topologyObservationDropped.Load()-dropped != 16 {
		t.Fatal("unbounded submission queue")
	}
	close(release)
	released = true
	deadline := time.Now().Add(2 * time.Second)
	for topologyObservationFailures.Load()-failures != 4 {
		if time.Now().After(deadline) {
			t.Fatal("HTTP failures not counted")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestRedisStartupTimeoutValidation(t *testing.T) {
	t.Setenv("REDIS_STARTUP_TIMEOUT", "45s")
	if value, err := correlationStartupTimeout(); err != nil || value != 45*time.Second {
		t.Fatal(value, err)
	}
	for _, value := range []string{"bad", "0s", "-1s", "11m"} {
		t.Setenv("REDIS_STARTUP_TIMEOUT", value)
		if _, err := correlationStartupTimeout(); err == nil {
			t.Fatal("accepted invalid startup timeout", value)
		}
	}
}
