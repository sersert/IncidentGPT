package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Реестр интеграций: что подключено, чем настроено и живо ли оно сейчас.
//
// Конфиг держим в памяти, как и всё остальное в этом сервисе. Значения по
// умолчанию берём из окружения — чтобы после рестарта интеграции подхватывали
// адреса, с которыми сервисы и так работают, а не превращались в пустые формы.

type IntegrationField struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Kind     string   `json:"kind"` // text | password | number | select
	Required bool     `json:"required"`
	Hint     string   `json:"hint,omitempty"`
	Options  []Option `json:"options,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Integration struct {
	Type     string             `json:"type"`
	Name     string             `json:"name"`
	Category string             `json:"category"` // data_source | output_channel
	Status   string             `json:"status"`   // connected | disconnected | not_configured
	Config   map[string]string  `json:"config"`
	Fields   []IntegrationField `json:"fields"`
	Enabled  bool               `json:"enabled"`

	LastHeartbeat *string `json:"last_heartbeat"`
	LastError     string  `json:"last_error,omitempty"`

	// Справочные данные без возможности правки: версия кластера, канал и т.п.
	Info map[string]string `json:"info,omitempty"`

	// Ключи, которые нельзя отдавать наружу в открытом виде.
	secretKeys []string
	// Настройка живёт в другом сервисе — форму не показываем, только состояние.
	readOnly bool
}

type TestResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

type IntegrationRegistry struct {
	mu    sync.RWMutex
	items map[string]*Integration
	http  *http.Client
}

var integrations = NewIntegrationRegistry()

func NewIntegrationRegistry() *IntegrationRegistry {
	r := &IntegrationRegistry{
		items: make(map[string]*Integration),
		http:  &http.Client{Timeout: 8 * time.Second},
	}

	r.items["metrics"] = &Integration{
		Type: "metrics", Name: "Метрики", Category: "data_source",
		Enabled: true,
		Config: map[string]string{
			"kind":     env("METRICS_KIND", "prometheus"),
			"url":      env("PROMETHEUS_URL", "http://kps-kube-prometheus-stack-prometheus.monitoring:9090"),
			"username": env("METRICS_USER", ""),
			"password": env("METRICS_PASSWORD", ""),
		},
		Fields: []IntegrationField{
			{Key: "kind", Label: "Тип", Kind: "select", Required: true, Options: []Option{
				{Value: "prometheus", Label: "Prometheus API — Prometheus, VictoriaMetrics, Thanos, Mimir"},
				{Value: "zabbix", Label: "Zabbix API"},
			}},
			{Key: "url", Label: "URL", Kind: "text", Required: true, Hint: "адрес внутри кластера"},
			{Key: "username", Label: "Пользователь", Kind: "text"},
			{Key: "password", Label: "Пароль", Kind: "password"},
		},
		secretKeys: []string{"password"},
	}

	r.items["logs"] = &Integration{
		Type: "logs", Name: "Логи", Category: "data_source",
		Enabled: env("LOGS_STORE_URL", "") != "",
		Config: map[string]string{
			"kind":     env("LOGS_KIND", "opensearch"),
			"url":      env("LOGS_STORE_URL", ""),
			"username": env("LOGS_STORE_USER", ""),
			"password": env("LOGS_STORE_PASSWORD", ""),
			"index":    env("LOGS_STORE_INDEX", "logs-*"),
		},
		Fields: []IntegrationField{
			{Key: "kind", Label: "Тип", Kind: "select", Required: true, Options: []Option{
				{Value: "opensearch", Label: "OpenSearch / Elasticsearch API"},
				{Value: "loki", Label: "Loki API"},
			}},
			{Key: "url", Label: "URL", Kind: "text", Required: true, Hint: "например http://opensearch.logging.svc:9200"},
			{Key: "username", Label: "Пользователь", Kind: "text"},
			{Key: "password", Label: "Пароль", Kind: "password"},
			{Key: "index", Label: "Индекс", Kind: "text", Hint: "шаблон индекса, откуда брать логи"},
		},
		secretKeys: []string{"password"},
	}

	r.items["kubernetes"] = &Integration{
		Type: "kubernetes", Name: "Kubernetes", Category: "data_source",
		Enabled: true,
		Config:  map[string]string{"cluster": env("CLUSTER_NAME", "prod-cluster-1")},
		// Адрес и учётные данные берутся из service account пода — вводить
		// их руками негде и незачем, поэтому форма только для чтения.
		Fields:   []IntegrationField{},
		readOnly: true,
	}

	r.items["telegram"] = &Integration{
		Type: "telegram", Name: "Telegram", Category: "output_channel",
		Enabled: true,
		Config:  map[string]string{},
		// Токен и канал живут в окружении ai-worker — он единственный, кто пишет
		// в Telegram. Здесь только показываем состояние.
		Fields:   []IntegrationField{},
		readOnly: true,
	}

	for _, it := range r.items {
		it.Status = deriveStatus(it)
	}
	return r
}

// deriveStatus: пока проверку не запускали, судим по заполненности обязательных полей.
func deriveStatus(it *Integration) string {
	if it.readOnly {
		return "connected"
	}
	for _, f := range it.Fields {
		if f.Required && strings.TrimSpace(it.Config[f.Key]) == "" {
			return "not_configured"
		}
	}
	if !it.Enabled {
		return "not_configured"
	}
	return "connected"
}

// mask прячет секреты: наружу уходит длина и хвост, не само значение.
func mask(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "••••"
	}
	return strings.Repeat("•", 8) + v[len(v)-4:]
}

func (r *IntegrationRegistry) public(it *Integration) Integration {
	cp := *it
	cp.Config = make(map[string]string, len(it.Config))
	for k, v := range it.Config {
		cp.Config[k] = v
	}
	for _, k := range it.secretKeys {
		if _, ok := cp.Config[k]; ok {
			cp.Config[k] = mask(cp.Config[k])
		}
	}
	return cp
}

func (r *IntegrationRegistry) List() []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Integration, 0, len(r.items))
	for _, it := range r.items {
		out = append(out, r.public(it))
	}
	// Стабильный порядок: сперва источники, потом каналы, внутри — по имени.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (r *IntegrationRegistry) Save(typ string, cfg map[string]string, enabled *bool) (*Integration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.items[typ]
	if !ok {
		return nil, fmt.Errorf("unknown integration %q", typ)
	}
	if it.readOnly {
		return nil, fmt.Errorf("интеграция %q настраивается в другом сервисе", typ)
	}
	for k, v := range cfg {
		// Пустое значение секрета или маска — значит поле не трогали, сохраняем прежнее.
		if isSecret(it, k) && (v == "" || strings.HasPrefix(v, "••••")) {
			continue
		}
		it.Config[k] = v
	}
	if enabled != nil {
		it.Enabled = *enabled
	} else if allRequiredFilled(it) {
		// Форму заполнили — значит интеграцию хотят использовать. Иначе она
		// осталась бы «не настроена» с полным конфигом, что выглядит поломкой.
		it.Enabled = true
	}
	it.Status = deriveStatus(it)
	it.LastError = ""
	pub := r.public(it)
	return &pub, nil
}

func allRequiredFilled(it *Integration) bool {
	for _, f := range it.Fields {
		if f.Required && strings.TrimSpace(it.Config[f.Key]) == "" {
			return false
		}
	}
	return true
}

func isSecret(it *Integration, key string) bool {
	for _, k := range it.secretKeys {
		if k == key {
			return true
		}
	}
	return false
}

// Test дёргает интеграцию по-настоящему: без реального запроса кнопка «Проверить»
// не значит ничего.
func (r *IntegrationRegistry) Test(typ string) TestResult {
	r.mu.RLock()
	it, ok := r.items[typ]
	if !ok {
		r.mu.RUnlock()
		return TestResult{Success: false, Message: "интеграция не найдена"}
	}
	cfg := make(map[string]string, len(it.Config))
	for k, v := range it.Config {
		cfg[k] = v
	}
	r.mu.RUnlock()

	start := time.Now()
	var err error

	switch typ {
	case "metrics":
		base := strings.TrimRight(cfg["url"], "/")
		switch cfg["kind"] {
		case "zabbix":
			// У Zabbix нет health-эндпоинта, проверяем доступность самого API.
			err = r.probeHTTP(base+"/api_jsonrpc.php", cfg["username"], cfg["password"])
		default:
			// Prometheus-совместимые: у всех есть /-/healthy, а если нет —
			// пробуем сам API запросов.
			if e := r.probeHTTP(base+"/-/healthy", cfg["username"], cfg["password"]); e != nil {
				err = r.probeHTTP(base+"/api/v1/query?query=up", cfg["username"], cfg["password"])
			}
		}
	case "logs":
		if cfg["url"] == "" {
			err = fmt.Errorf("не задан URL")
		} else if cfg["kind"] == "loki" {
			err = r.probeHTTP(strings.TrimRight(cfg["url"], "/")+"/ready", cfg["username"], cfg["password"])
		} else {
			err = r.probeHTTP(strings.TrimRight(cfg["url"], "/")+"/", cfg["username"], cfg["password"])
		}
	case "kubernetes":
		err = probeKubernetes()
	case "telegram":
		err = probeTelegramViaAIWorker()
	default:
		err = fmt.Errorf("проверка для %s не реализована", typ)
	}

	latency := time.Since(start).Milliseconds()

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		it.Status = "disconnected"
		it.LastError = err.Error()
		return TestResult{Success: false, Message: err.Error(), LatencyMS: latency}
	}
	now := time.Now().Format(time.RFC3339)
	it.Status = "connected"
	it.LastHeartbeat = &now
	it.LastError = ""
	if info := r.refreshInfo(typ); len(info) > 0 {
		it.Info = info
	}
	return TestResult{Success: true, Message: fmt.Sprintf("ответ получен за %d мс", latency), LatencyMS: latency}
}

func (r *IntegrationRegistry) probeHTTP(url, user, pass string) error {
	if url == "" {
		return fmt.Errorf("не задан адрес")
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("неверный адрес: %w", err)
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("нет связи: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ответ %d", resp.StatusCode)
	}
	return nil
}

// kubeAPI дергает API кластера сервисным аккаунтом пода и разбирает JSON.
func kubeAPI(path string) (map[string]any, error) {
	const saDir = "/var/run/secrets/kubernetes.io/serviceaccount"

	ca, err := os.ReadFile(saDir + "/ca.crt")
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("не удалось прочитать CA кластера")
	}
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
	req, err := http.NewRequest("GET", "https://kubernetes.default.svc"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API кластера ответил %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// probeKubernetes ходит в API кластера с CA и токеном service account'а —
// сертификат кластера подписан своим CA, системному доверию он не известен.
func probeKubernetes() error {
	const saDir = "/var/run/secrets/kubernetes.io/serviceaccount"

	ca, err := os.ReadFile(saDir + "/ca.crt")
	if err != nil {
		return fmt.Errorf("нет доступа к service account: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return fmt.Errorf("не удалось прочитать CA кластера")
	}
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return fmt.Errorf("нет токена service account: %w", err)
	}

	client := &http.Client{
		Timeout:   8 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
	req, err := http.NewRequest("GET", "https://kubernetes.default.svc/version", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("нет связи с API кластера: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API кластера ответил %d", resp.StatusCode)
	}
	return nil
}

// probeTelegramViaAIWorker: сами в Telegram не ходим — токена у нас нет и быть
// не должно. Спрашиваем ai-worker, настроен ли у него канал.
func probeTelegramViaAIWorker() error {
	st, err := fetchAIWorkerStatus()
	if err != nil {
		return err
	}
	tg, ok := st["telegram"].(map[string]any)
	if !ok {
		return fmt.Errorf("ai-worker не сообщил статус Telegram")
	}
	if configured, _ := tg["channel_configured"].(bool); !configured {
		return fmt.Errorf("канал не настроен в ai-worker")
	}
	return nil
}

// refreshInfo дособирает справочные данные интеграции. Вызывается при проверке:
// «имя кластера» в настройках ничего не говорит о том, работает ли доступ.
func (r *IntegrationRegistry) refreshInfo(typ string) map[string]string {
	switch typ {
	case "kubernetes":
		return kubernetesInfo()
	case "telegram":
		return telegramInfo()
	}
	return nil
}

func kubernetesInfo() map[string]string {
	info := map[string]string{"api": "https://kubernetes.default.svc"}

	if v, err := kubeAPI("/version"); err == nil {
		if s, ok := v["gitVersion"].(string); ok {
			info["Версия"] = s
		}
		if s, ok := v["platform"].(string); ok {
			info["Платформа"] = s
		}
	}
	if v, err := kubeAPI("/api/v1/nodes?limit=500"); err == nil {
		if items, ok := v["items"].([]any); ok {
			info["Нод в кластере"] = strconv.Itoa(len(items))
		}
	}
	if ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		info["Свой namespace"] = strings.TrimSpace(string(ns))
	}
	return info
}

func telegramInfo() map[string]string {
	st, err := fetchAIWorkerStatus()
	if err != nil {
		return map[string]string{"Состояние": "нет связи с ai-worker"}
	}
	info := map[string]string{}
	if tg, ok := st["telegram"].(map[string]any); ok {
		if v, ok := tg["channel_id"].(string); ok && v != "" {
			info["Канал"] = v
		}
		if v, ok := tg["channel_title"].(string); ok && v != "" {
			info["Название канала"] = v
		}
		if v, ok := tg["parse_mode"].(string); ok && v != "" {
			info["Формат разметки"] = v
		}
		if v, ok := tg["channel_configured"].(bool); ok {
			if v {
				info["Канал настроен"] = "да"
			} else {
				info["Канал настроен"] = "нет"
			}
		}
	}
	info["Управляется"] = "ai-worker"
	return info
}
