package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/http"
	"sort"
	"time"
)

type metricIncident struct {
	Incident  *Incident
	Lifecycle Lifecycle
}

func metricSnapshot(r *http.Request) ([]metricIncident, bool, error) {
	if store.disk == nil {
		return nil, false, fmt.Errorf("persistent lifecycle metrics unavailable")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	days := 30
	switch r.URL.Query().Get("period") {
	case "7d":
		days = 7
	case "90d":
		days = 90
	case "", "30d":
	default:
		return nil, false, fmt.Errorf("unsupported period")
	}
	ids, err := store.disk.rdb.ZRevRangeByScore(ctx, keyIncidentIdx, &redis.ZRangeBy{Min: fmt.Sprint(time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()), Max: "+inf", Offset: 0, Count: 1001}).Result()
	if err != nil {
		return nil, false, err
	}
	bounded := len(ids) > 1000
	if bounded {
		ids = ids[:1000]
	}
	if len(ids) == 0 {
		return []metricIncident{}, false, nil
	}
	keys := []string{}
	for _, id := range ids {
		keys = append(keys, keyIncident+id, "igpt:lifecycle:"+id, analysisLatest(id))
	}
	values, err := store.disk.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, false, err
	}
	out := []metricIncident{}
	analysisKeys := []string{}
	positions := []int{}
	for i, id := range ids {
		if values[i*3] == nil {
			continue
		}
		raw, ok := values[i*3].(string)
		if !ok {
			return nil, false, fmt.Errorf("incident metrics invalid")
		}
		var inc Incident
		if json.Unmarshal([]byte(raw), &inc) != nil {
			return nil, false, fmt.Errorf("incident metrics invalid")
		}
		l := deriveLifecycle(&inc)
		if value := values[i*3+1]; value != nil {
			if json.Unmarshal([]byte(value.(string)), &l) != nil {
				return nil, false, fmt.Errorf("lifecycle metrics invalid")
			}
		}
		if value := values[i*3+2]; value != nil {
			analysisKeys = append(analysisKeys, analysisKey(value.(string)))
			positions = append(positions, len(out))
		}
		if l.IncidentID != id {
			return nil, false, fmt.Errorf("lifecycle identity invalid")
		}
		out = append(out, metricIncident{Incident: &inc, Lifecycle: l})
	}
	if len(analysisKeys) > 0 {
		analyses, err := store.disk.rdb.MGet(ctx, analysisKeys...).Result()
		if err != nil {
			return nil, false, err
		}
		for i, value := range analyses {
			if value == nil {
				return nil, false, fmt.Errorf("analysis metrics missing")
			}
			var a AnalysisVersion
			if json.Unmarshal([]byte(value.(string)), &a) != nil || a.IncidentID != out[positions[i]].Incident.ID {
				return nil, false, fmt.Errorf("analysis metrics invalid")
			}
			l := &out[positions[i]].Lifecycle
			if !a.StartedAt.IsZero() {
				l.AnalysisStarted = &a.StartedAt
			}
			if !a.CompletedAt.IsZero() {
				l.AnalysisCompleted = &a.CompletedAt
			}
		}
	}
	return out, bounded, nil
}
func metricMean(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	v := avg(values)
	return &v
}
func metricValues(records []metricIncident, kind string) []float64 {
	values := []float64{}
	for _, record := range records {
		l := record.Lifecycle
		var d *float64
		switch kind {
		case "analysis":
			d = elapsed(l.AnalysisStarted, l.AnalysisCompleted)
		case "ack":
			d = elapsed(l.AlertReceipt, l.Acknowledged)
		case "recovery":
			d = elapsed(l.FirstSymptom, l.Recovery)
		case "closure":
			d = elapsed(l.AlertReceipt, l.Closed)
		}
		if d != nil {
			values = append(values, *d)
		}
	}
	return values
}
func metricSummary(records []metricIncident, kind string) map[string]any {
	values := metricValues(records, kind)
	sort.Float64s(values)
	var median, p90 *float64
	if len(values) > 0 {
		m := values[len(values)/2]
		if len(values)%2 == 0 {
			m = (values[len(values)/2-1] + m) / 2
		}
		median = &m
		p := values[(9*len(values)+9)/10-1]
		p90 = &p
	}
	return map[string]any{"mean_seconds": metricMean(values), "median_seconds": median, "p90_seconds": p90, "samples": len(values), "missing": len(records) - len(values)}
}
func handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	records, bounded, err := metricSnapshot(r)
	if err != nil {
		http.Error(w, "dashboard metrics unavailable", 503)
		return
	}
	active, critical := 0, 0
	for _, v := range records {
		if v.Incident.Status == "active" || v.Incident.Status == "acknowledged" {
			active++
			if v.Incident.Severity == "critical" {
				critical++
			}
		}
	}
	_, today := store.AlertStats()
	writeJSON(w, 200, map[string]any{"active_incidents": active, "active_critical": critical, "mttr_reduction_pct": nil, "noise_reduction_pct": nil, "avg_analysis_time_sec": metricMean(metricValues(records, "analysis")), "alerts_today": today, "alerts_sparkline": []int{}, "bounded": bounded, "sample_incidents": len(records), "metrics_note": "bounded last 30 days; unknown metrics are null; no causal AI benefit estimate"})
}
func handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	records, bounded, err := metricSnapshot(r)
	if err != nil {
		http.Error(w, "analytics unavailable", 503)
		return
	}
	total, _ := store.AlertStats()
	writeJSON(w, 200, map[string]any{"mttr_before_min": nil, "mttr_after_min": nil, "mttr_reduction_pct": nil, "mtta_before_min": nil, "mtta_after_min": nil, "mtta_reduction_pct": nil, "noise_before": total, "noise_after": len(records), "noise_reduction_pct": nil, "time_saved_hours": nil, "analysis": metricSummary(records, "analysis"), "acknowledgement": metricSummary(records, "ack"), "verified_recovery": metricSummary(records, "recovery"), "administrative_closure": metricSummary(records, "closure"), "bounded": bounded, "sample_incidents": len(records), "count_note": "alert counter is all-time; incident sample follows requested period; no reduction comparison is computed", "metrics_note": "observational lifecycle durations; missing events excluded; no inferred recovery or causal AI benefit"})
}
func handleMTTRTrend(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	records, bounded, err := metricSnapshot(r)
	if err != nil {
		http.Error(w, "trend unavailable", 503)
		return
	}
	byDay := map[string][]metricIncident{}
	for _, record := range records {
		day := record.Incident.CreatedAt.UTC().Format("2006-01-02")
		byDay[day] = append(byDay[day], record)
	}
	days := []string{}
	for day := range byDay {
		days = append(days, day)
	}
	sort.Strings(days)
	data := []map[string]any{}
	for _, day := range days {
		p := map[string]any{"date": day}
		for kind, field := range map[string]string{"analysis": "analysis_sec", "closure": "close_min", "recovery": "recovery_min", "ack": "ack_min"} {
			values := metricValues(byDay[day], kind)
			if mean := metricMean(values); mean != nil {
				v := *mean
				if kind != "analysis" {
					v /= 60
				}
				p[field] = v
			}
			p[field+"_samples"] = len(values)
		}
		data = append(data, p)
	}
	writeJSON(w, 200, map[string]any{"data": data, "bounded": bounded, "cohort": "incident creation day UTC; observational, no AI attribution"})
}
func handleAnalyticsQuality(w http.ResponseWriter, r *http.Request) {
	if !changeReadAuth(w, r) {
		return
	}
	records, bounded, err := metricSnapshot(r)
	if err != nil {
		http.Error(w, "quality unavailable", 503)
		return
	}
	total, helpful := 0, 0
	for _, record := range records {
		if record.Incident.Feedback != nil {
			total++
			if record.Incident.Feedback.Helpful {
				helpful++
			}
		}
	}
	var yes, no *float64
	if total > 0 {
		a := float64(helpful) * 100 / float64(total)
		b := 100 - a
		yes = &a
		no = &b
	}
	values := metricValues(records, "analysis")
	writeJSON(w, 200, map[string]any{"total_analyses": len(values), "feedback_samples": total, "helpful_pct": yes, "not_helpful_pct": no, "edited_pct": nil, "avg_confidence": nil, "avg_response_time_sec": metricMean(values), "bounded": bounded, "metrics_note": "structured completed analysis processing time only; feedback is not cause confirmation"})
}
