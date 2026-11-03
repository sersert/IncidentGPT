package main

import (
	"context"
	"encoding/json"
	"fmt"
)

const alertIndexReady = "igpt:alertfp:ready:v1"

func alertFingerprintKey(fp string) string { return "igpt:alertfp:" + digest(fp) }

// Only the latest firing episode owns the lookup. Resolving an older episode
// cannot remove a newer episode's binding. No incident JSON passes through cjson.
const updateAlertIndex = `
if ARGV[4]~='' and redis.call('GET',KEYS[2])~=ARGV[4] then return 0 end
local priorID=redis.call('HGET',KEYS[1],'alert_id')
if priorID then
 local priorRaw=redis.call('GET','igpt:alert:'..priorID)
 if not priorRaw or cjson.decode(priorRaw).status~='firing' then redis.call('DEL',KEYS[1]) end
end
if ARGV[3]=='firing' then
 local prior=redis.call('HGET',KEYS[1],'received_at')
 if not prior or tonumber(prior)<=tonumber(ARGV[2]) then redis.call('HSET',KEYS[1],'alert_id',ARGV[1],'received_at',ARGV[2]) end
elseif redis.call('HGET',KEYS[1],'alert_id')==ARGV[1] then redis.call('DEL',KEYS[1]) end
return 1`

// Run once before accepting opt-in regroup traffic, outside the composition
// lock. Bounded SCAN/MGET batches preserve legacy alerts without scanning the
// complete history for each fingerprint under the global ingest lock.
func (s *Storage) ensureAlertIndex(ctx context.Context) error {
	ready, err := s.rdb.Exists(ctx, alertIndexReady).Result()
	if err != nil || ready != 0 {
		return err
	}
	return s.rebuildAlertIndex(ctx)
}

func (s *Storage) rebuildAlertIndex(ctx context.Context) error {
	s.alertIndexEnabled.Store(true)
	var cursor uint64
	for {
		entries, next, err := s.rdb.ZScan(ctx, keyAlertIdx, cursor, "*", 256).Result()
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(entries)/2)
		for i := 0; i < len(entries); i += 2 {
			keys = append(keys, keyAlert+entries[i])
		}
		if len(keys) > 0 {
			values, err := s.rdb.MGet(ctx, keys...).Result()
			if err != nil {
				return err
			}
			pipe := s.rdb.Pipeline()
			for i, value := range values {
				if value == nil {
					continue
				}
				raw, ok := value.(string)
				if !ok {
					return fmt.Errorf("invalid alert index source")
				}
				var a Alert
				if json.Unmarshal([]byte(raw), &a) != nil || a.ID == "" {
					return fmt.Errorf("invalid alert index source")
				}
				if a.Status == "firing" {
					pipe.Eval(ctx, updateAlertIndex, []string{alertFingerprintKey(a.Fingerprint), keys[i]}, a.ID, a.ReceivedAt.UnixMicro(), a.Status, raw)
				}
			}
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return s.rdb.Set(ctx, alertIndexReady, "1", 0).Err()
}
