package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
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
	rdb *redis.Client
	ctx context.Context
}

// NewStorage подключается к Redis. nil означает «работаем в памяти».
func NewStorage(addr, password string) *Storage {
	if addr == "" {
		log.Printf("REDIS_ADDR не задан — история будет теряться при рестарте")
		return nil
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis недоступен (%v) — работаем в памяти", err)
		return nil
	}
	log.Printf("Redis подключён: %s", addr)
	return &Storage{rdb: rdb, ctx: context.Background()}
}

func (s *Storage) SaveIncident(inc *Incident) error {
	data, err := json.Marshal(inc)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Set(s.ctx, keyIncident+inc.ID, data, 0)
	pipe.ZAdd(s.ctx, keyIncidentIdx, redis.Z{Score: float64(inc.CreatedAt.Unix()), Member: inc.ID})
	for fp := range inc.Fingerprints {
		pipe.Set(s.ctx, keyFingerprint+fp, inc.ID, 0)
	}
	_, err = pipe.Exec(s.ctx)
	return err
}

func (s *Storage) GetIncident(id string) (*Incident, error) {
	data, err := s.rdb.Get(s.ctx, keyIncident+id).Bytes()
	if err != nil {
		return nil, err
	}
	var inc Incident
	if err := json.Unmarshal(data, &inc); err != nil {
		return nil, err
	}
	// Отпечатки не сериализуются (json:"-"), восстанавливаем из алертов.
	inc.Fingerprints = make(map[string]bool, len(inc.Alerts))
	for _, a := range inc.Alerts {
		inc.Fingerprints[a.Fingerprint] = true
	}
	return &inc, nil
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
			continue // запись могла истечь или побиться — пропускаем, но не падаем
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
	pipe := s.rdb.TxPipeline()
	pipe.Set(s.ctx, keyAlert+a.ID, data, 0)
	pipe.ZAdd(s.ctx, keyAlertIdx, redis.Z{Score: float64(a.ReceivedAt.Unix()), Member: a.ID})
	_, err = pipe.Exec(s.ctx)
	return err
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
