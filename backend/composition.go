package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func alertNamespace(a *Alert) string {
	if a.Namespace != "" {
		return a.Namespace
	}
	return a.Labels["namespace"]
}

// All diagnostic coordinates come from one current member, never a mix of
// namespaces, workloads, pods or nodes from different members.
func representativeAlert(inc *Incident) *Alert {
	for _, a := range inc.Alerts {
		if a != nil && alertNamespace(a) != "" && a.Labels["pod"] != "" {
			return a
		}
	}
	for _, a := range inc.Alerts {
		if a != nil && alertNamespace(a) != "" {
			return a
		}
	}
	for _, a := range inc.Alerts {
		if a != nil && a.Labels["pod"] != "" {
			return a
		}
	}
	for _, a := range inc.Alerts {
		if a != nil {
			return a
		}
	}
	return nil
}

var clearManualBinding = redis.NewScript(`
if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end
return 0`)

// A resolved delivery updates only the delivered fingerprints. Close each
// affected incident only after all of its members have resolved; unrelated
// firing members must not be marked resolved by a partial batch.
func (s *Store) resolveDeliveredAlerts(alerts []EnrichedAlert) *Incident {
	affected := map[string]*Incident{}
	order := []string{}
	for _, delivered := range alerts {
		if delivered.Status != "resolved" {
			continue
		}
		inc := s.FindIncidentForGroup([]string{delivered.Fingerprint})
		if inc == nil {
			continue
		}
		if current, ok := affected[inc.ID]; ok {
			inc = current
		} else {
			affected[inc.ID] = inc
			order = append(order, inc.ID)
		}
		for _, member := range inc.Alerts {
			if member.Fingerprint != delivered.Fingerprint {
				continue
			}
			member.Status = "resolved"
			member.EndsAt = delivered.EndsAt
			s.PutAlert(member)
		}
	}
	var first *Incident
	for _, id := range order {
		inc := affected[id]
		allResolved := len(inc.Alerts) > 0
		for _, a := range inc.Alerts {
			if a.Status != "resolved" {
				allResolved = false
				break
			}
		}
		now := time.Now()
		inc.UpdatedAt = now
		if allResolved {
			inc.Status = "resolved"
			duration := int(now.Sub(inc.CreatedAt).Seconds())
			inc.DurationSec = &duration
		}
		s.Put(inc)
		recordIngestLifecycle(inc)
		if allResolved && s.disk != nil && s.disk.writeErr == nil {
			ctx, cancel := context.WithTimeout(s.disk.ctx, time.Second)
			for _, a := range inc.Alerts {
				_ = clearManualBinding.Run(ctx, s.disk.rdb, []string{"igpt:regroup:binding:" + a.Fingerprint}, inc.ID).Err()
			}
			cancel()
		}
		if first == nil {
			first = inc
		}
	}
	return first
}
