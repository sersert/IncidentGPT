package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDurableCorrelationSnapshotPreservesArrivals(t *testing.T) {
	addr := os.Getenv("INCIDENTGPT_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires disposable Redis")
	}
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	if rdb.Ping(ctx).Err() != nil {
		t.Fatal("Redis unavailable")
	}
	rdb.Del(ctx, correlationDue, batchesDue)
	gk := requestID()
	pending := "igpt:corr:pending:" + gk
	batchID := requestID()
	first := EnrichedAlert{Fingerprint: "first", Status: "firing"}
	raw, _ := json.Marshal(first)
	if _, err := bufferDurableScript.Run(ctx, rdb, []string{pending, correlationDue, pendingFingerprintGroup(first)}, "first", raw, time.Now().Add(-time.Second).UnixMilli(), gk, 60000).Result(); err != nil {
		t.Fatal(err)
	}
	id, err := snapshotBatchScript.Run(ctx, rdb, []string{correlationDue, batchesDue}, time.Now().UnixMilli(), batchID).Text()
	if err != nil || id != batchID {
		t.Fatal("snapshot failed")
	}
	// Concurrent ingestion gets a new pending hash after the atomic rename.
	second := first
	second.Fingerprint = "second"
	raw, _ = json.Marshal(second)
	_, _ = bufferDurableScript.Run(ctx, rdb, []string{pending, correlationDue, pendingFingerprintGroup(second)}, "second", raw, time.Now().Add(-time.Second).UnixMilli(), gk, 60000).Result()
	_, err = claimBatchScript.Run(ctx, rdb, []string{batchesDue}, time.Now().UnixMilli(), "owner", time.Now().Add(time.Minute).UnixMilli()).Result()
	if err != nil {
		t.Fatal(err)
	}
	meta := "igpt:corr:meta:" + id
	_, _ = acknowledgeBatchScript.Run(ctx, rdb, []string{meta, "igpt:corr:batch:" + id, batchesDue}, "owner", "retrying", id, time.Now().Add(-time.Second).UnixMilli()).Result()
	// Retry is due with no new alert and a second owner fences the first.
	retried, err := claimBatchScript.Run(ctx, rdb, []string{batchesDue}, time.Now().UnixMilli(), "second-owner", time.Now().Add(time.Minute).UnixMilli()).Text()
	if err != nil || retried != id {
		t.Fatal("retry requires new ingest")
	}
	n, _ := acknowledgeBatchScript.Run(ctx, rdb, []string{meta, "igpt:corr:batch:" + id, batchesDue}, "owner", "ok", id, 0).Int()
	if n != 0 {
		t.Fatal("old owner acknowledged")
	}
	_, _ = acknowledgeBatchScript.Run(ctx, rdb, []string{meta, "igpt:corr:batch:" + id, batchesDue}, "second-owner", "ok", id, 0).Result()
	if rdb.HGet(ctx, pending, "second").Err() != nil {
		t.Fatal("ack lost concurrent arrival")
	}
}
