package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//
// ====== МОДЕЛИ ДАННЫХ ======
//

// EnrichedAlert — то, что присылает Data Enricher.
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

	PromSample map[string]string `json:"prom_sample,omitempty"`
	K8sContext map[string]string `json:"k8s_context,omitempty"`

	GrafanaLinks  []string `json:"grafana_links,omitempty"`
	ExtraNotes    []string `json:"extra_notes,omitempty"`
	IncidentHints []string `json:"incident_hints,omitempty"`

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

	PrimaryMetric  string `json:"primary_metric,omitempty"`
	PrimarySummary string `json:"primary_summary,omitempty"`
}

// IncidentResponse — ответ ai-worker'а для enricher'а.
type IncidentResponse struct {
	Status    string `json:"status"`
	MessageID int64  `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

//
// ====== КОНФИГ / ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ======
//

type Config struct {
	ListenAddr string

	TGBotToken      string
	TGChannelID     string // канал, куда летит алерт
	TGThreadChatID  string // чат обсуждения, куда летит AI-ответ
	TGParseMode     string

	ORAPIKey    string
	ORBaseURL   string
	ORModel     string
	ORTimeout   time.Duration
	ORMaxTokens int
}

var (
	appCfg     Config
	httpClient = &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			TLSNextProto:      make(map[string]func(string, *tls.Conn) http.RoundTripper),
			DisableKeepAlives: true,
		},
	}
	// llmSemaphore ограничивает конкурентность LLM-вызовов при шторме алертов.
	llmSemaphore = make(chan struct{}, 8)
	// activeTasks отслеживает незавершённые фоновые задачи для graceful shutdown.
	activeTasks sync.WaitGroup
)

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envMust(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("required env %s is not set", key)
	}
	return v
}

func envIntOr(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("WARN: invalid int in %s=%q, using default %d", key, raw, def)
		return def
	}
	return n
}

func loadConfig() Config {
	channelID := envMust("TELEGRAM_CHANNEL_ID")
	threadChatID := envOr("TELEGRAM_THREAD_CHAT_ID", "")
	if threadChatID == "" {
		// если не задано отдельно — шлём комментарий в тот же чат
		threadChatID = channelID
	}

	cfg := Config{
		ListenAddr: envOr("LISTEN_ADDR", ":8080"),

		TGBotToken:     envMust("TELEGRAM_BOT_TOKEN"),
		TGChannelID:    channelID,
		TGThreadChatID: threadChatID,
		TGParseMode:    envOr("TELEGRAM_PARSE_MODE", "Markdown"),

		ORAPIKey:    envMust("OPENROUTER_API_KEY"),
		ORBaseURL:   envOr("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1/chat/completions"),
		ORModel:     envOr("OPENROUTER_MODEL", "google/gemini-2.5-flash"),
		ORTimeout:   time.Duration(envIntOr("OPENROUTER_TIMEOUT_SECONDS", 300)) * time.Second,
		ORMaxTokens: envIntOr("OPENROUTER_MAX_TOKENS", 600),
	}
	return cfg
}

//
// ====== MAIN ======
//

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	appCfg = loadConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/incident", incidentHandler)
	mux.HandleFunc("/incident-raw", incidentRawHandler)
	mux.HandleFunc("/incident-group", incidentGroupHandler)

	srv := &http.Server{
		Addr:              appCfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("ai-worker starting on %s", appCfg.ListenAddr)
	log.Printf("Telegram channel_id=%s, thread_chat_id=%s, parse_mode=%s",
		appCfg.TGChannelID, appCfg.TGThreadChatID, appCfg.TGParseMode)
	log.Printf("OpenRouter model=%s, timeout=%s, max_tokens=%d",
		appCfg.ORModel, appCfg.ORTimeout, appCfg.ORMaxTokens)

	// Graceful shutdown: ловим SIGTERM/SIGINT, ждём завершения фоновых задач.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("INFO: received signal %v, shutting down (waiting for %d active tasks)", sig, activeTasksCount())

		shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutCancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			log.Printf("ERROR: server shutdown: %v", err)
		}
		// Ждём завершения фоновых горутин (LLM + Telegram), чтобы не терять разбор.
		activeTasks.Wait()
		log.Printf("INFO: all background tasks completed, exiting")
	}()
	
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func activeTasksCount() int {
	// Приблизительная оценка для лога; точное число — в WaitGroup.
	return len(llmSemaphore)
}

//
// ====== HTTP HANDLERS ======
//

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// incidentHandler — точка входа от Data Enricher.
func incidentHandler(w http.ResponseWriter, r *http.Request) {
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

	var alert EnrichedAlert
	if err := json.Unmarshal(body, &alert); err != nil {
		log.Printf("ERROR: cannot unmarshal alert: %v", err)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	if os.Getenv("DEBUG_ENRICHED") == "1" {
    pretty, _ := json.MarshalIndent(alert, "", "  ")
    log.Printf("DEBUG: enriched alert from enricher:\n%s", string(pretty))
	}
	
	log.Printf("INFO: got incident alert=%s severity=%s status=%s",
		alert.Labels["alertname"], alert.Labels["severity"], alert.Status)

	processSingleAlert(w, r.Context(), alert)
}

// processSingleAlert — общая логика для одиночного алерта: пост сырого алерта в
// канал, ответ Enricher'у и асинхронный AI-разбор реплаем. Используется как
// /incident, так и /incident-group при len(alerts)==1.
func processSingleAlert(w http.ResponseWriter, ctx context.Context, alert EnrichedAlert) {
	// 1) Текст алерта
	alertText := buildAlertMessage(alert)

	// 2) Отправляем пост в КАНАЛ (быстрый запрос, контекст берём из r)
	msgID, err := sendTelegramMessage(ctx, appCfg.TGChannelID, alertText, 0)
	if err != nil {
		log.Printf("ERROR: send alert to telegram failed: %v", err)
		writeJSON(w, http.StatusBadGateway, IncidentResponse{
			Status: "telegram_error",
			Error:  err.Error(),
		})
		return
	}
	log.Printf("INFO: alert posted to telegram channel message_id=%d", msgID)

	// 3) Сразу отвечаем Enricher'у — он свою работу сделал, дальше мы сами
	writeJSON(w, http.StatusOK, IncidentResponse{
		Status:    "accepted",
		MessageID: msgID,
	})

	// 4) Resolved-алерты не анализируем — проблема уже закрыта, LLM не нужен.
	//    Постим только [RESOLVED]-уведомление (уже отправлено выше) и выходим.
	if strings.EqualFold(alert.Status, "resolved") {
		log.Printf("INFO: alert=%s status=resolved, skip AI analysis", alert.Labels["alertname"])
		return
	}

	// 5) Асинхронно делаем OpenRouter + ответ в тред (комментарий к посту в КАНАЛЕ).
	// Используем bounded semaphore для защиты от шторма алертов.
	activeTasks.Add(1)
	go func(alert EnrichedAlert, parentMsgID int64) {
		defer activeTasks.Done()

		// Ограничиваем конкурентность LLM-вызовов.
		llmSemaphore <- struct{}{}
		defer func() { <-llmSemaphore }()

		// Отдельный контекст для OpenRouter, не зависящий от HTTP-запроса Enricher-а
		aiCtx, cancelAI := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancelAI()

		prompt := buildPromptFromAlert(alert)

		if os.Getenv("DEBUG_PROMPT") == "1" {
			log.Printf("DEBUG: OpenRouter prompt for message_id=%d:\n%s", parentMsgID, prompt)
		}

		aiText, err := callOpenRouter(aiCtx, prompt)
		if err != nil {
			log.Printf("ERROR: openrouter call failed for message_id=%d: %v", parentMsgID, err)
			return
		}

		// Отдельный контекст для отправки ответа в Telegram
		replyCtx, cancelReply := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelReply()

		if err := sendTelegramChunked(replyCtx, aiText, parentMsgID); err != nil {
			log.Printf("ERROR: send AI reply to telegram failed for message_id=%d: %v", parentMsgID, err)
			return
		}

		log.Printf("INFO: AI reply sent as comment for channel message_id=%d", parentMsgID)
	}(alert, msgID)
}

// incidentRawHandler — постит СЫРОЙ алерт в канал сразу, БЕЗ вызова LLM.
// Live-фид: инженер видит каждый алерт мгновенно, а AI-разбор приходит позже
// отдельным сообщением через /incident-group (после окна корреляции).
func incidentRawHandler(w http.ResponseWriter, r *http.Request) {
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

	var alert EnrichedAlert
	if err := json.Unmarshal(body, &alert); err != nil {
		log.Printf("ERROR: cannot unmarshal raw alert: %v", err)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	msgID, err := sendTelegramMessage(r.Context(), appCfg.TGChannelID, buildAlertMessage(alert), 0)
	if err != nil {
		log.Printf("ERROR: send raw alert to telegram failed: %v", err)
		writeJSON(w, http.StatusBadGateway, IncidentResponse{Status: "telegram_error", Error: err.Error()})
		return
	}
	log.Printf("INFO: raw alert posted alert=%s status=%s message_id=%d",
		alert.Labels["alertname"], alert.Status, msgID)
	writeJSON(w, http.StatusOK, IncidentResponse{Status: "accepted", MessageID: msgID})
}

// GroupRequest — запрос на разбор группы связанных алертов.
type GroupRequest struct {
	GroupKey string          `json:"group_key"`
	Window   string          `json:"window"`
	Alerts   []EnrichedAlert `json:"alerts"`
}

// incidentGroupHandler — точка входа для ГРУППЫ связанных алертов.
func incidentGroupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req GroupRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("ERROR: cannot unmarshal group: %v", err)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	if len(req.Alerts) == 0 {
		http.Error(w, "empty alerts", http.StatusBadRequest)
		return
	}

	log.Printf("INFO: got incident group key=%s window=%s alerts=%d",
		req.GroupKey, req.Window, len(req.Alerts))

	// Для группы (>1) постим СВЯЗКУ — сводное сообщение "N связанных алертов",
	// оно визуально связывает сырые алерты, уже показанные по одному через /incident-raw.
	// AI-разбор потом уйдёт реплаем на эту сводку.
	var parentMsgID int64
	if len(req.Alerts) > 1 {
		msgID, err := sendTelegramMessage(r.Context(), appCfg.TGChannelID, buildGroupSummaryMessage(req), 0)
		if err != nil {
			log.Printf("ERROR: send group summary to telegram failed: %v", err)
			writeJSON(w, http.StatusBadGateway, IncidentResponse{Status: "telegram_error", Error: err.Error()})
			return
		}
		parentMsgID = msgID
		log.Printf("INFO: group summary posted message_id=%d", msgID)
	}

	writeJSON(w, http.StatusOK, IncidentResponse{Status: "accepted", MessageID: parentMsgID})

	// Если ВСЕ алерты resolved — разбирать нечего (сырьё/RESOLVED уже показано).
	allResolved := true
	for _, a := range req.Alerts {
		if !strings.EqualFold(a.Status, "resolved") {
			allResolved = false
			break
		}
	}
	if allResolved {
		log.Printf("INFO: group key=%s all resolved, skip AI analysis", req.GroupKey)
		return
	}

	// Асинхронно: один вызов LLM → AI-разбор. Для группы — реплаем на сводку-связку,
	// для одиночного — отдельным сообщением. Bounded semaphore защищает от OOM.
	activeTasks.Add(1)
	go func(req GroupRequest, parentMsgID int64) {
		defer activeTasks.Done()

		llmSemaphore <- struct{}{}
		defer func() { <-llmSemaphore }()

		aiCtx, cancelAI := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancelAI()

		var sysPrompt, prompt, header string
		if len(req.Alerts) == 1 {
			sysPrompt = systemPrompt
			prompt = buildPromptFromAlert(req.Alerts[0])
			header = fmt.Sprintf("📊 *Разбор алерта* (%s)\n\n", req.GroupKey)
		} else {
			sysPrompt = groupSystemPrompt
			prompt = buildGroupPrompt(req)
		}

		if os.Getenv("DEBUG_PROMPT") == "1" {
			log.Printf("DEBUG: OpenRouter group prompt (%d alerts):\n%s", len(req.Alerts), prompt)
		}

		aiText, err := callOpenRouterWithSystem(aiCtx, sysPrompt, prompt)
		if err != nil {
			log.Printf("ERROR: openrouter group call failed: %v", err)
			return
		}

		sendCtx, cancelSend := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelSend()

		if parentMsgID != 0 {
			// группа: разбор комментом к сводке-связке (чанками)
			if err := sendTelegramChunked(sendCtx, aiText, parentMsgID); err != nil {
				log.Printf("ERROR: send group analysis reply failed: %v", err)
				return
			}
		} else {
			// одиночный: разбор отдельным сообщением (чанками)
			if _, err := sendTelegramMessage(sendCtx, appCfg.TGChannelID, header+aiText, 0); err != nil {
				log.Printf("ERROR: send single analysis failed: %v", err)
				return
			}
		}
		log.Printf("INFO: group analysis posted key=%s alerts=%d", req.GroupKey, len(req.Alerts))
	}(req, parentMsgID)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

//
// ====== TELEGRAM ======
//

func sendTelegramMessage(ctx context.Context, chatID, text string, replyTo int64) (int64, error) {
	if appCfg.TGBotToken == "" || chatID == "" {
		return 0, fmt.Errorf("telegram config is not set")
	}
	text = escapeMarkdown(text)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": appCfg.TGParseMode,
	}
	if replyTo != 0 {
		payload["reply_to_message_id"] = replyTo
		payload["allow_sending_without_reply"] = true
	}

	body, _ := json.Marshal(payload)
	log.Printf("DEBUG: telegram sendMessage payload=%s", string(body))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", appCfg.TGBotToken),
		bytes.NewReader(body),
	)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return 0, fmt.Errorf("telegram HTTP %d: %s", resp.StatusCode, string(b))
	}

	var tgResp struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return 0, err
	}
	if !tgResp.OK {
		return 0, fmt.Errorf("telegram response not ok")
	}
	return tgResp.Result.MessageID, nil
}


func sendTelegramReply(ctx context.Context, text string, replyTo int64) error {
	// Отвечаем в канал (TGChannelID). Комментарий автоматически появится
	// и в привязанной группе обсуждения (TGThreadChatID), если она настроена.
	_, err := sendTelegramMessage(ctx, appCfg.TGChannelID, text, replyTo)
	return err
}

// Простой escape для Markdown (не MarkdownV2).
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)
	return replacer.Replace(s)
}

//
// ====== OPENROUTER ======
//

// systemPrompt — системный промпт для разбора ОДИНОЧНОГО алерта.
const systemPrompt = `Ты — опытный SRE/DevOps-инженер, анализирующий инциденты в Kubernetes.
Тебе приходят обогащённые данные алерта: метрики Prometheus с трендами, контекст Kubernetes API (статус подов, события, состояние нод, здоровье namespace) и аннотации.

Формат ответа:
**Первопричина:** 1-2 предложения. Что именно произошло и почему, со ссылкой на конкретные данные (тренды метрик, статус подов, события).
**Исправление:** 2-3 конкретных шага. Используй команды kubectl с реальными namespace/pod/node из алерта.
**Профилактика:** 1-2 практических совета (лимиты ресурсов, HPA, тюнинг алертов и т.п.).

Правила:
- Если видишь recent_deployment=true — рассматривай это как возможную причину.
- Если pod_restarts > 0 или pod_waiting_reason содержит OOMKilled/CrashLoopBackOff — сфокусируйся на этом.
- Если node_memory_pressure=True или node_disk_pressure=True — упомяни проблемы на уровне ноды.
- Если тренд метрики "rising" или "spike" — подчеркни направление тренда.
- Ответ — не длиннее 250 слов. Не повторяй сырые значения метрик.`

// groupSystemPrompt — системный промпт для разбора ГРУППЫ связанных алертов.
const groupSystemPrompt = `Тебе даны N связанных алертов из одного namespace за короткое окно. Определи наиболее вероятный КОРЕНЬ (что упало и вызвало остальное) и СЛЕДСТВИЯ. Учитывай, что часть алертов может быть не связана — отбрось шум. Формат: **Корень:** … **Цепочка:** … **Исправление:** … **Профилактика:** … Не длиннее 300 слов. Это гипотеза, финальное решение за инженером.`

func callOpenRouter(ctx context.Context, prompt string) (string, error) {
	return callOpenRouterWithSystem(ctx, systemPrompt, prompt)
}

func callOpenRouterWithSystem(ctx context.Context, sysPrompt, prompt string) (string, error) {
	if appCfg.ORAPIKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY is not set")
	}

	type ORMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	payload := map[string]interface{}{
		"model":      appCfg.ORModel,
		"max_tokens": appCfg.ORMaxTokens,
		"messages": []ORMessage{
			{
				Role:    "system",
				Content: sysPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	body, _ := json.Marshal(payload)
	ctx2, cancel := context.WithTimeout(ctx, appCfg.ORTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx2, http.MethodPost, appCfg.ORBaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	if os.Getenv("DEBUG_OR_PAYLOAD") == "1" {
    log.Printf("DEBUG: OpenRouter request payload:\n%s", string(body))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+appCfg.ORAPIKey)
	req.Header.Set("Connection", "close")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", fmt.Errorf("openrouter HTTP %d: %s", resp.StatusCode, string(b))
	}

	var orResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return "", err
	}
	if len(orResp.Choices) == 0 {
		return "", fmt.Errorf("openrouter: empty choices")
	}
	return orResp.Choices[0].Message.Content, nil
}

//
// ====== ПОСТРОЕНИЕ ТЕКСТОВ ======
//

// buildAlertMessage рендерит СЫРОЙ алерт максимально близко к тому, что отдаёт
// Alertmanager: заголовок [STATUS] alertname (severity), затем все labels и
// annotations списком, плюс время старта и generatorURL. Никакого обогащения —
// это «до» в связке до/после (обогащённый разбор уходит отдельным AI-реплаем).
func buildAlertMessage(a EnrichedAlert) string {
	var b strings.Builder

	status := strings.ToUpper(a.Status)
	if status == "" {
		status = "FIRING"
	}
	emoji := "🔥"
	if strings.EqualFold(a.Status, "resolved") {
		emoji = "✅"
	}
	name := a.Labels["alertname"]
	if name == "" {
		name = "UnknownAlert"
	}
	if severity := firstNonEmpty(a.Labels["severity"], a.Severity); severity != "" {
		fmt.Fprintf(&b, "%s [%s] %s (%s)\n", emoji, status, name, severity)
	} else {
		fmt.Fprintf(&b, "%s [%s] %s\n", emoji, status, name)
	}

	if len(a.Labels) > 0 {
		b.WriteString("\nLabels:\n")
		keys := make([]string, 0, len(a.Labels))
		for k := range a.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, " - %s = %s\n", k, a.Labels[k])
		}
	}

	if len(a.Annotations) > 0 {
		b.WriteString("\nAnnotations:\n")
		keys := make([]string, 0, len(a.Annotations))
		for k := range a.Annotations {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, " - %s = %s\n", k, a.Annotations[k])
		}
	}

	if a.StartsAt.Unix() > 0 {
		fmt.Fprintf(&b, "\nStarts: %s\n", a.StartsAt.UTC().Format(time.RFC3339))
	}
	if a.GeneratorURL != "" {
		fmt.Fprintf(&b, "Source: %s\n", a.GeneratorURL)
	}

	return b.String()
}

func buildPromptFromAlert(a EnrichedAlert) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Enriched Kubernetes alert. Analyze using all data below.\n\n")

	// === Alert identity ===
	fmt.Fprintf(&b, "=== Alert ===\n")
	fmt.Fprintf(&b, "alertname: %s\n", a.Labels["alertname"])
	fmt.Fprintf(&b, "severity: %s\n", firstNonEmpty(a.Labels["severity"], a.Severity))
	fmt.Fprintf(&b, "status: %s\n", a.Status)
	fmt.Fprintf(&b, "cluster: %s\n", a.ClusterName)
	fmt.Fprintf(&b, "environment: %s\n", a.Environment)
	if a.StartsAt.Unix() > 0 {
		fmt.Fprintf(&b, "starts_at: %s\n", a.StartsAt.UTC().Format(time.RFC3339))
	}
	if a.EndsAt.Unix() > 0 {
		fmt.Fprintf(&b, "ends_at: %s\n", a.EndsAt.UTC().Format(time.RFC3339))
	}
	if a.Owner != "" {
		fmt.Fprintf(&b, "owner: %s\n", a.Owner)
	}
	fmt.Fprintf(&b, "\n")

	// === Annotations ===
	if len(a.Annotations) > 0 {
		fmt.Fprintf(&b, "=== Annotations ===\n")
		for k, v := range a.Annotations {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&b, "\n")
	}

	// === K8s Context (full map from enricher) ===
	if len(a.K8sContext) > 0 {
		fmt.Fprintf(&b, "=== Kubernetes Context ===\n")
		// Сортируем ключи для детерминированного вывода
		keys := make([]string, 0, len(a.K8sContext))
		for k := range a.K8sContext {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v := a.K8sContext[k]; v != "" {
				fmt.Fprintf(&b, "%s: %s\n", k, v)
			}
		}
		fmt.Fprintf(&b, "\n")
	} else {
		// Fallback: берём из labels если K8sContext пустой
		ns := a.Labels["namespace"]
		pod := a.Labels["pod"]
		node := firstNonEmpty(a.Labels["node"], a.Labels["instance"])
		if ns != "" || pod != "" || node != "" {
			fmt.Fprintf(&b, "=== Kubernetes Context (from labels) ===\n")
			if ns != "" {
				fmt.Fprintf(&b, "namespace: %s\n", ns)
			}
			if pod != "" {
				fmt.Fprintf(&b, "pod: %s\n", pod)
			}
			if c := a.Labels["container"]; c != "" {
				fmt.Fprintf(&b, "container: %s\n", c)
			}
			if node != "" {
				fmt.Fprintf(&b, "node: %s\n", node)
			}
			fmt.Fprintf(&b, "\n")
		}
	}

	// === Prometheus Metrics ===
	if len(a.PromSample) > 0 {
		fmt.Fprintf(&b, "=== Prometheus Metrics ===\n")
		if a.PrimaryMetric != "" {
			fmt.Fprintf(&b, "primary_metric: %s\n", a.PrimaryMetric)
		}
		if a.PrimarySummary != "" {
			fmt.Fprintf(&b, "primary_summary: %s\n", a.PrimarySummary)
		}
		keys := make([]string, 0, len(a.PromSample))
		for k := range a.PromSample {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v := a.PromSample[k]; v != "" {
				fmt.Fprintf(&b, "%s: %s\n", k, v)
			}
		}
		fmt.Fprintf(&b, "\n")
	}

	// === Grafana Links ===
	if len(a.GrafanaLinks) > 0 {
		fmt.Fprintf(&b, "=== Grafana ===\n")
		for _, link := range a.GrafanaLinks {
			fmt.Fprintf(&b, "- %s\n", link)
		}
		fmt.Fprintf(&b, "\n")
	}

	// === Extra Notes / Hints ===
	if len(a.ExtraNotes) > 0 {
		fmt.Fprintf(&b, "=== Notes ===\n")
		for _, n := range a.ExtraNotes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(a.IncidentHints) > 0 {
		fmt.Fprintf(&b, "=== Diagnostic Hints ===\n")
		for _, h := range a.IncidentHints {
			fmt.Fprintf(&b, "- %s\n", h)
		}
		fmt.Fprintf(&b, "\n")
	}

	fmt.Fprintf(&b, "Provide a structured incident analysis per the system prompt instructions.\n")

	return b.String()
}

// buildGroupSummaryMessage рендерит «сводный» пост для группы: заголовок с
// числом алертов и namespace, затем список сырых заголовков каждого алерта
// (alertname + severity).
func buildGroupSummaryMessage(req GroupRequest) string {
	var b strings.Builder

	firing := 0
	for _, a := range req.Alerts {
		if !strings.EqualFold(a.Status, "resolved") {
			firing++
		}
	}
	status := "FIRING"
	emoji := "🔥"
	if firing == 0 {
		status = "RESOLVED"
		emoji = "✅"
	}

	fmt.Fprintf(&b, "%s [%s] %d связанных алертов (%s)\n", emoji, status, len(req.Alerts), req.GroupKey)
	b.WriteString("\n")

	for i, a := range req.Alerts {
		name := a.Labels["alertname"]
		if name == "" {
			name = "UnknownAlert"
		}
		st := strings.ToUpper(a.Status)
		if st == "" {
			st = "FIRING"
		}

		line := fmt.Sprintf("%d. [%s] %s", i+1, st, name)
		if severity := firstNonEmpty(a.Labels["severity"], a.Severity); severity != "" {
			line += " (" + severity + ")"
		}
		// Без объекта два разных алерта с одним alertname выглядят одинаково.
		if subj := alertSubject(a); subj != "" {
			line += " → " + subj
		}
		fmt.Fprintf(&b, "%s\n", line)

		if s := a.Annotations["summary"]; s != "" {
			fmt.Fprintf(&b, "   %s\n", s)
		}
	}

	return b.String()
}

// alertSubject возвращает, ЧТО именно затронуто (deployment=orders-db, pod=..., node=...).
// Имя алерта само по себе не говорит, какой объект упал — берём первый содержательный лейбл.
func alertSubject(a EnrichedAlert) string {
	for _, k := range []string{
		"deployment", "statefulset", "daemonset", "job",
		"service", "pod", "container", "node", "instance",
	} {
		if v := a.Labels[k]; v != "" {
			return k + "=" + v
		}
	}
	return ""
}

// buildGroupPrompt склеивает user-сообщения по каждому алерту группы с
// разделителями === Alert i/N ===.
func buildGroupPrompt(req GroupRequest) string {
	var b strings.Builder

	n := len(req.Alerts)
	fmt.Fprintf(&b, "Группа из %d связанных алертов. group_key=%s, window=%s.\n\n", n, req.GroupKey, req.Window)

	for i, a := range req.Alerts {
		fmt.Fprintf(&b, "=== Alert %d/%d ===\n", i+1, n)
		b.WriteString(buildPromptFromAlert(a))
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Проанализируй группу целиком по инструкциям системного промпта.\n")

	return b.String()
}

// sendTelegramChunked разбивает текст на чанки ≤4096 символов (лимит Telegram API)
// и отправляет их как reply-цепочку (первый ответ на parentMsgID, остальные друг за другом).
func sendTelegramChunked(ctx context.Context, text string, parentMsgID int64) error {
	const maxLen = 4000 // запас под parse_mode overhead

	if len(text) <= maxLen {
		return sendTelegramReply(ctx, text, parentMsgID)
	}

	// Используем руны для безопасного деления по границам символов.
	runes := []rune(text)
	prevMsgID := parentMsgID

	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		msgID, err := sendTelegramMessage(ctx, appCfg.TGChannelID, chunk, prevMsgID)
		if err != nil {
			return fmt.Errorf("chunk %d/%d (parent=%d): %w", i/maxLen+1, (len(runes)+maxLen-1)/maxLen, prevMsgID, err)
		}
		prevMsgID = msgID
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
