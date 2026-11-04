package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const aiControlKey = "igpt:operations:ai-control"

type AIControlState struct {
	Schema    int       `json:"schema_version"`
	Enabled   bool      `json:"enabled"`
	Revision  int       `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

func operationsEnabled() bool { return env("OPERATIONS_ENABLED", "false") == "true" }
func readAIControl(ctx context.Context) (AIControlState, error) {
	state := AIControlState{Enabled: false}
	if store.disk == nil {
		return state, fmt.Errorf("persistent control unavailable")
	}
	raw, err := store.disk.rdb.Get(ctx, aiControlKey).Bytes()
	if err == redis.Nil {
		return state, nil
	}
	var record struct {
		Schema    int       `json:"schema_version"`
		Enabled   *bool     `json:"enabled"`
		Revision  int       `json:"revision"`
		UpdatedAt time.Time `json:"updated_at"`
		UpdatedBy string    `json:"updated_by"`
	}
	if err != nil || json.Unmarshal(raw, &record) != nil || record.Schema != 1 || record.Enabled == nil || record.Revision < 1 || record.UpdatedAt.IsZero() {
		return AIControlState{}, fmt.Errorf("control storage unavailable")
	}
	return AIControlState{Schema: 1, Enabled: *record.Enabled, Revision: record.Revision, UpdatedAt: record.UpdatedAt, UpdatedBy: record.UpdatedBy}, nil
}

var writeAIControl = redis.NewScript(`
local current=redis.call('GET',KEYS[1]);local revision=0
if current then revision=cjson.decode(current).revision end
if revision~=tonumber(ARGV[1]) then return 0 end
redis.call('SET',KEYS[1],ARGV[2])
redis.call('XADD',KEYS[2],'MAXLEN','~',1000,'*','state',ARGV[2])
return 1`)

func handleAIControl(w http.ResponseWriter, r *http.Request) {
	if !operationsEnabled() {
		http.Error(w, "operations disabled", 404)
		return
	}
	if r.Method == "PUT" {
		if !sharedAdminMutationAuth(w, r) {
			return
		}
		if !trustedBrowserOrigin(r) {
			http.Error(w, "trusted Origin required", 403)
			return
		}
	} else if !sharedAdminAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	if r.Method == "GET" {
		state, err := readAIControl(ctx)
		if err != nil {
			http.Error(w, "control unavailable", 503)
			return
		}
		writeJSON(w, 200, state)
		return
	}
	var body struct {
		Enabled  *bool `json:"enabled"`
		Revision int   `json:"revision"`
	}
	if decodeChange(r, &body) != nil || body.Enabled == nil || body.Revision < 0 {
		http.Error(w, "enabled and revision required", 400)
		return
	}
	if store.disk == nil {
		http.Error(w, "persistent control unavailable", 503)
		return
	}
	if *body.Enabled {
		workerCheck, runtime := probeComponent(ctx, "worker_runtime", "Worker", workerBase()+"/internal/operations", true)
		if workerCheck.Status != "healthy" || runtime["managed"] != true {
			http.Error(w, "managed worker unavailable; control not changed", 503)
			return
		}
	}
	actor, _, _ := r.BasicAuth()
	if len(actor) > 200 {
		http.Error(w, "actor identity exceeds budget", 400)
		return
	}
	state := AIControlState{Schema: 1, Enabled: *body.Enabled, Revision: body.Revision + 1, UpdatedAt: time.Now().UTC(), UpdatedBy: actor}
	raw, _ := json.Marshal(state)
	n, err := writeAIControl.Run(ctx, store.disk.rdb, []string{aiControlKey, aiControlKey + ":audit"}, body.Revision, raw).Int()
	if err != nil {
		http.Error(w, "control persistence failed", 503)
		return
	}
	if n != 1 {
		http.Error(w, "control changed; refresh before retrying", 409)
		return
	}
	operationsCache.Lock()
	operationsCache.at = time.Time{}
	operationsCache.Unlock()
	writeJSON(w, 200, state)
}
func operationsServiceAuth(w http.ResponseWriter, r *http.Request) bool {
	token := strings.TrimSpace(env("OPERATIONS_SERVICE_TOKEN", ""))
	if len(token) < 32 || r.Header.Get("Origin") != "" || !constantToken(r.Header.Get("Authorization"), "Bearer "+token) {
		http.Error(w, "unauthorized", 401)
		return false
	}
	return true
}
func handleWorkerAIControl(w http.ResponseWriter, r *http.Request) {
	if !operationsEnabled() {
		http.Error(w, "operations disabled", 404)
		return
	}
	if !operationsServiceAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	state, err := readAIControl(ctx)
	if err != nil {
		http.Error(w, "control unavailable", 503)
		return
	}
	writeJSON(w, 200, map[string]any{"enabled": state.Enabled, "revision": state.Revision, "updated_at": state.UpdatedAt})
}

type ComponentCheck struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail"`
	CheckedAt time.Time `json:"checked_at"`
	LatencyMS int64     `json:"latency_ms"`
}
type OperationsSnapshot struct {
	CheckedAt        time.Time        `json:"checked_at"`
	Components       []ComponentCheck `json:"components"`
	Worker           map[string]any   `json:"worker,omitempty"`
	Control          *AIControlState  `json:"control"`
	ControlAvailable bool             `json:"control_available"`
}

var operationsClient = &http.Client{Timeout: 3500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
var operationsCache struct {
	sync.Mutex
	at    time.Time
	value OperationsSnapshot
}

func workerBase() string {
	if configured := env("OPERATIONS_WORKER_URL", ""); configured != "" {
		return strings.TrimRight(configured, "/")
	}
	u, err := url.Parse(aiWorkerURL)
	if err != nil {
		return ""
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}
func probeComponent(ctx context.Context, id, name, endpoint string, authenticated bool) (ComponentCheck, map[string]any) {
	check := ComponentCheck{ID: id, Name: name, Status: "unknown", Detail: "Нет подтверждённого результата", CheckedAt: time.Now().UTC()}
	if endpoint == "" {
		check.Status = "not_configured"
		check.Detail = "Не настроен"
		return check, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		check.Detail = "Некорректный адрес проверки"
		return check, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return check, nil
	}
	if authenticated {
		token := strings.TrimSpace(env("OPERATIONS_SERVICE_TOKEN", ""))
		if len(token) < 32 {
			check.Detail = "Не настроены credentials мониторинга"
			return check, nil
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	resp, err := operationsClient.Do(req)
	check.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		check.Status = "unhealthy"
		check.Detail = "Нет ответа до таймаута"
		return check, nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32769))
	if err != nil || len(raw) > 32768 {
		check.Detail = "Некорректный ответ"
		return check, nil
	}
	if resp.StatusCode != 200 {
		check.Status = "unhealthy"
		check.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return check, nil
	}
	check.Status = "healthy"
	check.Detail = "Проверка готовности успешна"
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	return check, data
}
func collectOperations(ctx context.Context) OperationsSnapshot {
	operationsCache.Lock()
	defer operationsCache.Unlock()
	if time.Since(operationsCache.at) < 10*time.Second {
		return operationsCache.value
	}
	snapshot := OperationsSnapshot{CheckedAt: time.Now().UTC(), Components: []ComponentCheck{{ID: "backend", Name: "Backend API", Status: "healthy", Detail: "API ответил на этот запрос", CheckedAt: time.Now().UTC()}}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, target := range []struct {
		id, name, url string
		auth          bool
	}{{"worker", "AI worker", workerBase() + "/readyz", false}, {"enricher", "Enricher", strings.TrimRight(env("OPERATIONS_ENRICHER_URL", ""), "/") + "/readyz", false}, {"sanitizer", "Sanitizer", strings.TrimRight(env("SANITIZER_URL", ""), "/") + "/readyz", false}, {"mcp", "База знаний MCP", env("OPERATIONS_MCP_HEALTH_URL", ""), false}, {"worker_runtime", "Runtime AI worker", workerBase() + "/internal/operations", true}, {"enricher_runtime", "Runtime enricher", strings.TrimRight(env("OPERATIONS_ENRICHER_URL", ""), "/") + "/internal/operations", true}} {
		wg.Add(1)
		go func(target struct {
			id, name, url string
			auth          bool
		}) {
			defer wg.Done()
			endpoint := target.url
			if endpoint == "/readyz" {
				endpoint = ""
			}
			check, data := probeComponent(ctx, target.id, target.name, endpoint, target.auth)
			mu.Lock()
			defer mu.Unlock()
			if target.id == "enricher_runtime" {
				names := map[string]string{"metrics": "Источник метрик", "enricher_redis": "Redis из enricher", "kubernetes": "Kubernetes API"}
				checks, _ := data["checks"].([]any)
				for id, name := range names {
					component := ComponentCheck{ID: id, Name: name, Status: "unknown", Detail: "Нет свежей проверки enricher", CheckedAt: check.CheckedAt}
					if check.Status == "healthy" {
						for _, item := range checks {
							value, ok := item.(map[string]any)
							if !ok || value["id"] != id {
								continue
							}
							if status, ok := value["status"].(string); ok && knownComponentStatus(status) {
								component.Status = status
							}
							if detail, ok := value["detail"].(string); ok && len(detail) <= 200 {
								component.Detail = detail
							}
						}
					}
					snapshot.Components = append(snapshot.Components, component)
				}
				return
			}
			if target.id == "worker_runtime" {
				if check.Status == "healthy" && data["service"] == "ai-worker" {
					snapshot.Worker = safeWorkerSnapshot(data)
				}
				return
			}

			snapshot.Components = append(snapshot.Components, check)
		}(target)
	}
	wg.Wait()
	redisCheck := ComponentCheck{ID: "redis", Name: "Redis", Status: "not_configured", Detail: "Постоянное хранилище не настроено", CheckedAt: time.Now().UTC()}
	if store.disk != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		start := time.Now()
		err := store.disk.rdb.Ping(pingCtx).Err()
		cancel()
		redisCheck.LatencyMS = time.Since(start).Milliseconds()
		if err == nil {
			redisCheck.Status = "healthy"
			redisCheck.Detail = "Redis PING успешен"
		} else {
			redisCheck.Status = "unhealthy"
			redisCheck.Detail = "Redis не отвечает"
		}
	}
	snapshot.Components = append(snapshot.Components, redisCheck)
	model := ComponentCheck{ID: "model", Name: "Модель LLM", Status: "unknown", Detail: "Нет недавнего подтверждённого ответа модели", CheckedAt: time.Now().UTC()}
	if data, ok := snapshot.Worker["model"].(map[string]any); ok {
		if status, ok := data["status"].(string); ok && (status == "healthy" || status == "unhealthy" || status == "unknown") {
			model.Status = status
		}
		if at, ok := data["checked_at"].(string); ok {
			model.CheckedAt, _ = time.Parse(time.RFC3339Nano, at)
		}
		if model.CheckedAt.IsZero() || time.Since(model.CheckedAt) > 5*time.Minute {
			model.Status = "unknown"
		}
		if model.Status == "healthy" {
			model.Detail = "Получен корректный ответ на реальный запрос"
		} else if model.Status == "unhealthy" {
			model.Detail = "Последний вызов модели завершился ошибкой"
		}
	}
	snapshot.Components = append(snapshot.Components, model)
	telegram := ComponentCheck{ID: "telegram", Name: "Telegram Bot API", Status: "unknown", Detail: "Нет проверки Telegram getMe", CheckedAt: time.Now().UTC()}
	if data, ok := snapshot.Worker["telegram"].(map[string]any); ok {
		if status, ok := data["status"].(string); ok && knownComponentStatus(status) {
			telegram.Status = status
		}
		if at, ok := data["checked_at"].(string); ok {
			telegram.CheckedAt, _ = time.Parse(time.RFC3339Nano, at)
		}
		if telegram.Status == "healthy" {
			telegram.Detail = "getMe успешен; отправка в канал отдельно не проверялась"
		}
	}
	snapshot.Components = append(snapshot.Components, telegram, probeLogs(ctx))

	order := map[string]int{"model": 0, "worker": 1, "backend": 2, "enricher": 3, "redis": 4, "enricher_redis": 5, "sanitizer": 6, "telegram": 7, "metrics": 8, "logs": 9, "kubernetes": 10, "mcp": 11}
	sort.SliceStable(snapshot.Components, func(i, j int) bool { return order[snapshot.Components[i].ID] < order[snapshot.Components[j].ID] })
	operationsCache.at = time.Now()
	operationsCache.value = snapshot
	return snapshot
}
func handleOperations(w http.ResponseWriter, r *http.Request) {
	if !operationsEnabled() {
		http.Error(w, "operations disabled", 404)
		return
	}
	if !sharedAdminAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	snapshot := collectOperations(ctx)
	state, err := readAIControl(ctx)
	snapshot.ControlAvailable = err == nil
	if err == nil {
		snapshot.Control = &state
	}
	writeJSON(w, 200, snapshot)
}
func handleModelProbe(w http.ResponseWriter, r *http.Request) {
	if !operationsEnabled() {
		http.Error(w, "operations disabled", 404)
		return
	}
	if !sharedAdminMutationAuth(w, r) {
		return
	}
	if !trustedBrowserOrigin(r) {
		http.Error(w, "trusted Origin required", 403)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	state, err := readAIControl(ctx)
	if err != nil {
		http.Error(w, "control unavailable", 503)
		return
	}
	if !state.Enabled {
		http.Error(w, "AI paused", 423)
		return
	}
	allowed, err := store.disk.rdb.SetNX(ctx, "igpt:operations:model-probe:limit", "1", time.Minute).Result()
	if err != nil {
		http.Error(w, "probe limiter unavailable", 503)
		return
	}
	if !allowed {
		http.Error(w, "probe limited to once per minute", 429)
		return
	}
	req, err := http.NewRequestWithContext(ctx, "POST", workerBase()+"/internal/operations/model-probe", nil)
	if err != nil {
		http.Error(w, "invalid worker configuration", 503)
		return
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(env("OPERATIONS_SERVICE_TOKEN", "")))
	client := &http.Client{Timeout: 11 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "worker probe unavailable", 502)
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8193))
	if err != nil || len(raw) > 8192 {
		http.Error(w, "invalid worker response", 502)
		return
	}
	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprintf("worker probe HTTP %d", resp.StatusCode), resp.StatusCode)
		return
	}
	var result struct {
		Success   bool  `json:"success"`
		LatencyMS int64 `json:"latency_ms"`
	}
	if json.Unmarshal(raw, &result) != nil {
		http.Error(w, "invalid probe response", 502)
		return
	}
	operationsCache.Lock()
	operationsCache.at = time.Time{}
	operationsCache.Unlock()
	writeJSON(w, 200, result)
}

func knownComponentStatus(status string) bool {
	switch status {
	case "healthy", "unhealthy", "unknown", "disabled", "not_configured":
		return true
	}
	return false
}
func safeWorkerSnapshot(data map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"managed", "control_known"} {
		if value, ok := data[key].(bool); ok {
			result[key] = value
		}
	}
	if value, ok := data["active_provider_requests"].(float64); ok && value >= 0 && value < 1000000 {
		result["active_provider_requests"] = value
	}
	if control, ok := data["control"].(map[string]any); ok {
		item := map[string]any{}
		if enabled, ok := control["enabled"].(bool); ok {
			item["enabled"] = enabled
		}
		if revision, ok := control["revision"].(float64); ok && revision >= 0 {
			item["revision"] = revision
		}
		result["control"] = item
	}
	for _, key := range []string{"model", "telegram"} {
		if value, ok := data[key].(map[string]any); ok {
			item := map[string]any{}
			if status, ok := value["status"].(string); ok && knownComponentStatus(status) {
				item["status"] = status
			}
			if at, ok := value["checked_at"].(string); ok {
				if _, err := time.Parse(time.RFC3339Nano, at); err == nil {
					item["checked_at"] = at
				}
			}
			result[key] = item
		}
	}
	return result
}
func probeLogs(ctx context.Context) ComponentCheck {
	check := ComponentCheck{ID: "logs", Name: "Источник логов", Status: "not_configured", Detail: "Хранилище логов не настроено", CheckedAt: time.Now().UTC()}
	endpoint := strings.TrimRight(env("LOGS_STORE_URL", ""), "/")
	if endpoint == "" {
		return check
	}
	if env("LOGS_KIND", "opensearch") == "loki" {
		endpoint += "/ready"
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" {
		check.Status = "unknown"
		check.Detail = "Некорректный адрес источника"
		return check
	}
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		check.Status = "unknown"
		return check
	}
	if user := env("LOGS_STORE_USER", ""); user != "" {
		req.SetBasicAuth(user, env("LOGS_STORE_PASSWORD", ""))
	}
	start := time.Now()
	resp, err := operationsClient.Do(req)
	check.LatencyMS = time.Since(start).Milliseconds()
	check.Status = "unhealthy"
	check.Detail = "Нет подтверждённой связи с хранилищем"
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			check.Status = "healthy"
			check.Detail = "Health API источника ответил; поиск логов не проверялся"
		}
	}
	return check
}
