package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Чтение логов инцидента из хранилища логов (OpenSearch/Elasticsearch API).
//
// Логи собирает fluent-bit, но до сих пор их никто не читал: вкладка «Логи»
// всегда была пустой. Берём окно вокруг инцидента и тот же namespace —
// это и есть контекст, в котором инженер ищет причину.

const (
	// Насколько заглядываем до и после начала инцидента. До — чтобы увидеть,
	// что происходило перед сбоем; после — чтобы поймать последствия.
	logsBefore = 10 * time.Minute
	logsAfter  = 5 * time.Minute
	logsLimit  = 300
)

// Уровень fluent-bit не размечает, поэтому определяем по тексту строки.
// Порядок важен: FATAL проверяем раньше ERROR, иначе «fatal error» станет ERROR.
var levelPatterns = []struct {
	level string
	re    *regexp.Regexp
}{
	{"FATAL", regexp.MustCompile(`(?i)\b(fatal|panic|emergency)\b`)},
	// Набор слов должен совпадать с enricher/logs.go: иначе строку, которую
	// модель получила как ошибку, инженер не найдёт под фильтром ERROR.
	{"ERROR", regexp.MustCompile(`(?i)\b(error|err|exception|failed|failure|refused|timeout)\b`)},
	{"WARN", regexp.MustCompile(`(?i)\b(warn|warning)\b`)},
}

func detectLevel(line string) string {
	for _, p := range levelPatterns {
		if p.re.MatchString(line) {
			return p.level
		}
	}
	return "INFO"
}

var stacktraceRe = regexp.MustCompile(`(?m)^\s+(at |\.\.\.|goroutine |\tat )`)

// GET /api/v1/incidents/{id}/logs
func handleIncidentLogs(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
	inc := store.Get(parts[0])
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}

	cfg := logsStoreConfig()
	if cfg.url == "" {
		// Не выдаём пустой список молча: пусть интерфейс скажет, почему логов нет.
		writeJSON(w, 200, map[string]any{
			"data":   []LogEntry{},
			"reason": "хранилище логов не настроено в интеграциях",
		})
		return
	}

	// Имя пода: сначала из обогащённого контекста, иначе из меток алерта —
	// enricher не всегда заполняет k8s_context, а в labels под почти всегда есть.
	pod := ""
	if inc.Context != nil {
		pod = inc.Context.Pod
	}
	if pod == "" {
		pod = inc.Labels["pod"]
	}
	// Логи всего namespace тоже полезны: сосед мог упасть первым. Поэтому
	// сужение до пода включается флагом, а не молча.
	if r.URL.Query().Get("scope") == "namespace" {
		pod = ""
	}

	entries, err := fetchLogs(cfg, inc.Namespace, pod, inc.CreatedAt, r.URL.Query().Get("level"))
	if err != nil {
		writeJSON(w, 200, map[string]any{"data": []LogEntry{}, "reason": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"data": entries})
}

type logsStore struct {
	url, user, pass, index string
}

func logsStoreConfig() logsStore {
	integrations.mu.RLock()
	defer integrations.mu.RUnlock()
	it, ok := integrations.items["logs"]
	if !ok {
		return logsStore{}
	}
	idx := it.Config["index"]
	if idx == "" {
		idx = "logs-*"
	}
	return logsStore{url: it.Config["url"], user: it.Config["username"], pass: it.Config["password"], index: idx}
}

func fetchLogs(cfg logsStore, namespace, pod string, at time.Time, level string) ([]LogEntry, error) {
	from := at.Add(-logsBefore).UTC().Format(time.RFC3339)
	to := at.Add(logsAfter).UTC().Format(time.RFC3339)

	filters := []map[string]any{
		{"range": map[string]any{"@timestamp": map[string]any{"gte": from, "lte": to}}},
	}
	if namespace != "" {
		filters = append(filters, map[string]any{
			// Поле индексируется как text с подполем keyword; точное совпадение
			// работает только по .keyword, иначе term молча находит ноль.
			"term": map[string]any{"kubernetes.namespace_name.keyword": namespace},
		})
	}
	// Под указываем как префикс: в алерте приходит полное имя, а в индексе
	// оно совпадает — но если под пересоздался, суффикс уже другой.
	if pod != "" {
		base := pod
		if i := strings.LastIndex(pod, "-"); i > 0 {
			base = pod[:i]
		}
		filters = append(filters, map[string]any{
			"prefix": map[string]any{"kubernetes.pod_name.keyword": base},
		})
	}

	query := map[string]any{
		"size": logsLimit,
		"sort": []any{map[string]any{"@timestamp": "asc"}},
		"query": map[string]any{
			"bool": map[string]any{"filter": filters},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_search", strings.TrimRight(cfg.url, "/"), cfg.index)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.user != "" {
		req.SetBasicAuth(cfg.user, cfg.pass)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("нет связи с хранилищем логов: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return []LogEntry{}, nil // индекса ещё нет — логов просто не накопилось
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("хранилище логов ответило %d", resp.StatusCode)
	}

	var out struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("не разобрать ответ хранилища логов: %w", err)
	}

	entries := make([]LogEntry, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		msg := firstString(h.Source, "log", "message", "msg")
		if strings.TrimSpace(msg) == "" {
			continue
		}
		lvl := strings.ToUpper(firstString(h.Source, "level", "severity"))
		if lvl == "" || (lvl != "INFO" && lvl != "WARN" && lvl != "ERROR" && lvl != "FATAL") {
			lvl = detectLevel(msg)
		}
		if level != "" && !strings.EqualFold(level, lvl) {
			continue
		}
		ts, _ := h.Source["@timestamp"].(string)

		// Имя пода в строке помогает, когда в группе несколько сервисов.
		if k, ok := h.Source["kubernetes"].(map[string]any); ok {
			if name, ok := k["pod_name"].(string); ok && name != "" {
				msg = name + "  " + msg
			}
		}

		entries = append(entries, LogEntry{
			Timestamp:    ts,
			Level:        lvl,
			Message:      strings.TrimRight(msg, "\n"),
			IsStacktrace: stacktraceRe.MatchString(msg),
		})
	}
	return entries, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
