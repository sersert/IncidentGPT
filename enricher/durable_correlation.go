package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
)

const correlationDue = "igpt:corr:due"
const batchesDue = "igpt:corr:batches"

func durableCorrelation() bool { return os.Getenv("DURABLE_CORRELATION_ENABLED") == "true" }

var bufferDurableScript = redis.NewScript(`
if redis.call('HLEN',KEYS[1]) >= 50 and redis.call('HEXISTS',KEYS[1],ARGV[1]) == 0 then return 0 end
redis.call('HSET',KEYS[1],ARGV[1],ARGV[2]);redis.call('ZADD',KEYS[2],'NX',ARGV[3],ARGV[4]);return 1`)

func bufferDurable(e EnrichedAlert) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var gk string
	if dependencyCorrelation() {
		gk, e.CorrelationReason = dependencyDecision(e)
	} else {
		gk = groupKeyFor(e)
		e.CorrelationReason = "legacy cluster/namespace correlation window"
	}
	key := "igpt:corr:pending:" + gk
	if e.Status == "resolved" {
		if err := redisClient.HDel(ctx, key, e.Fingerprint).Err(); err != nil {
			return err
		}
		return errSkipResolved
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	n, err := bufferDurableScript.Run(ctx, redisClient, []string{key, correlationDue}, e.Fingerprint, raw, time.Now().Add(appCfg.CorrSettle).UnixMilli(), gk).Int()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("correlation batch limit reached")
	}
	go func() {
		if claimRawDelivery(e) {
			sendRawToBackend(e)
		}
	}()
	return nil
}

var snapshotBatchScript = redis.NewScript(`
local groups=redis.call('ZRANGEBYSCORE',KEYS[1],'-inf',ARGV[1],'LIMIT',0,1)
if #groups == 0 then return nil end
local gk=groups[1];local pending='igpt:corr:pending:'..gk
redis.call('ZREM',KEYS[1],gk)
if redis.call('EXISTS',pending) == 0 then return nil end
local batch='igpt:corr:batch:'..ARGV[2]
redis.call('RENAME',pending,batch)
redis.call('HSET','igpt:corr:meta:'..ARGV[2],'group_key',gk,'attempts',0)
redis.call('ZADD',KEYS[2],ARGV[1],ARGV[2]);return ARGV[2]`)
var claimBatchScript = redis.NewScript(`
local ids=redis.call('ZRANGEBYSCORE',KEYS[1],'-inf',ARGV[1],'LIMIT',0,1)
if #ids == 0 then return nil end
local id=ids[1];local meta='igpt:corr:meta:'..id
local n=redis.call('HINCRBY',meta,'attempts',1)
if n>8 then redis.call('ZREM',KEYS[1],id);redis.call('HSET',meta,'status','dead_letter');redis.call('ZADD','igpt:corr:dead',ARGV[1],id);return nil end
redis.call('HSET',meta,'owner',ARGV[2],'status','running')
redis.call('ZADD',KEYS[1],ARGV[3],id);return id`)
var acknowledgeBatchScript = redis.NewScript(`
if redis.call('HGET',KEYS[1],'owner') ~= ARGV[1] then return 0 end
if ARGV[2] == 'ok' then redis.call('DEL',KEYS[1],KEYS[2]);redis.call('ZREM',KEYS[3],ARGV[3])
else redis.call('HSET',KEYS[1],'status','retrying');redis.call('ZADD',KEYS[3],ARGV[4],ARGV[3]) end
return 1`)

func recoverCorrelation(ctx context.Context) {
	now := time.Now().UnixMilli()
	_, _ = snapshotBatchScript.Run(ctx, redisClient, []string{correlationDue, batchesDue}, now, requestID()).Result()
	owner := requestID()
	id, err := claimBatchScript.Run(ctx, redisClient, []string{batchesDue}, now, owner, time.Now().Add(45*time.Second).UnixMilli()).Text()
	if err != nil {
		return
	}
	meta := "igpt:corr:meta:" + id
	batchKey := "igpt:corr:batch:" + id
	gk, err := redisClient.HGet(ctx, meta, "group_key").Result()
	if err != nil {
		return
	}
	fields, err := redisClient.HGetAll(ctx, batchKey).Result()
	if err != nil || len(fields) > 50 {
		return
	}
	fps := make([]string, 0, len(fields))
	for fp := range fields {
		fps = append(fps, fp)
	}
	sort.Strings(fps)
	alerts := make([]json.RawMessage, 0, len(fps))
	for _, fp := range fps {
		alerts = append(alerts, json.RawMessage(fields[fp]))
	}
	status := "ok"
	if len(alerts) > 0 {
		if sendDurableGroup(ctx, gk, id, alerts) != nil {
			status = "retrying"
		}
	}
	attempts, _ := redisClient.HGet(ctx, meta, "attempts").Int()
	delay := time.Duration(1<<min(attempts, 6)) * time.Second
	_, _ = acknowledgeBatchScript.Run(ctx, redisClient, []string{meta, batchKey, batchesDue}, owner, status, id, time.Now().Add(delay).UnixMilli()).Result()
}
func startDurableCorrelation() {
	if !durableCorrelation() || redisClient == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			recoverCorrelation(ctx)
			cancel()
		}
	}()
}
