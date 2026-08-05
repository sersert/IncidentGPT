package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu       sync.Mutex
	counters map[string]float64
}

func New() *Metrics {
	return &Metrics{counters: map[string]float64{}}
}

func (m *Metrics) Inc(name string, labels map[string]string) {
	m.Add(name, labels, 1)
}

func (m *Metrics) Add(name string, labels map[string]string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[key(name, labels)] += value
}

func (m *Metrics) ObserveDuration(source, destination, status string, start time.Time) {
	m.Add("incidentgpt_sanitizer_request_duration_seconds_sum", map[string]string{"source": source, "destination": destination, "status": status}, time.Since(start).Seconds())
	m.Inc("incidentgpt_sanitizer_request_duration_seconds_count", map[string]string{"source": source, "destination": destination, "status": status})
}

func (m *Metrics) Handler(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	keys := make([]string, 0, len(m.counters))
	for k := range m.counters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "%s %g\n", k, m.counters[k])
	}
}

func key(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, sanitizeLabel(v)))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%s{%s}", name, strings.Join(parts, ","))
}

func sanitizeLabel(v string) string {
	switch v {
	case "enricher", "ai-worker", "llm", "telegram", "logs", "webhook", "debug", "ok", "error", "denied", "sanitized":
		return v
	default:
		if strings.HasPrefix(v, "SANITIZER_") || strings.HasSuffix(v, "_ERROR") || strings.HasPrefix(v, "INVALID_") || strings.HasPrefix(v, "PAYLOAD_") || strings.HasPrefix(v, "POLICY_") || strings.HasPrefix(v, "CONFIGURATION_") || strings.HasPrefix(v, "MAX_") || strings.HasPrefix(v, "UNSUPPORTED_") || strings.HasPrefix(v, "INTERNAL_") {
			return v
		}
		return "other"
	}
}
