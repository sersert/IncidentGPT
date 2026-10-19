package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/http"
	"time"
)

type Lifecycle struct {
	Version           int        `json:"schema_version"`
	IncidentID        string     `json:"incident_id"`
	Revision          int        `json:"revision"`
	FirstSymptom      *time.Time `json:"first_known_symptom,omitempty"`
	AlertReceipt      *time.Time `json:"alert_received_at,omitempty"`
	Acknowledged      *time.Time `json:"acknowledged_at,omitempty"`
	AnalysisStarted   *time.Time `json:"analysis_started_at,omitempty"`
	AnalysisCompleted *time.Time `json:"analysis_completed_at,omitempty"`
	Recovery          *time.Time `json:"verified_recovery_at,omitempty"`
	Closed            *time.Time `json:"administratively_closed_at,omitempty"`
	RecoveryEvidence  []string   `json:"recovery_evidence_ids,omitempty"`
	RecoveryActor     string     `json:"recovery_actor,omitempty"`
	LastActor         string     `json:"last_actor,omitempty"`
	RecordedAt        *time.Time `json:"recorded_at,omitempty"`
}

func deriveLifecycle(inc *Incident) Lifecycle {
	l := Lifecycle{Version: 1, IncidentID: inc.ID}
	for _, a := range inc.Alerts {
		if !a.ReceivedAt.IsZero() && (l.AlertReceipt == nil || a.ReceivedAt.Before(*l.AlertReceipt)) {
			t := a.ReceivedAt.UTC()
			l.AlertReceipt = &t
		}
		if t, err := time.Parse(time.RFC3339Nano, a.StartsAt); err == nil && !t.After(time.Now().UTC()) && (l.FirstSymptom == nil || t.Before(*l.FirstSymptom)) {
			t = t.UTC()
			l.FirstSymptom = &t
		}
	}
	return l
}
func readLifecycle(ctx context.Context, inc *Incident) (Lifecycle, error) {
	raw, err := store.disk.rdb.Get(ctx, "igpt:lifecycle:"+inc.ID).Bytes()
	if err == redis.Nil {
		return deriveLifecycle(inc), nil
	}
	var l Lifecycle
	if err != nil || json.Unmarshal(raw, &l) != nil {
		return l, fmt.Errorf("lifecycle unavailable")
	}
	return l, nil
}
func mutateLifecycle(ctx context.Context, inc *Incident, update func(*Lifecycle) error) error {
	key := "igpt:lifecycle:" + inc.ID
	for attempt := 0; attempt < 3; attempt++ {
		err := store.disk.rdb.Watch(ctx, func(tx *redis.Tx) error {
			raw, err := tx.Get(ctx, key).Bytes()
			var l Lifecycle
			if err == redis.Nil {
				l = deriveLifecycle(inc)
			} else if err != nil {
				return err
			} else if json.Unmarshal(raw, &l) != nil {
				return fmt.Errorf("invalid lifecycle")
			}
			if err := update(&l); err != nil {
				return err
			}
			l.Revision++
			data, _ := json.Marshal(l)
			_, err = tx.TxPipelined(ctx, func(p redis.Pipeliner) error {
				p.Set(ctx, key, data, 0)
				p.Set(ctx, fmt.Sprintf("igpt:lifecycle:history:%s:%d", inc.ID, l.Revision), data, 0)
				return nil
			})
			return err
		}, key)
		if err != redis.TxFailedErr {
			return err
		}
	}
	return fmt.Errorf("lifecycle changed; retry")
}
func recordIngestLifecycle(inc *Incident) {
	if inc == nil || store.disk == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = mutateLifecycle(ctx, inc, func(l *Lifecycle) error {
		observed := deriveLifecycle(inc)
		if observed.FirstSymptom != nil && (l.FirstSymptom == nil || observed.FirstSymptom.Before(*l.FirstSymptom)) {
			l.FirstSymptom = observed.FirstSymptom
		}
		if observed.AlertReceipt != nil && (l.AlertReceipt == nil || observed.AlertReceipt.Before(*l.AlertReceipt)) {
			l.AlertReceipt = observed.AlertReceipt
		}
		return nil
	})
}
func recordHumanLifecycle(ctx context.Context, inc *Incident, status, actor string) error {
	now := time.Now().UTC()
	return mutateLifecycle(ctx, inc, func(l *Lifecycle) error {
		if status == "acknowledged" && l.Acknowledged == nil {
			l.Acknowledged = &now
		}
		if status == "resolved" {
			l.Closed = &now
		}
		l.LastActor = actor
		l.RecordedAt = &now
		return nil
	})
}
func handleLifecycle(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	if store.disk == nil {
		http.Error(w, "lifecycle storage unavailable", 503)
		return
	}
	inc := store.Get(r.PathValue("id"))
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if r.Method == "GET" {
		l, err := readLifecycle(ctx, inc)
		if err != nil {
			http.Error(w, "lifecycle unavailable", 503)
			return
		}
		latest, err := store.disk.rdb.Get(ctx, analysisLatest(inc.ID)).Result()
		if err != nil && err != redis.Nil {
			http.Error(w, "analysis lifecycle unavailable", 503)
			return
		}
		if latest != "" {
			raw, err := store.disk.rdb.Get(ctx, analysisKey(latest)).Bytes()
			var a AnalysisVersion
			if err != nil || json.Unmarshal(raw, &a) != nil {
				http.Error(w, "analysis lifecycle unavailable", 503)
				return
			}
			if !a.StartedAt.IsZero() {
				l.AnalysisStarted = &a.StartedAt
			}
			if !a.CompletedAt.IsZero() {
				l.AnalysisCompleted = &a.CompletedAt
			}
		}
		writeJSON(w, 200, l)
		return
	}
	var req struct {
		At       time.Time `json:"verified_recovery_at"`
		Evidence []string  `json:"evidence_ids"`
		Revision int       `json:"revision"`
	}
	if decodeChange(r, &req) != nil || req.At.IsZero() || req.At.After(time.Now()) || len(req.Evidence) == 0 || len(req.Evidence) > 20 {
		http.Error(w, "known recovery time and verification evidence required", 400)
		return
	}
	for _, id := range req.Evidence {
		raw, err := store.disk.rdb.Get(ctx, "igpt:evidence:"+id).Bytes()
		var e Evidence
		if err != nil || json.Unmarshal(raw, &e) != nil || e.IncidentID != inc.ID || e.Availability != "available" {
			http.Error(w, "available incident verification evidence required", 400)
			return
		}
	}
	actor, _, _ := r.BasicAuth()
	err := mutateLifecycle(ctx, inc, func(l *Lifecycle) error {
		if l.Revision != req.Revision {
			return redis.TxFailedErr
		}
		if l.FirstSymptom != nil && req.At.Before(*l.FirstSymptom) {
			return fmt.Errorf("recovery precedes symptom")
		}
		at := req.At.UTC()
		now := time.Now().UTC()
		l.Recovery = &at
		l.RecoveryEvidence = req.Evidence
		l.RecoveryActor = actor
		l.LastActor = actor
		l.RecordedAt = &now
		return nil
	})
	if err != nil {
		http.Error(w, "lifecycle changed or verification invalid; reload", 409)
		return
	}
	l, _ := readLifecycle(ctx, inc)
	writeJSON(w, 200, l)
}
func elapsed(start, end *time.Time) *float64 {
	if start == nil || end == nil || start.IsZero() || end.IsZero() || end.Before(*start) {
		return nil
	}
	seconds := end.Sub(*start).Seconds()
	return &seconds
}
