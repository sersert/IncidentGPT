package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisClient — клиент для лёгкой корреляции. nil, если Redis недоступен (фолбэк на поштучную отправку).
var redisClient *redis.Client

// groupTimers — отслеживает активные debounce-таймеры по ключам групп,
// чтобы для одной группы не заводить несколько таймеров.
var groupTimers = struct {
	mu     sync.Mutex
	active map[string]bool
}{active: make(map[string]bool)}

// groupBatch — тело запроса на /incident-group.
type groupBatch struct {
	GroupKey string            `json:"group_key"`
	Window   string            `json:"window"`
	Alerts   []json.RawMessage `json:"alerts"`
}

// initRedis подключается к Redis. При ошибке redisClient остаётся nil (корреляция отключена).
func initRedis() error {
	client := redis.NewClient(&redis.Options{
		Addr:     appCfg.RedisAddr,
		Password: appCfg.RedisPassword,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	redisClient = client
	return nil
}

// groupKeyFor возвращает namespace для группировки ("_nolabel" если пусто).
func groupKeyFor(e EnrichedAlert) string {
	if ns := e.Labels["namespace"]; ns != "" {
		return ns
	}
	return "_nolabel"
}

// bufferAlert кладёт обогащённый алерт в Redis-список grp:{namespace} с TTL CORR_WINDOW.
// На первом алерте группы заводит debounce-таймер на CORR_SETTLE.
// Возвращает ошибку, если Redis недоступен — тогда вызывающий делает фолбэк на sendToBackend.
// errSkipResolved — алерт зарезолвился: в группу «связанных инцидентов» его класть незачем,
// вызывающий отправит его поштучно как [RESOLVED]-уведомление.
var errSkipResolved = fmt.Errorf("resolved alert, not buffered")

func bufferAlert(e EnrichedAlert) error {
	if redisClient == nil {
		return fmt.Errorf("redis not configured")
	}

	gk := groupKeyFor(e)
	key := "grp:" + gk

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Resolved-алерт: убираем его из пачки (если он там висел как firing) и не корреллируем.
	if strings.EqualFold(e.Status, "resolved") {
		redisClient.HDel(ctx, key, e.Fingerprint)
		log.Printf("ALERT_RESOLVED_SKIP: fingerprint=%s group=%s", e.Fingerprint, gk)
		return errSkipResolved
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal enriched alert: %w", err)
	}

	// HSet, а не RPush: ключ поля — fingerprint, поэтому повторный приход того же
	// алерта (firing → resolved → firing) перезаписывает запись, а не плодит дубли.
	if err := redisClient.HSet(ctx, key, e.Fingerprint, data).Err(); err != nil {
		return fmt.Errorf("redis hset: %w", err)
	}
	redisClient.Expire(ctx, key, appCfg.CorrWindow)

	// Live-фид: сразу постим сырой алерт (не дожидаясь окна), чтобы инженер видел
	// поток в реальном времени. AI-разбор группы придёт позже через /incident-group.
	go func(e EnrichedAlert) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("ERROR: panic in sendRawToBackend for fingerprint=%s: %v", e.Fingerprint, r)
			}
		}()
		sendRawToBackend(e)
	}(e)

	// Debounce: заводим таймер только для первого алерта группы.
	groupTimers.mu.Lock()
	if !groupTimers.active[key] {
		groupTimers.active[key] = true
		time.AfterFunc(appCfg.CorrSettle, func() { flushGroup(key, gk) })
	}
	groupTimers.mu.Unlock()

	log.Printf("ALERT_BUFFERED: fingerprint=%s group=%s key=%s", e.Fingerprint, gk, key)
	return nil
}

// flushGroup вызывается по срабатыванию debounce-таймера: читает все алерты группы,
// очищает ключ и отправляет их одной пачкой на /incident-group.
func flushGroup(key, groupKey string) {
	groupTimers.mu.Lock()
	delete(groupTimers.active, key)
	groupTimers.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	// HGetAll: поля — fingerprint'ы, значения — обогащённые алерты. Дубликатов быть не может.
	vals, err := redisClient.HGetAll(ctx, key).Result()
	if err != nil {
		log.Printf("ERROR: corr flush HGetAll %s failed: %v", key, err)
		return
	}

	if len(vals) == 0 {
		return
	}

	alerts := make([]json.RawMessage, 0, len(vals))
	for _, v := range vals {
		alerts = append(alerts, json.RawMessage(v))
	}

	// Отправляем пакет ДО удаления ключа из Redis.
	// Если отправка упадёт — группа останется в Redis и будет доставлена
	// при следующем вызове flushGroup или по истечении TTL.
	if err := sendGroupToBackend(ctx, groupKey, alerts); err != nil {
		log.Printf("ERROR: failed to send incident group to backend: %v", err)
		return
	}

	// Только после успешной отправки удаляем группу из Redis.
	redisClient.Del(ctx, key)
}

// sendRawToBackend постит один сырой алерт на RAW_BACKEND_URL (/incident-raw) —
// для live-фида: алерт виден в канале сразу, не дожидаясь окна корреляции.
func sendRawToBackend(e EnrichedAlert) {
	if appCfg.RawBackendURL == "" {
		return
	}
	data, err := json.Marshal(e)
	if err != nil {
		log.Printf("WARN: raw marshal failed: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appCfg.RawBackendURL, bytes.NewReader(data))
	if err != nil {
		log.Printf("WARN: raw request build failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("WARN: raw post failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		log.Printf("WARN: raw backend HTTP %d: %s", resp.StatusCode, string(body))
	}
}

// sendGroupToBackend отправляет пачку алертов на GROUP_BACKEND_URL.
func sendGroupToBackend(ctx context.Context, groupKey string, alerts []json.RawMessage) error {
	if appCfg.GroupBackendURL == "" {
		log.Printf("INFO: GROUP_BACKEND_URL is empty, skip sending incident group")
		return nil
	}

	batch := groupBatch{
		GroupKey: groupKey,
		Window:   appCfg.CorrWindowRaw,
		Alerts:   alerts,
	}

	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("marshal incident group: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appCfg.GroupBackendURL, bytes.NewReader(data))
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
		return fmt.Errorf("group backend HTTP %d: %s", resp.StatusCode, string(body))
	}

	log.Printf(
		"GROUP_SENT: group_key=%s alerts=%d window=%s backend=%s status=%d",
		groupKey, len(alerts), appCfg.CorrWindowRaw, appCfg.GroupBackendURL, resp.StatusCode,
	)
	return nil
}
