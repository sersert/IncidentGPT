package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Прогнозы: какие ресурсы упрутся в предел, если тренд сохранится.
//
// Считает Prometheus, а не мы: у него для этого есть deriv() по историческому
// окну. Раньше эндпоинт отдавал пустой массив, и раздел выглядел так, будто
// «всё в норме», хотя на деле просто ничего не считалось.

const (
	// Окно, по которому оцениваем скорость изменения. Меньше — шумит на всплесках,
	// больше — не замечает свежий тренд.
	trendWindow = "6h"
	// Показываем ресурс, если он упрётся в предел в ближайшие трое суток.
	horizonHours = 72.0
	// Либо если он уже занят выше этого порога — тренда может и не быть,
	// а место всё равно почти кончилось.
	usageThreshold = 80.0
)

type Prediction struct {
	ID             string    `json:"id"`
	Resource       string    `json:"resource"`
	ResourceType   string    `json:"resource_type"`
	CurrentValue   float64   `json:"current_value"`
	CurrentUnit    string    `json:"current_unit"`
	PredictedValue float64   `json:"predicted_value"`
	TimeToCritical string    `json:"time_to_critical"`
	GrowthRate     string    `json:"growth_rate"`
	Severity       string    `json:"severity"`
	Recommendation string    `json:"recommendation"`
	Sparkline      []float64 `json:"sparkline"`
	Node           string    `json:"node"`
	Namespace      string    `json:"namespace"`
}

type promSample struct {
	labels map[string]string
	value  float64
}

// queryProm выполняет мгновенный запрос к Prometheus.
func queryProm(promURL, query string) ([]promSample, error) {
	if promURL == "" {
		return nil, fmt.Errorf("адрес Prometheus не задан")
	}
	endpoint := strings.TrimRight(promURL, "/") + "/api/v1/query?query=" + url.QueryEscape(query)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("нет связи с Prometheus: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Prometheus ответил %d", resp.StatusCode)
	}

	var out struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("не разобрать ответ Prometheus: %w", err)
	}

	samples := make([]promSample, 0, len(out.Data.Result))
	for _, r := range out.Data.Result {
		if len(r.Value) < 2 {
			continue
		}
		raw, _ := r.Value[1].(string)
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		samples = append(samples, promSample{labels: r.Metric, value: v})
	}
	return samples, nil
}

func promURLFromIntegrations() string {
	integrations.mu.RLock()
	defer integrations.mu.RUnlock()
	if it, ok := integrations.items["metrics"]; ok {
		return it.Config["url"]
	}
	return ""
}

// GET /api/v1/predictions
func handlePredictions(w http.ResponseWriter, _ *http.Request) {
	promURL := promURLFromIntegrations()
	preds := collectPredictions(promURL)
	// Ближайшие к пределу — первыми.
	sort.Slice(preds, func(i, j int) bool { return preds[i].CurrentValue > preds[j].CurrentValue })
	writeJSON(w, 200, preds)
}

func collectPredictions(promURL string) []Prediction {
	preds := []Prediction{}
	preds = append(preds, diskPredictions(promURL)...)
	preds = append(preds, memoryPredictions(promURL)...)
	return preds
}

// diskPredictions: занятость файловых систем и часы до заполнения по тренду.
func diskPredictions(promURL string) []Prediction {
	const fsFilter = `fstype!~"tmpfs|ramfs|overlay|squashfs|fuse.*",mountpoint!~"/(run|boot|var/lib/kubelet/pods).*"`

	usage, err := queryProm(promURL, fmt.Sprintf(
		`100 - (node_filesystem_avail_bytes{%s} / node_filesystem_size_bytes{%s} * 100)`, fsFilter, fsFilter))
	if err != nil {
		return nil
	}

	// Часы до нуля свободного места. deriv отрицателен, когда место убывает,
	// поэтому берём со знаком минус; растущие тома дадут отрицательный ответ
	// и отсеются ниже.
	hours, _ := queryProm(promURL, fmt.Sprintf(
		`node_filesystem_avail_bytes{%s} / -deriv(node_filesystem_avail_bytes{%s}[%s]) / 3600`,
		fsFilter, fsFilter, trendWindow))

	// Скорость в байтах в час — для человекочитаемой подписи.
	rate, _ := queryProm(promURL, fmt.Sprintf(
		`-deriv(node_filesystem_avail_bytes{%s}[%s]) * 3600`, fsFilter, trendWindow))

	key := func(l map[string]string) string { return l["instance"] + "|" + l["mountpoint"] }
	hoursBy := map[string]float64{}
	for _, s := range hours {
		hoursBy[key(s.labels)] = s.value
	}
	rateBy := map[string]float64{}
	for _, s := range rate {
		rateBy[key(s.labels)] = s.value
	}

	out := []Prediction{}
	for _, s := range usage {
		k := key(s.labels)
		h := hoursBy[k]
		fillingSoon := h > 0 && h < horizonHours
		if !fillingSoon && s.value < usageThreshold {
			continue
		}

		node := s.labels["instance"]
		mount := s.labels["mountpoint"]
		p := Prediction{
			ID:             "disk:" + k,
			Resource:       fmt.Sprintf("%s %s", node, mount),
			ResourceType:   "disk",
			CurrentValue:   round1(s.value),
			CurrentUnit:    "%",
			PredictedValue: 100,
			Node:           node,
			Severity:       "warning",
			GrowthRate:     humanRate(rateBy[k]),
			TimeToCritical: humanHours(h),
			Recommendation: fmt.Sprintf("Освободить место на %s или расширить том", mount),
		}
		if s.value >= 90 || (h > 0 && h < 12) {
			p.Severity = "critical"
		}
		out = append(out, p)
	}
	return out
}

// memoryPredictions: ноды, у которых мало свободной памяти.
func memoryPredictions(promURL string) []Prediction {
	usage, err := queryProm(promURL,
		`100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)`)
	if err != nil {
		return nil
	}

	out := []Prediction{}
	for _, s := range usage {
		if s.value < 85 {
			continue
		}
		node := s.labels["instance"]
		p := Prediction{
			ID:             "memory:" + node,
			Resource:       node + " память",
			ResourceType:   "memory",
			CurrentValue:   round1(s.value),
			CurrentUnit:    "%",
			PredictedValue: 100,
			Node:           node,
			Severity:       "warning",
			GrowthRate:     "—",
			TimeToCritical: "—",
			Recommendation: "Проверить, какие поды заняли память, и уточнить их лимиты",
		}
		if s.value >= 95 {
			p.Severity = "critical"
		}
		out = append(out, p)
	}
	return out
}

// NodeUsage — состояние одной ноды.
type NodeUsage struct {
	Node   string  `json:"node"`
	CPU    float64 `json:"cpu"`
	Memory float64 `json:"memory"`
	Disk   float64 `json:"disk"`
}

// GET /api/v1/predictions/summary
//
// Отдаём разбивку по нодам, а не средние по кластеру: среднее прячет проблему.
// Одна нода на 95% и три пустые дают «30%, всё в норме».
func handlePredictionsSummary(w http.ResponseWriter, _ *http.Request) {
	promURL := promURLFromIntegrations()
	const fsFilter = `fstype!~"tmpfs|ramfs|overlay|squashfs|fuse.*",mountpoint!~"/(run|boot|var/lib/kubelet/pods).*"`

	byNode := map[string]*NodeUsage{}
	get := func(node string) *NodeUsage {
		if _, ok := byNode[node]; !ok {
			byNode[node] = &NodeUsage{Node: node}
		}
		return byNode[node]
	}

	if s, err := queryProm(promURL, `100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[10m])) * 100)`); err == nil {
		for _, x := range s {
			get(x.labels["instance"]).CPU = round1(x.value)
		}
	}
	if s, err := queryProm(promURL, `100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)`); err == nil {
		for _, x := range s {
			get(x.labels["instance"]).Memory = round1(x.value)
		}
	}
	// По диску берём самую заполненную файловую систему ноды, а не среднюю:
	// кончится место именно на ней.
	if s, err := queryProm(promURL, fmt.Sprintf(
		`max by(instance) (100 - (node_filesystem_avail_bytes{%s} / node_filesystem_size_bytes{%s} * 100))`,
		fsFilter, fsFilter)); err == nil {
		for _, x := range s {
			get(x.labels["instance"]).Disk = round1(x.value)
		}
	}

	nodes := make([]NodeUsage, 0, len(byNode))
	for _, n := range byNode {
		nodes = append(nodes, *n)
	}
	// Самые нагруженные — первыми.
	sort.Slice(nodes, func(i, j int) bool {
		return math.Max(nodes[i].Disk, nodes[i].Memory) > math.Max(nodes[j].Disk, nodes[j].Memory)
	})

	preds := collectPredictions(promURL)
	warning, critical := 0, 0
	for _, p := range preds {
		if p.Severity == "critical" {
			critical++
		} else {
			warning++
		}
	}

	// «Под присмотром» — файловые системы и ноды, за которыми следим.
	watched := 0
	if s, err := queryProm(promURL, fmt.Sprintf(
		`count(node_filesystem_avail_bytes{%s}) + count(node_memory_MemTotal_bytes)`, fsFilter)); err == nil && len(s) > 0 {
		watched = int(s[0].value)
	}
	healthy := watched - len(preds)
	if healthy < 0 {
		healthy = 0
	}

	writeJSON(w, 200, map[string]any{
		"healthy_count":  healthy,
		"watched_count":  watched,
		"warning_count":  warning,
		"critical_count": critical,
		"nodes":          nodes,
		"network_status": "stable",
	})
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func humanHours(h float64) string {
	if h <= 0 || math.IsInf(h, 0) || math.IsNaN(h) {
		return "—"
	}
	if h < 1 {
		return fmt.Sprintf("%d мин", int(h*60))
	}
	if h < 48 {
		return fmt.Sprintf("%d ч", int(h))
	}
	return fmt.Sprintf("%d д", int(h/24))
}

func humanRate(bytesPerHour float64) string {
	if bytesPerHour <= 0 || math.IsNaN(bytesPerHour) || math.IsInf(bytesPerHour, 0) {
		return "—"
	}
	const mb = 1024 * 1024
	if bytesPerHour < mb {
		return fmt.Sprintf("+%.0f КБ/ч", bytesPerHour/1024)
	}
	if bytesPerHour < 1024*mb {
		return fmt.Sprintf("+%.1f МБ/ч", bytesPerHour/mb)
	}
	return fmt.Sprintf("+%.1f ГБ/ч", bytesPerHour/(1024*mb))
}
