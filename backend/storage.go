package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// Хранилище инцидентов и алертов.
//
// Раньше всё жило в map в памяти, и любой рестарт пода стирал историю — на проде
// это выглядело как «инцидент исчез из списка». Теперь состояние в Redis, который
// и так стоит в кластере для корреляции в enricher.
//
// Если Redis недоступен, сервис продолжает работать в памяти: потерять историю
// неприятно, но отказаться принимать алерты — хуже.

const (
	keyIncident    = "igpt:incident:"    // + id   → JSON
	keyAlert       = "igpt:alert:"       // + id   → JSON
	keyIncidentIdx = "igpt:incidents"    // ZSET   score = created_at, member = id
	keyAlertIdx    = "igpt:alerts"       // ZSET   score = received_at, member = id
	keyFingerprint = "igpt:fp:"          // + fingerprint → incident id
	keyIncidentSeq = "igpt:seq:incident" // счётчик
	keyAlertSeq    = "igpt:seq:alert"
	keyAlertsToday = "igpt:alerts:today:" // + YYYY-MM-DD
	keyAlertsTotal = "igpt:alerts:total"
)

type Storage struct {
	rdb               *redis.Client
	ctx               context.Context
	lockKey           string
	lockOwner         string
	writeErr          error
	alertIndexEnabled atomic.Bool
}

// NewStorage подключается к Redis. nil означает «работаем в памяти».
func NewStorage(addr, password string) *Storage {
	return connectStorage(context.Background(), addr, password)
}

func connectStorage(parent context.Context, addr, password string) *Storage {
	if addr == "" {
		log.Printf("REDIS_ADDR не задан — история будет теряться при рестарте")
		return nil
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:                  addr,
		ContextTimeoutEnabled: true,
		Password:              password,
		DialTimeout:           5 * time.Second,
		ReadTimeout:           3 * time.Second,
		WriteTimeout:          3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		log.Printf("Redis недоступен (%v) — работаем в памяти", err)
		return nil
	}
	log.Printf("Redis подключён: %s", addr)
	return &Storage{rdb: rdb, ctx: context.Background()}
}

// Durable queues and persistent AI control cannot use memory fallback. Retry before
// serving traffic so a fresh Helm install does not depend on Redis pod ordering.
func awaitDurableStorage(ctx context.Context) error {
	if (!durableEnabled() && !operationsEnabled()) || store.disk != nil {
		return nil
	}
	addr := env("REDIS_ADDR", "")
	if addr == "" {
		if operationsEnabled() && !durableEnabled() {
			return fmt.Errorf("operations requires REDIS_ADDR")
		}
		return fmt.Errorf("durable analysis requires REDIS_ADDR")
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if disk := connectStorage(ctx, addr, env("REDIS_PASSWORD", "")); disk != nil {
			store.mu.Lock()
			store.disk = disk
			store.mu.Unlock()
			return nil
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *Storage) SaveIncident(inc *Incident) error {
	data, err := json.Marshal(inc)
	if err != nil {
		return err
	}
	return s.compositionWrite(func(pipe redis.Pipeliner) {
		pipe.Set(s.ctx, keyIncident+inc.ID, data, 0)
		pipe.ZAdd(s.ctx, keyIncidentIdx, redis.Z{Score: float64(inc.CreatedAt.Unix()), Member: inc.ID})
		for fp := range inc.Fingerprints {
			pipe.Set(s.ctx, keyFingerprint+fp, inc.ID, 0)
		}
	})
}

// decodeIncident also reads records rewritten by older Redis cjson scripts.
// Only the two known empty slice fields are repaired; malformed data stays an error.
func decodeIncident(data []byte) (*Incident, error) {
	var inc Incident
	if err := json.Unmarshal(data, &inc); err != nil {
		var object map[string]json.RawMessage
		if json.Unmarshal(data, &object) != nil {
			return nil, err
		}
		var analysis map[string]json.RawMessage
		if json.Unmarshal(object["analysis"], &analysis) != nil {
			return nil, err
		}
		changed := false
		for _, key := range []string{"recommendations", "similar_incidents"} {
			var empty map[string]json.RawMessage
			value := analysis[key]
			if len(value) > 0 && json.Unmarshal(value, &empty) == nil && empty != nil && len(empty) == 0 {
				analysis[key] = json.RawMessage(`[]`)
				changed = true
			}
		}
		if !changed {
			return nil, err
		}
		object["analysis"], _ = json.Marshal(analysis)
		data, _ = json.Marshal(object)
		inc = Incident{}
		if err := json.Unmarshal(data, &inc); err != nil {
			return nil, err
		}
	}
	return &inc, nil
}

func (s *Storage) GetIncident(id string) (*Incident, error) {
	var record, metadata *redis.StringCmd
	_, err := s.rdb.Pipelined(s.ctx, func(p redis.Pipeliner) error {
		record = p.Get(s.ctx, keyIncident+id)
		metadata = p.Get(s.ctx, "igpt:incident:postmortem:"+id)
		return nil
	})
	if err != nil && err != redis.Nil {
		return nil, err
	}
	data, err := record.Bytes()
	if err != nil {
		return nil, err
	}
	inc, err := decodeIncident(data)
	if err != nil {
		return nil, err
	}
	inc.HasPostmortem = inc.HasPostmortem || metadata.Val() == "1"
	// Legacy records did not observe a deployment; their creation time was
	// previously written into last_deploy_at. Do not present that as evidence.
	if inc.AnalysisRequestID == "" && inc.Context != nil {
		inc.Context.LastDeployAt = ""
	}
	// Отпечатки не сериализуются (json:"-"), восстанавливаем из алертов.
	inc.Fingerprints = make(map[string]bool, len(inc.Alerts))
	for _, a := range inc.Alerts {
		inc.Fingerprints[a.Fingerprint] = true
	}
	return inc, nil
}

// ListIncidents отдаёт инциденты, новые сверху.
func (s *Storage) ListIncidents() ([]*Incident, error) {
	ids, err := s.rdb.ZRevRange(s.ctx, keyIncidentIdx, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Incident, 0, len(ids))
	for _, id := range ids {
		inc, err := s.GetIncident(id)
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, err // Failed reads must not masquerade as an empty result.
		}
		out = append(out, inc)
	}
	return out, nil
}

func (s *Storage) FindIncidentByFingerprint(fp string) (*Incident, error) {
	id, err := s.rdb.Get(s.ctx, keyFingerprint+fp).Result()
	if err != nil {
		return nil, err
	}
	return s.GetIncident(id)
}

func (s *Storage) SaveAlert(a *Alert) error {
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.compositionWrite(func(pipe redis.Pipeliner) {
		pipe.Set(s.ctx, keyAlert+a.ID, data, 0)
		pipe.ZAdd(s.ctx, keyAlertIdx, redis.Z{Score: float64(a.ReceivedAt.Unix()), Member: a.ID})
		if s.alertIndexEnabled.Load() || regroupEnabled() {
			pipe.Eval(s.ctx, updateAlertIndex, []string{alertFingerprintKey(a.Fingerprint), keyAlert + a.ID}, a.ID, a.ReceivedAt.UnixMicro(), a.Status, "")
		}
	})
}

func (s *Storage) GetAlert(id string) (*Alert, error) {
	data, err := s.rdb.Get(s.ctx, keyAlert+id).Bytes()
	if err != nil {
		return nil, err
	}
	var a Alert
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Storage) ListAlerts() ([]*Alert, error) {
	ids, err := s.rdb.ZRevRange(s.ctx, keyAlertIdx, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*Alert, 0, len(ids))
	for _, id := range ids {
		a, err := s.GetAlert(id)
		if err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Storage) FindAlertByFingerprint(fp string) (*Alert, error) {
	id, err := s.rdb.HGet(s.ctx, alertFingerprintKey(fp), "alert_id").Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	if id != "" {
		s.alertIndexEnabled.Store(true)
		a, err := s.GetAlert(id)
		if err != nil {
			return nil, err
		}
		if a.Fingerprint != fp {
			return nil, fmt.Errorf("alert fingerprint index mismatch")
		}
		if a.Status == "firing" {
			return a, nil
		}
	}
	ready, err := s.rdb.Exists(s.ctx, alertIndexReady).Result()
	if err != nil {
		return nil, err
	}
	if ready != 0 {
		s.alertIndexEnabled.Store(true)
		return nil, redis.Nil
	}
	if regroupEnabled() {
		return nil, fmt.Errorf("alert fingerprint index requires startup migration")
	}
	alerts, err := s.ListAlerts()
	if err != nil {
		return nil, err
	}
	for _, a := range alerts {
		if a.Fingerprint == fp && a.Status == "firing" {
			return a, nil
		}
	}
	return nil, redis.Nil
}

func (s *Storage) NextIncidentID() (string, error) {
	n, err := s.rdb.Incr(s.ctx, keyIncidentSeq).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("INC-%d-%04d", time.Now().Year(), n), nil
}

func (s *Storage) NextAlertID() (string, error) {
	n, err := s.rdb.Incr(s.ctx, keyAlertSeq).Result()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALT-%d-%05d", time.Now().Year(), n), nil
}

func (s *Storage) TrackAlert() {
	day := time.Now().Format("2006-01-02")
	pipe := s.rdb.TxPipeline()
	pipe.Incr(s.ctx, keyAlertsTotal)
	todayKey := keyAlertsToday + day
	pipe.Incr(s.ctx, todayKey)
	// Счётчик за день живёт двое суток — дольше он не нужен.
	pipe.Expire(s.ctx, todayKey, 48*time.Hour)
	if _, err := pipe.Exec(s.ctx); err != nil {
		log.Printf("не удалось учесть алерт: %v", err)
	}
}

func (s *Storage) AlertStats() (total, today int) {
	t, _ := s.rdb.Get(s.ctx, keyAlertsTotal).Int()
	d, _ := s.rdb.Get(s.ctx, keyAlertsToday+time.Now().Format("2006-01-02")).Int()
	return t, d
}

// sortIncidents — общий порядок выдачи: новые сверху.
func sortIncidents(list []*Incident) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
}

// A Redis WATCH fences all composition writes, including delayed writes after
// lock expiry or replacement. Legacy callers keep their existing transaction.
func (s *Storage) compositionWrite(queue func(redis.Pipeliner)) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	var err error
	if s.lockKey == "" {
		pipe := s.rdb.TxPipeline()
		queue(pipe)
		_, err = pipe.Exec(s.ctx)
	} else {
		err = s.rdb.Watch(s.ctx, func(tx *redis.Tx) error {
			owner, e := tx.Get(s.ctx, s.lockKey).Result()
			if e != nil || owner != s.lockOwner {
				return fmt.Errorf("composition lease lost")
			}
			_, e = tx.TxPipelined(s.ctx, func(pipe redis.Pipeliner) error { queue(pipe); return nil })
			return e
		}, s.lockKey)
	}
	if s.lockKey != "" && err != nil {
		s.writeErr = err
	}
	return err
}
