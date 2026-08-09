package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Логи как ещё один слой обогащения — рядом с метриками и контекстом Kubernetes.
//
// Раньше модель видела цифры и события, но не строки ошибок, и в разборе честно
// советовала «посмотри логи пода». Теперь выжимка логов едет вместе с алертом.
//
// Важно, что этим занимается именно enricher: его payload проходит через
// sanitizer, поэтому секреты в логах маскируются до отправки в LLM.

type logsConfig struct {
	enabled  bool
	url      string
	user     string
	password string
	index    string
	before   time.Duration
	after    time.Duration
	maxLines int
}

var logsCfg = loadLogsConfig()

func loadLogsConfig() logsConfig {
	c := logsConfig{
		url:      os.Getenv("LOGS_STORE_URL"),
		user:     os.Getenv("LOGS_STORE_USER"),
		password: os.Getenv("LOGS_STORE_PASSWORD"),
		index:    envOr("LOGS_STORE_INDEX", "logs-*"),
		before:   durationOr("LOGS_RANGE_BEFORE", 10*time.Minute),
		after:    durationOr("LOGS_RANGE_AFTER", 2*time.Minute),
		// В промпт больше нескольких десятков строк класть нельзя: он раздувается,
		// дорожает и топит важное в шуме.
		maxLines: intOr("LOGS_MAX_LINES", 40),
	}
	c.enabled = c.url != ""
	return c
}

func durationOr(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func intOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

var logsHTTP = &http.Client{Timeout: 8 * time.Second}

// Уровень в логах не размечен, определяем по тексту. FATAL раньше ERROR, иначе
// «fatal error» будет распознан как ERROR.
var (
	reFatal = regexp.MustCompile(`(?i)\b(fatal|panic|emergency)\b`)
	reError = regexp.MustCompile(`(?i)\b(error|err|exception|failed|failure|refused|timeout)\b`)
	reWarn  = regexp.MustCompile(`(?i)\b(warn|warning)\b`)

	// Для схлопывания повторов убираем из строки всё, что меняется от раза к разу:
	// времена, числа, идентификаторы. Иначе одна и та же ошибка выглядит уникальной.
	reTimestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?Z?`)
	reNumbers   = regexp.MustCompile(`\b\d+\b`)
	reHex       = regexp.MustCompile(`\b[0-9a-f]{8,}\b`)
)

func logLevel(line string) string {
	switch {
	case reFatal.MatchString(line):
		return "FATAL"
	case reError.MatchString(line):
		return "ERROR"
	case reWarn.MatchString(line):
		return "WARN"
	default:
		return "INFO"
	}
}

func logFingerprint(line string) string {
	s := reTimestamp.ReplaceAllString(line, "")
	s = reHex.ReplaceAllString(s, "")
	s = reNumbers.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

type logGroup struct {
	level   string
	pod     string
	sample  string
	count   int
	firstAt string
}

// attachLogsContext добавляет к алерту выжимку логов из хранилища.
func attachLogsContext(ctx context.Context, e *EnrichedAlert, a AMAlert, at time.Time) {
	if !logsCfg.enabled {
		return
	}
	ns := a.Labels["namespace"]
	if ns == "" {
		return
	}

	// Берём весь namespace, а не только под из алерта: причина часто у соседа.
	// Разбор про catalogue упирался в базу, а логов catalogue-db в данных не было,
	// и модель справедливо просила их принести.
	groups, err := fetchLogGroups(ctx, ns, "", at)
	if err != nil {
		log.Printf("WARN: logs enrichment failed ns=%s: %v", ns, err)
		return
	}
	if len(groups) == 0 {
		return
	}

	// Под из алерта показываем первым: он в центре внимания, остальные — контекст.
	if pod := a.Labels["pod"]; pod != "" {
		base := pod
		if i := strings.LastIndex(pod, "-"); i > 0 {
			base = pod[:i]
		}
		sort.SliceStable(groups, func(i, j int) bool {
			return strings.HasPrefix(groups[i].pod, base) && !strings.HasPrefix(groups[j].pod, base)
		})
	}

	if len(groups) > logsCfg.maxLines {
		groups = groups[:logsCfg.maxLines]
	}

	lines := make([]string, 0, len(groups))
	for _, g := range groups {
		// Счётчик повторов — сам по себе сигнал: одна ошибка или шторм.
		if g.count > 1 {
			lines = append(lines, fmt.Sprintf("[%s ×%d] %s: %s", g.level, g.count, g.pod, g.sample))
		} else {
			lines = append(lines, fmt.Sprintf("[%s] %s: %s", g.level, g.pod, g.sample))
		}
	}
	e.RecentLogs = lines
	log.Printf("LOGS_ENRICHED: ns=%s pod=%s groups=%d", ns, a.Labels["pod"], len(lines))
}

func fetchLogGroups(ctx context.Context, namespace, pod string, at time.Time) ([]logGroup, error) {
	from := at.Add(-logsCfg.before).UTC().Format(time.RFC3339)
	to := at.Add(logsCfg.after).UTC().Format(time.RFC3339)

	filters := []map[string]any{
		{"range": map[string]any{"@timestamp": map[string]any{"gte": from, "lte": to}}},
		{ // Поле индексируется как text с подполем keyword; точное совпадение
			// работает только по .keyword, иначе term молча находит ноль.
			"term": map[string]any{"kubernetes.namespace_name.keyword": namespace}},
	}
	if pod != "" {
		base := pod
		if i := strings.LastIndex(pod, "-"); i > 0 {
			base = pod[:i]
		}
		filters = append(filters, map[string]any{
			"prefix": map[string]any{"kubernetes.pod_name.keyword": base},
		})
	}

	// Берём с запасом: после схлопывания повторов останется намного меньше.
	query := map[string]any{
		"size":  500,
		"sort":  []any{map[string]any{"@timestamp": "desc"}},
		"query": map[string]any{"bool": map[string]any{"filter": filters}},
	}
	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_search", strings.TrimRight(logsCfg.url, "/"), logsCfg.index)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if logsCfg.user != "" {
		req.SetBasicAuth(logsCfg.user, logsCfg.password)
	}

	resp, err := logsHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("logs store responded %d", resp.StatusCode)
	}

	var out struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	byFingerprint := map[string]*logGroup{}
	for _, h := range out.Hits.Hits {
		msg, _ := h.Source["log"].(string)
		if msg == "" {
			msg, _ = h.Source["message"].(string)
		}
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		level := logLevel(msg)
		// INFO в разбор не тянем: причину ищут в ошибках, а место в промпте дорогое.
		if level == "INFO" {
			continue
		}

		podName := ""
		if k, ok := h.Source["kubernetes"].(map[string]any); ok {
			podName, _ = k["pod_name"].(string)
		}
		ts, _ := h.Source["@timestamp"].(string)

		key := podName + "|" + logFingerprint(msg)
		if g, ok := byFingerprint[key]; ok {
			g.count++
			if ts < g.firstAt {
				g.firstAt = ts
			}
			continue
		}
		if len([]rune(msg)) > 300 {
			msg = string([]rune(msg)[:300]) + "…"
		}
		byFingerprint[key] = &logGroup{level: level, pod: podName, sample: msg, count: 1, firstAt: ts}
	}

	groups := make([]logGroup, 0, len(byFingerprint))
	for _, g := range byFingerprint {
		groups = append(groups, *g)
	}
	// Сначала самое серьёзное, внутри уровня — самое частое.
	rank := map[string]int{"FATAL": 0, "ERROR": 1, "WARN": 2}
	sort.Slice(groups, func(i, j int) bool {
		if rank[groups[i].level] != rank[groups[j].level] {
			return rank[groups[i].level] < rank[groups[j].level]
		}
		return groups[i].count > groups[j].count
	})
	return groups, nil
}
