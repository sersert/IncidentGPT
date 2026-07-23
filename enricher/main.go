package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"text/template"
)

//
// ====== МОДЕЛИ ВХОДА / ВЫХОДА ======
//

// AMAlert — алерт от Alertmanager V2.
type AMAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// AMWebhook — обёртка Alertmanager webhook, когда он шлёт не []Alert, а объект.
type AMWebhook struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alerts            []AMAlert         `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
}

// promRangeResponse — ответ Prometheus /api/v1/query_range.
type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"` // [ [ ts, "val" ], ... ]
		} `json:"result"`
	} `json:"data"`
}

// MetricStats — агрегаты по метрике.
type MetricStats struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
	Last  float64 `json:"last"`
	Count int     `json:"count"`
	Trend string  `json:"trend"` // "rising", "falling", "spike", "stable", "insufficient_data"
}

// EnrichedAlert — структура, для backend.
type EnrichedAlert struct {
	Source       string            `json:"source"`
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"starts_at"`
	EndsAt       time.Time         `json:"ends_at"`
	Severity     string            `json:"severity,omitempty"`
	Fingerprint  string            `json:"fingerprint,omitempty"`
	GeneratorURL string            `json:"generator_url,omitempty"`

	// Обогащение Prometheus (ключи: primary_*, node_*, svc_*, cluster_*, ext_*).
	PromSample map[string]string `json:"prom_sample,omitempty"`

	// Доп. контекст по kubernetes / внешним штукам.
	K8sContext map[string]string `json:"k8s_context,omitempty"`

	// Ссылки на Grafana / логи / runbook.
	GrafanaLinks []string `json:"grafana_links,omitempty"`

	// Заметки по тому, как собирали метрики.
	ExtraNotes []string `json:"extra_notes,omitempty"`

	// Простые текстовые подсказки по инциденту.
	IncidentHints []string `json:"incident_hints,omitempty"`

	// Тех. инфа.
	Enriched      bool   `json:"enriched,omitempty"`
	EnrichedAt    string `json:"enriched_at,omitempty"`
	EnricherVer   string `json:"enricher_version,omitempty"`
	ClusterName   string `json:"cluster_name,omitempty"`
	Environment   string `json:"environment,omitempty"`
	MetricsWindow string `json:"metrics_window,omitempty"`
	MetricsAnchor string `json:"metrics_anchor,omitempty"`
	MetricsBefore string `json:"metrics_before,omitempty"`
	MetricsAfter  string `json:"metrics_after,omitempty"`
	Owner         string `json:"owner,omitempty"`

	// primary metric
	PrimaryMetric  string `json:"primary_metric,omitempty"`
	PrimarySummary string `json:"primary_summary,omitempty"`
}

//
// ====== КОНФИГ ПРИЛОЖЕНИЯ / МЕТРИК ======
//

// Config — конфиг через ENV (для Helm).
type Config struct {
	PromURL         string
	BackendURL      string
	GrafanaURL      string
	LogsBaseURL     string
	RunbookBaseURL  string
	ListenAddr      string
	EnricherVersion string
	ClusterName     string
	Environment     string

	RangeBefore time.Duration
	RangeAfter  time.Duration

	// Лёгкая корреляция через Redis
	RedisAddr       string
	RedisPassword   string
	GroupBackendURL string
	RawBackendURL   string
	CorrWindow      time.Duration
	CorrWindowRaw   string
	CorrSettle      time.Duration
	RawDedupTTL     time.Duration

	EnableClusterContext  bool
	EnableNodeContext     bool
	EnableWorkloadContext bool
	EnableExternalContext bool
	EnableK8sContext      bool
}

// MetricsConfig — то, что лежит в metrics.yaml (ConfigMap).
type MetricsConfig struct {
	Cluster  []MetricDef `yaml:"cluster"`
	Node     []MetricDef `yaml:"node"`
	Workload []MetricDef `yaml:"workload"`
	External []MetricDef `yaml:"external"`
}

// MetricDef — одна метрика в конфиге.
type MetricDef struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"`
	Expr string `yaml:"expr"`
}

// metricTemplateContext — контекст для шаблонов expr ({{ .Cluster }}, {{ .Namespace }} и т.п.).
type metricTemplateContext struct {
	Cluster   string
	Namespace string
	Service   string
	Node      string
	Instance  string
}

// глобалы
var (
	appCfg     Config
	httpClient = &http.Client{Timeout: 35 * time.Second}
	metricsCfg MetricsConfig
)

//
// ====== ВСПОМОГАТЕЛЬНЫЕ ======
//

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("WARN: invalid duration in %s=%q: %v, using default %s", key, raw, err, def)
		return def
	}
	return d
}

func envBool(key string, def bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return def
	}
	switch raw {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		log.Printf("WARN: invalid bool in %s=%q, using default %v", key, raw, def)
		return def
	}
}

func stableFingerprint(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}

	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func ownerFromLabels(lbl map[string]string) string {
	if o := lbl["owner"]; o != "" {
		return o
	}
	if t := lbl["team"]; t != "" {
		return t
	}
	if ns := lbl["namespace"]; ns != "" {
		return ns
	}
	return ""
}

func cloneStrMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func parsePromValue(v interface{}) (float64, bool) {
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func calcStats(values []float64) MetricStats {
	s := MetricStats{}
	if len(values) == 0 {
		return s
	}
	s.Min = values[0]
	s.Max = values[0]
	var sum float64
	for _, v := range values {
		if v < s.Min {
			s.Min = v
		}
		if v > s.Max {
			s.Max = v
		}
		sum += v
	}
	s.Avg = sum / float64(len(values))
	s.Last = values[len(values)-1]
	s.Count = len(values)
	s.Trend = calcTrend(values)
	return s
}

// calcTrend определяет направление метрики: rising, falling, spike, stable.
func calcTrend(values []float64) string {
	if len(values) < 4 {
		return "insufficient_data"
	}

	mid := len(values) / 2
	var firstSum, secondSum float64
	for _, v := range values[:mid] {
		firstSum += v
	}
	for _, v := range values[mid:] {
		secondSum += v
	}
	firstAvg := firstSum / float64(mid)
	secondAvg := secondSum / float64(len(values)-mid)

	last := values[len(values)-1]
	avg := (firstSum + secondSum) / float64(len(values))

	// Spike: последнее значение > 2x от среднего
	if avg > 0 && last > avg*2 {
		return "spike"
	}

	// Защита от деления на ноль
	if firstAvg == 0 {
		if secondAvg > 0 {
			return "rising"
		}
		return "stable"
	}

	change := (secondAvg - firstAvg) / firstAvg
	if change > 0.15 {
		return "rising"
	}
	if change < -0.15 {
		return "falling"
	}
	return "stable"
}

//
// ====== ЗАГРУЗКА КОНФИГА ======
//

func loadConfig() Config {
	cfg := Config{
		PromURL:         envOr("PROMETHEUS_URL", "http://kube-prometheus-stack-prometheus.monitoring:9090"),
		BackendURL:      envOr("PYTHON_BACKEND_URL", "http://incidentgpt-backend.incidentgpt.svc.cluster.local:8000/api/enriched"),
		GrafanaURL:      envOr("GRAFANA_URL", ""),
		LogsBaseURL:     envOr("LOGS_BASE_URL", ""),
		RunbookBaseURL:  envOr("RUNBOOK_BASE_URL", ""),
		ListenAddr:      envOr("LISTEN_ADDR", ":9099"),
		EnricherVersion: envOr("ENRICHER_VERSION", "0.4.0"),
		ClusterName:     envOr("CLUSTER_NAME", "unknown"),
		Environment:     envOr("ENVIRONMENT", "unknown"),

		RangeBefore: envDuration("PROM_RANGE_BEFORE", 15*time.Minute),
		RangeAfter:  envDuration("PROM_RANGE_AFTER", 5*time.Minute),

		RedisAddr:       envOr("REDIS_ADDR", "redis.incidentgpt.svc.cluster.local:6379"),
		RedisPassword:   envOr("REDIS_PASSWORD", ""),
		GroupBackendURL: envOr("GROUP_BACKEND_URL", "http://ai-worker.incidentgpt.svc:8080/incident-group"),
		RawBackendURL:   envOr("RAW_BACKEND_URL", "http://ai-worker.incidentgpt.svc:8080/incident-raw"),
		CorrWindow:      envDuration("CORR_WINDOW", 10*time.Minute),
		CorrWindowRaw:   envOr("CORR_WINDOW", "10m"),
		CorrSettle:      envDuration("CORR_SETTLE", 20*time.Second),
		RawDedupTTL:     envDuration("RAW_DEDUP_TTL", 2*time.Minute),

		EnableClusterContext:  envBool("ENRICH_CLUSTER_CONTEXT", true),
		EnableNodeContext:     envBool("ENRICH_NODE_CONTEXT", true),
		EnableWorkloadContext: envBool("ENRICH_WORKLOAD_CONTEXT", true),
		EnableExternalContext: envBool("ENRICH_EXTERNAL_CONTEXT", true),
		EnableK8sContext:      envBool("ENRICH_K8S_CONTEXT", true),
	}

	// Legacy поддержка PROM_QUERY_WINDOW
	if raw := strings.TrimSpace(os.Getenv("PROM_QUERY_WINDOW")); raw != "" && cfg.RangeBefore == 0 && cfg.RangeAfter == 0 {
		if d, err := time.ParseDuration(raw); err == nil {
			cfg.RangeBefore = d
			cfg.RangeAfter = 0
			log.Printf("INFO: using legacy PROM_QUERY_WINDOW=%s as RangeBefore", raw)
		}
	}

	return cfg
}

func loadMetricsConfig(path string) (MetricsConfig, error) {
	var cfg MetricsConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read metrics config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal metrics config: %w", err)
	}
	return cfg, nil
}

func renderExpr(expr string, ctx metricTemplateContext) (string, error) {
	t, err := template.New("expr").Parse(expr)
	if err != nil {
		return "", fmt.Errorf("parse expr template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute expr template: %w", err)
	}
	return buf.String(), nil
}

//
// ====== main() ======
//

func main() {
	appCfg = loadConfig()

	metricsPath := envOr("METRICS_CONFIG_PATH", "/etc/enricher/metrics.yaml")
	cfg, err := loadMetricsConfig(metricsPath)
	if err != nil {
		log.Fatalf("cannot load metrics config %s: %v", metricsPath, err)
	}
	metricsCfg = cfg

	// Redis для лёгкой корреляции (optional — фолбэк на поштучную отправку)
	if err := initRedis(); err != nil {
		log.Printf("WARN: redis init failed: %v (correlation disabled, alerts sent individually)", err)
	} else {
		log.Printf("INFO: redis connected at %s, correlation enabled (window=%s settle=%s raw_dedup_ttl=%s)",
			appCfg.RedisAddr, appCfg.CorrWindow, appCfg.CorrSettle, appCfg.RawDedupTTL)
	}

	// Kubernetes client (optional — graceful degradation if not in cluster)
	if appCfg.EnableK8sContext {
		if err := initK8sClient(); err != nil {
			log.Printf("WARN: k8s client init failed: %v (k8s enrichment disabled)", err)
		} else {
			log.Printf("INFO: k8s client initialized, pod/event/node enrichment enabled")
		}
	}

	log.Printf("Starting Data Enricher on %s", appCfg.ListenAddr)
	log.Printf("Prometheus: %s", appCfg.PromURL)
	log.Printf("Backend: %s", appCfg.BackendURL)
	log.Printf("Cluster: %s, Env: %s", appCfg.ClusterName, appCfg.Environment)
	log.Printf("Metrics range: before=%s, after=%s", appCfg.RangeBefore, appCfg.RangeAfter)
	log.Printf("Metrics config: cluster=%d node=%d workload=%d external=%d",
		len(metricsCfg.Cluster), len(metricsCfg.Node), len(metricsCfg.Workload), len(metricsCfg.External))

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/alert", alertHandler)

	srv := &http.Server{
		Addr:              appCfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown: deactivate machine on SIGTERM/SIGINT
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("INFO: received shutdown signal")

		shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

//
// ====== HTTP HANDLER ======
//

func alertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var alerts []AMAlert

	// Пытаемся распарсить сразу как []AMAlert
	if err := json.Unmarshal(body, &alerts); err != nil {
		// Если не получилось — пробуем как AMWebhook
		var wrap AMWebhook
		if err2 := json.Unmarshal(body, &wrap); err2 != nil {
			log.Printf("ERROR: failed to unmarshal alert body: %v / %v", err, err2)
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		alerts = wrap.Alerts
	}

	if len(alerts) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	ctx := r.Context()
	enriched := make([]EnrichedAlert, 0, len(alerts))

	for _, a := range alerts {
		e := enrichAlert(ctx, a)
		// Основной путь — буферизуем в Redis для групповой отправки.
		// Если Redis недоступен — фолбэк на поштучную отправку, чтобы не терять инциденты.
		if err := bufferAlert(e); err != nil {
			// errSkipResolved — не ошибка: resolved-алерты не корреллируем,
			// шлём поштучно, чтобы в канал ушло [RESOLVED]-уведомление.
			if !errors.Is(err, errSkipResolved) {
				log.Printf("WARN: redis buffering failed, falling back to direct send: %v", err)
			}
			if err := sendToBackend(ctx, e); err != nil {
				log.Printf("ERROR: failed to send enriched alert to backend: %v", err)
			}
		}
		enriched = append(enriched, e)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(enriched)
}

//
// ====== ОБОГАЩЕНИЕ АЛЕРТА ======
//

func enrichAlert(ctx context.Context, a AMAlert) EnrichedAlert {
	e := EnrichedAlert{
		Source:       "alertmanager",
		Status:       a.Status,
		Labels:       cloneStrMap(a.Labels),
		Annotations:  cloneStrMap(a.Annotations),
		StartsAt:     a.StartsAt,
		EndsAt:       a.EndsAt,
		Fingerprint:  a.Fingerprint,
		GeneratorURL: a.GeneratorURL,
		Severity:     a.Labels["severity"],
		PromSample:   map[string]string{},
		K8sContext:   map[string]string{},
		ExtraNotes:   []string{},
		GrafanaLinks: []string{},
	}
	// Лог: сырой алерт до обогащения
	logAlertRaw(a)
	if e.Fingerprint == "" {
		e.Fingerprint = stableFingerprint(e.Labels)
	}
	e.Owner = ownerFromLabels(a.Labels)

	e.EnricherVer = appCfg.EnricherVersion
	e.ClusterName = appCfg.ClusterName
	e.Environment = appCfg.Environment
	e.Enriched = true
	e.EnrichedAt = time.Now().UTC().Format(time.RFC3339)

	anchor := a.StartsAt
	if anchor.IsZero() {
		anchor = time.Now().UTC()
	}
	from := anchor.Add(-appCfg.RangeBefore)
	to := anchor.Add(appCfg.RangeAfter)

	e.MetricsAnchor = anchor.UTC().Format(time.RFC3339)
	e.MetricsBefore = appCfg.RangeBefore.String()
	e.MetricsAfter = appCfg.RangeAfter.String()
	e.MetricsWindow = fmt.Sprintf("[%s .. %s]", from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))

	// Primary expr
	primaryExpr := guessPrimaryExpr(a)
	if primaryExpr != "" {
		e.PrimaryMetric = primaryExpr
		addNote(&e, fmt.Sprintf("Primary metric expr: %s", primaryExpr))

		if stats, err := queryStats(ctx, primaryExpr, from, to); err != nil {
			log.Printf("WARN: primary metric query failed: %v", err)
		} else {
			e.PromSample["primary_expr"] = primaryExpr
			e.PromSample["primary_min"] = fmt.Sprintf("%f", stats.Min)
			e.PromSample["primary_max"] = fmt.Sprintf("%f", stats.Max)
			e.PromSample["primary_avg"] = fmt.Sprintf("%f", stats.Avg)
			e.PromSample["primary_last"] = fmt.Sprintf("%f", stats.Last)
			e.PromSample["primary_count"] = strconv.Itoa(stats.Count)
			e.PromSample["primary_trend"] = stats.Trend
			e.PrimarySummary = fmt.Sprintf("avg=%.2f, last=%.2f, min=%.2f, max=%.2f, points=%d, trend=%s",
				stats.Avg, stats.Last, stats.Min, stats.Max, stats.Count, stats.Trend)
		}
	}

	// Node / workload / cluster / external
	if appCfg.EnableNodeContext {
		attachNodeContext(ctx, &e, a, from, to)
	}
	if appCfg.EnableWorkloadContext {
		attachWorkloadContext(ctx, &e, a, from, to)
	}
	if appCfg.EnableClusterContext {
		attachClusterContext(ctx, &e, a, from, to)
	}
	if appCfg.EnableExternalContext {
		attachExternalContext(ctx, &e, a, from, to)
	}

	// Kubernetes API enrichment (pod status, events, node conditions, namespace health)
	if appCfg.EnableK8sContext {
		runK8sEnrichment(ctx, &e, a)
	}

	// Grafana / логи / runbook
	if appCfg.GrafanaURL != "" {
		if links := buildGrafanaLinks(a, from, to); len(links) > 0 {
			e.GrafanaLinks = append(e.GrafanaLinks, links...)
		}
	}
	if appCfg.LogsBaseURL != "" {
		if _, ok := e.Annotations["logs_url"]; !ok {
			e.Annotations["logs_url"] = buildLogsURL(a, from, to)
		}
	}
	if rb := defaultRunbook(a); rb != "" {
		if _, ok := e.Annotations["runbook_url"]; !ok {
			e.Annotations["runbook_url"] = rb
		}
	}

	e.IncidentHints = ruleOfThumbHints(a, e)

	logEnrichedAlert(e)

	return e
}

func addNote(e *EnrichedAlert, note string) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	e.ExtraNotes = append(e.ExtraNotes, note)
}

//
// ====== PROMETHEUS ======
//

func queryStats(ctx context.Context, expr string, from, to time.Time) (MetricStats, error) {
	u, err := url.Parse(appCfg.PromURL)
	if err != nil {
		return MetricStats{}, fmt.Errorf("invalid PROMETHEUS_URL: %w", err)
	}
	u.Path = "/api/v1/query_range"

	total := to.Sub(from)
	if total <= 0 {
		return MetricStats{}, fmt.Errorf("invalid range: from >= to")
	}
	step := total / 60
	if step < 15*time.Second {
		step = 15 * time.Second
	}

	q := u.Query()
	q.Set("query", expr)
	q.Set("start", fmt.Sprintf("%d", from.Unix()))
	q.Set("end", fmt.Sprintf("%d", to.Unix()))
	q.Set("step", fmt.Sprintf("%.0f", step.Seconds()))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return MetricStats{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return MetricStats{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return MetricStats{}, fmt.Errorf("prometheus HTTP %d: %s", resp.StatusCode, string(body))
	}

	var pr promRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return MetricStats{}, fmt.Errorf("decode prometheus response: %w", err)
	}
	if pr.Status != "success" {
		return MetricStats{}, fmt.Errorf("prometheus status=%s", pr.Status)
	}

	var allVals []float64
	for _, series := range pr.Data.Result {
		for _, pair := range series.Values {
			if len(pair) != 2 {
				continue
			}
			val, ok := parsePromValue(pair[1])
			if !ok {
				continue
			}
			allVals = append(allVals, val)
		}
	}

	return calcStats(allVals), nil
}

func guessPrimaryExpr(a AMAlert) string {
	if a.GeneratorURL != "" {
		if u, err := url.Parse(a.GeneratorURL); err == nil {
			if expr := u.Query().Get("g0.expr"); expr != "" {
				return expr
			}
			if expr := u.Query().Get("expr"); expr != "" {
				return expr
			}
		}
	}
	for _, key := range []string{"expr", "expression", "query"} {
		if v := strings.TrimSpace(a.Annotations[key]); v != "" {
			return v
		}
	}
	return ""
}

//
// ====== NODE / WORKLOAD / CLUSTER / EXTERNAL CONTEXT ======
//

// Функции attachNodeContext, attachWorkloadContext, attachClusterContext,
// attachExternalContext вынесены в enrichment.go (единая attachMetrics).

//
// ====== GRAFANA / LOGS / RUNBOOK ======
//

func buildGrafanaLinks(a AMAlert, from, to time.Time) []string {
	if appCfg.GrafanaURL == "" {
		return nil
	}
	u, err := url.Parse(appCfg.GrafanaURL)
	if err != nil {
		return nil
	}
	u.Path = "/explore"

	ns := a.Labels["namespace"]
	svc := a.Labels["service"]
	inst := a.Labels["instance"]

	var exprParts []string
	if ns != "" {
		exprParts = append(exprParts, fmt.Sprintf(`namespace="%s"`, ns))
	}
	if svc != "" {
		exprParts = append(exprParts, fmt.Sprintf(`service="%s"`, svc))
	}
	if inst != "" {
		exprParts = append(exprParts, fmt.Sprintf(`instance="%s"`, inst))
	}
	queryExpr := strings.Join(exprParts, " ")

	q := u.Query()
	if queryExpr != "" {
		q.Set("left", fmt.Sprintf(`{"datasource":"Loki","queries":[{"expr":"%s"}]}`, queryExpr))
	}
	q.Set("from", fmt.Sprintf("%d", from.UnixMilli()))
	q.Set("to", fmt.Sprintf("%d", to.UnixMilli()))
	u.RawQuery = q.Encode()

	return []string{u.String()}
}

func buildLogsURL(a AMAlert, from, to time.Time) string {
	if appCfg.LogsBaseURL == "" {
		return ""
	}
	u, err := url.Parse(appCfg.LogsBaseURL)
	if err != nil {
		return ""
	}

	ns := a.Labels["namespace"]
	svc := a.Labels["service"]
	inst := a.Labels["instance"]

	q := u.Query()
	if ns != "" {
		q.Set("namespace", ns)
	}
	if svc != "" {
		q.Set("service", svc)
	}
	if inst != "" {
		q.Set("instance", inst)
	}
	q.Set("from", fmt.Sprintf("%d", from.Unix()))
	q.Set("to", fmt.Sprintf("%d", to.Unix()))
	u.RawQuery = q.Encode()
	return u.String()
}

// defaultRunbook строит ссылку на runbook по имени алерта.
// База берётся из RUNBOOK_BASE_URL; если не задана — ссылку не добавляем.
func defaultRunbook(a AMAlert) string {
	base := strings.TrimRight(appCfg.RunbookBaseURL, "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", base, strings.ToLower(a.Labels["alertname"]))
}

//
// ====== ПОДСКАЗКИ ПО ИНЦИДЕНТУ ======
//

func ruleOfThumbHints(a AMAlert, e EnrichedAlert) []string {
	var hints []string

	if e.Severity == "critical" {
		hints = append(hints, "Критический алерт — убедись, что on-call уведомлён и заведи инцидент в тикет-системе.")
	}
	if ns := a.Labels["namespace"]; ns != "" {
		hints = append(hints, fmt.Sprintf("Проверь состояние pod'ов в namespace %q (kubectl get pods -n %s).", ns, ns))
	}
	if svc := a.Labels["service"]; svc != "" {
		hints = append(hints, fmt.Sprintf("Проверь HTTP-ошибки и latency для сервиса %q в Grafana.", svc))
	}
	if node := a.Labels["node"]; node != "" {
		hints = append(hints, fmt.Sprintf("Проверь состояние ноды %q (kubectl describe node %s).", node, node))
	}

	return hints
}

// ====== ЛОГИРОВАНИЕ АЛЕРТОВ ======

func logAlertRaw(a AMAlert) {
	alertName := a.Labels["alertname"]
	severity := a.Labels["severity"]

	fp := a.Fingerprint
	if fp == "" {
		fp = stableFingerprint(a.Labels)
	}

	log.Printf(
		"ALERT_RAW: fingerprint=%s alertname=%s severity=%s namespace=%s labels=%d annotations=%d",
		fp, alertName, severity,
		a.Labels["namespace"],
		len(a.Labels),
		len(a.Annotations),
	)
}

func logEnrichedAlert(e EnrichedAlert) {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		log.Printf("ERROR: cannot marshal enriched alert for logging (fingerprint=%s): %v", e.Fingerprint, err)
		return
	}

	// Немного «резюме», чтобы в логах было видно, что именно добавилось
	log.Printf(
		"ALERT_ENRICHED: fingerprint=%s alertname=%s severity=%s "+
			"prom_keys=%d k8s_keys=%d grafana_links=%d extra_notes=%d hints=%d\n%s",
		e.Fingerprint,
		e.Labels["alertname"],
		e.Severity,
		len(e.PromSample),
		len(e.K8sContext),
		len(e.GrafanaLinks),
		len(e.ExtraNotes),
		len(e.IncidentHints),
		string(b),
	)
}

//
// ====== ОТПРАВКА В BACKEND ======
//

func sendToBackend(ctx context.Context, e EnrichedAlert) error {
	if appCfg.BackendURL == "" {
		log.Printf("INFO: PYTHON_BACKEND_URL is empty, skip sending enriched alert")
		return nil
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal enriched alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appCfg.BackendURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("backend HTTP %d: %s", resp.StatusCode, string(body))
	}

	log.Printf(
		"ALERT_SENT: fingerprint=%s alertname=%s severity=%s backend=%s status=%d",
		e.Fingerprint,
		e.Labels["alertname"],
		e.Severity,
		appCfg.BackendURL,
		resp.StatusCode,
	)

	return nil
}
