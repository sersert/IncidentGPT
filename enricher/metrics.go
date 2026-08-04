package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
)

type metricsRegistry struct {
	mu         sync.Mutex
	counters   map[string]float64
	histograms map[string]*histogramMetric
}

type histogramMetric struct {
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

var appMetrics = newMetricsRegistry()

func newMetricsRegistry() *metricsRegistry {
	m := &metricsRegistry{
		counters:   make(map[string]float64),
		histograms: make(map[string]*histogramMetric),
	}
	for _, name := range []string{
		"incidentgpt_alerts_received_total",
		"incidentgpt_alerts_enriched_total",
		"incidentgpt_raw_sent_total",
		"incidentgpt_groups_sent_total",
		"incidentgpt_telegram_errors_total",
		"incidentgpt_openrouter_errors_total",
		"incidentgpt_redis_fallback_total",
	} {
		m.counters[name] = 0
	}
	m.histograms["incidentgpt_enrichment_duration_seconds"] = newHistogram()
	m.histograms["incidentgpt_llm_duration_seconds"] = newHistogram()
	return m
}

func newHistogram() *histogramMetric {
	buckets := []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300}
	return &histogramMetric{
		buckets: buckets,
		counts:  make([]uint64, len(buckets)),
	}
}

func (m *metricsRegistry) inc(name string) {
	m.add(name, 1)
}

func (m *metricsRegistry) add(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}

func (m *metricsRegistry) observe(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.histograms[name]
	if !ok {
		h = newHistogram()
		m.histograms[name] = h
	}
	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
	h.sum += value
	h.count++
}

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(appMetrics.render()))
}

func (m *metricsRegistry) render() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out string
	counterNames := make([]string, 0, len(m.counters))
	for name := range m.counters {
		counterNames = append(counterNames, name)
	}
	sort.Strings(counterNames)
	for _, name := range counterNames {
		out += fmt.Sprintf("# TYPE %s counter\n%s %s\n", name, name, formatMetricFloat(m.counters[name]))
	}

	histNames := make([]string, 0, len(m.histograms))
	for name := range m.histograms {
		histNames = append(histNames, name)
	}
	sort.Strings(histNames)
	for _, name := range histNames {
		h := m.histograms[name]
		out += fmt.Sprintf("# TYPE %s histogram\n", name)
		for i, bucket := range h.buckets {
			out += fmt.Sprintf("%s_bucket{le=%q} %d\n", name, formatMetricFloat(bucket), h.counts[i])
		}
		out += fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", name, h.count)
		out += fmt.Sprintf("%s_sum %s\n", name, formatMetricFloat(h.sum))
		out += fmt.Sprintf("%s_count %d\n", name, h.count)
	}
	return out
}

func formatMetricFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
