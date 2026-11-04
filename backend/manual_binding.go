package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const manualBindingTTL = 7 * 24 * time.Hour

func releaseOwnedLock(client *redis.Client, key, owner string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = clearManualBinding.Run(ctx, client, []string{key}, owner).Err()
}

func (s *Storage) manualBinding(ctx context.Context, fp string) (string, error) {
	if !regroupEnabled() {
		return "", nil
	}
	key := "igpt:regroup:binding:" + fp
	target, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	inc, err := s.GetIncident(target)
	if err != nil && err != redis.Nil {
		return "", err
	}
	if inc == nil || inc.Status == "resolved" || !inc.Fingerprints[fp] {
		return "", s.compositionWrite(func(p redis.Pipeliner) { p.Del(ctx, key) })
	}
	// Adopt bounded retention for legacy bindings on first use, without
	// extending a pin forever on every repeated delivery.
	err = s.compositionWrite(func(p redis.Pipeliner) { p.ExpireNX(ctx, key, manualBindingTTL) })
	return target, err
}
