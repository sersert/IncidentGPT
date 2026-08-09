package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── Config ──────────────────────────────────────────────────────────────────

var (
	listenAddr  = env("LISTEN_ADDR", ":8080")
	aiWorkerURL = env("AI_WORKER_URL", "http://ai-worker.incidentgpt.svc:8080/incident")
	forwardToAI = env("FORWARD_TO_AI_WORKER", "true")
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── Models ──────────────────────────────────────────────────────────────────

// Alert — единица сырого потока: один алерт от Alertmanager/Zabbix/Loki.
// Инцидент — это группа таких алертов, склеенная enricher'ом. Одно к одному
// они НЕ соответствуют: у инцидента может быть 8 алертов, часть из которых
// разбор отбросил как не относящиеся к делу.
type Alert struct {
	ID          string            `json:"id"`
	Fingerprint string            `json:"fingerprint"`
	AlertName   string            `json:"alertname"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Namespace   string            `json:"namespace"`
	Source      string            `json:"source"`
	StartsAt    string            `json:"starts_at"`
	EndsAt      string            `json:"ends_at"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels,omitempty"`
	ReceivedAt  time.Time         `json:"received_at"`

	// Привязка к инциденту. Пусто — алерт пришёл, но в группу ещё не попал.
	IncidentID *string `json:"incident_id"`
	// Разбор посчитал алерт не относящимся к первопричине (затянувшийся,
	// с другим starts_at и т.п.). Он остаётся в группе, но помечен.
	Discarded     bool   `json:"discarded"`
	DiscardReason string `json:"discard_reason,omitempty"`
}

type Incident struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Severity        string            `json:"severity"`
	Status          string            `json:"status"`
	Namespace       string            `json:"namespace"`
	Source          string            `json:"source"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	AnalysisSummary *string           `json:"analysis_summary"`
	AlertCount      int               `json:"alert_count"`
	DiscardedCount  int               `json:"discarded_count"`
	ChainLength     int               `json:"chain_length"`
	LogErrors       int               `json:"log_errors"`
	Confidence      *int              `json:"confidence"`
	DurationSec     *int              `json:"duration_sec"`
	HasPostmortem   bool              `json:"has_postmortem"`
	Labels          map[string]string `json:"labels,omitempty"`

	// Detail fields
	Alerts       []*Alert         `json:"alerts,omitempty"`
	Chain        []ChainEvent     `json:"chain,omitempty"`
	RootCauseID  *string          `json:"root_cause_event_id"`
	Logs         []LogEntry       `json:"logs,omitempty"`
	Context      *IncidentContext `json:"context,omitempty"`
	Analysis     *AIAnalysis      `json:"analysis,omitempty"`
	RawAlert     map[string]any   `json:"raw_alert,omitempty"`
	EnrichedData map[string]any   `json:"enriched_data,omitempty"`
	Metrics      []MetricSeries   `json:"metrics,omitempty"`
	Feedback     *Feedback        `json:"feedback,omitempty"`

	// Кто взял инцидент и что с ним делали. Журнал нужен не только людям:
	// это разметка, без которой потом не на чем учить модель.
	Assignee *string        `json:"assignee"`
	History  []StatusChange `json:"history,omitempty"`

	// Ключ группы от enricher. Ai-worker знает только его, поэтому результат
	// разбора мы сопоставляем с инцидентом именно по нему.
	GroupKey string `json:"group_key,omitempty"`

	// Отпечатки всех алертов группы — по ним находим инцидент при доборе.
	Fingerprints map[string]bool `json:"-"`
}

// StatusChange — одна запись журнала: что произошло, когда и кто это сделал.
type StatusChange struct {
	Status   string  `json:"status"`
	At       string  `json:"at"`
	By       string  `json:"by"`
	Assignee *string `json:"assignee,omitempty"`
}

type ChainEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Source      string         `json:"source"`
	Timestamp   string         `json:"timestamp"`
	Title       string         `json:"title"`
	Data        map[string]any `json:"data"`
	IsRootCause bool           `json:"is_root_cause"`
}

type AIAnalysis struct {
	RootCause       string           `json:"root_cause"`
	Confidence      int              `json:"confidence"`
	Recommendations []Recommendation `json:"recommendations"`
	Similar         []SimilarInc     `json:"similar_incidents"`
	// Какие алерты группы разбор отбросил и почему. Это объясняет, почему из
	// 8 пришедших алертов в первопричину пошло 6 — без объяснения группировке
	// не доверяют.
	DiscardedNoise []DiscardedAlert `json:"discarded_noise,omitempty"`
	GeneratedAt    string           `json:"generated_at"`
}

type DiscardedAlert struct {
	AlertName string `json:"alertname"`
	Reason    string `json:"reason"`
}

type Recommendation struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Command     *string `json:"command"`
	Category    string  `json:"category"`
}

type SimilarInc struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Date       string  `json:"date"`
	Resolution string  `json:"resolution"`
	Similarity float64 `json:"similarity"`
}

type LogEntry struct {
	Timestamp    string `json:"timestamp"`
	Level        string `json:"level"`
	Message      string `json:"message"`
	IsStacktrace bool   `json:"is_stacktrace"`
}

type IncidentContext struct {
	Namespace         string            `json:"namespace"`
	Pod               string            `json:"pod"`
	PodStatus         string            `json:"pod_status"`
	Node              string            `json:"node"`
	NodeRAM           string            `json:"node_ram"`
	NodeCPU           string            `json:"node_cpu"`
	LastDeployVersion string            `json:"last_deploy_version"`
	LastDeployAt      string            `json:"last_deploy_at"`
	LastDeploySource  string            `json:"last_deploy_source"`
	Labels            map[string]string `json:"labels"`
}

type MetricSeries struct {
	Name          string        `json:"name"`
	Unit          string        `json:"unit"`
	Data          []MetricPoint `json:"data"`
	IncidentStart string        `json:"incident_start"`
}

type MetricPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type Feedback struct {
	Helpful     bool    `json:"helpful"`
	Comment     *string `json:"comment"`
	SubmittedAt string  `json:"submitted_at"`
}

// Enriched alert from enricher
type EnrichedAlert struct {
	Source         string            `json:"source"`
	Status         string            `json:"status"`
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
	StartsAt       string            `json:"starts_at"`
	EndsAt         string            `json:"ends_at"`
	Severity       string            `json:"severity"`
	Fingerprint    string            `json:"fingerprint"`
	PromSample     map[string]any    `json:"prom_sample"`
	K8sContext     map[string]any    `json:"k8s_context"`
	GrafanaLinks   []string          `json:"grafana_links"`
	ExtraNotes     []string          `json:"extra_notes"`
	IncidentHints  []string          `json:"incident_hints"`
	RecentLogs     []string          `json:"recent_logs"`
	Enriched       bool              `json:"enriched"`
	EnrichedAt     string            `json:"enriched_at"`
	ClusterName    string            `json:"cluster_name"`
	Environment    string            `json:"environment"`
	PrimaryMetric  string            `json:"primary_metric"`
	PrimarySummary string            `json:"primary_summary"`
}

// ── Store ───────────────────────────────────────────────────────────────────

// Store — фасад над хранилищем. Если Redis доступен, состояние живёт в нём и
// переживает рестарт пода; иначе работаем в памяти, чтобы приём алертов не встал.
//
// Заметка на будущее: List* каждый раз тянет и разбирает все записи. Для нынешних
// объёмов это нормально, но при росте сюда нужен постраничный доступ на стороне
// Redis, а лучше — полноценная БД.
type Store struct {
	mu   sync.RWMutex
	disk *Storage
	// Ниже — работа в памяти, когда Redis недоступен.
	incidents        map[string]*Incident
	alerts           map[string]*Alert
	counter          int
	alertCounter     int
	totalAlertsAll   int
	totalAlertsToday int
	alertsDay        string
}

func NewStore() *Store {
	return &Store{
		disk:      NewStorage(env("REDIS_ADDR", ""), env("REDIS_PASSWORD", "")),
		incidents: make(map[string]*Incident),
		alerts:    make(map[string]*Alert),
		alertsDay: time.Now().Format("2006-01-02"),
	}
}

func (s *Store) persistent() bool { return s.disk != nil }

func (s *Store) TrackAlert() {
	if s.persistent() {
		s.disk.TrackAlert()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if today != s.alertsDay {
		s.alertsDay = today
		s.totalAlertsToday = 0
	}
	s.totalAlertsAll++
	s.totalAlertsToday++
}

func (s *Store) AlertStats() (total, today int) {
	if s.persistent() {
		return s.disk.AlertStats()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if time.Now().Format("2006-01-02") != s.alertsDay {
		return s.totalAlertsAll, 0
	}
	return s.totalAlertsAll, s.totalAlertsToday
}

func (s *Store) NextID() string {
	if s.persistent() {
		if id, err := s.disk.NextIncidentID(); err == nil {
			return id
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	return fmt.Sprintf("INC-%d-%04d", time.Now().Year(), s.counter)
}

func (s *Store) Put(inc *Incident) {
	if s.persistent() {
		if err := s.disk.SaveIncident(inc); err != nil {
			log.Printf("не удалось сохранить инцидент %s: %v", inc.ID, err)
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incidents[inc.ID] = inc
}

func (s *Store) Get(id string) *Incident {
	if s.persistent() {
		inc, err := s.disk.GetIncident(id)
		if err != nil {
			return nil
		}
		return inc
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.incidents[id]
}

func (s *Store) NextAlertID() string {
	if s.persistent() {
		if id, err := s.disk.NextAlertID(); err == nil {
			return id
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertCounter++
	return fmt.Sprintf("ALT-%d-%05d", time.Now().Year(), s.alertCounter)
}

func (s *Store) PutAlert(a *Alert) {
	if s.persistent() {
		if err := s.disk.SaveAlert(a); err != nil {
			log.Printf("не удалось сохранить алерт %s: %v", a.ID, err)
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts[a.ID] = a
}

func (s *Store) GetAlert(id string) *Alert {
	if s.persistent() {
		a, err := s.disk.GetAlert(id)
		if err != nil {
			return nil
		}
		return a
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alerts[id]
}

// ListAlerts — сырой поток, новые сверху.
func (s *Store) ListAlerts() []*Alert {
	if s.persistent() {
		out, err := s.disk.ListAlerts()
		if err != nil {
			log.Printf("не удалось прочитать алерты: %v", err)
			return []*Alert{}
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Alert, 0, len(s.alerts))
	for _, a := range s.alerts {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ReceivedAt.After(out[j].ReceivedAt)
	})
	return out
}

func (s *Store) FindAlertByFingerprint(fp string) *Alert {
	if s.persistent() {
		a, err := s.disk.FindAlertByFingerprint(fp)
		if err != nil {
			return nil
		}
		return a
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.alerts {
		if a.Fingerprint == fp && a.Status == "firing" {
			return a
		}
	}
	return nil
}

// FindIncidentForGroup ищет активный инцидент, которому принадлежит хотя бы
// один из отпечатков группы. Нужно, чтобы повторная доставка той же группы
// не плодила дубли, а дополняла существующий инцидент.
func (s *Store) FindIncidentForGroup(fingerprints []string) *Incident {
	if s.persistent() {
		for _, fp := range fingerprints {
			inc, err := s.disk.FindIncidentByFingerprint(fp)
			if err == nil && inc != nil && inc.Status == "active" {
				return inc
			}
		}
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, inc := range s.incidents {
		if inc.Status != "active" {
			continue
		}
		for _, fp := range fingerprints {
			if inc.Fingerprints[fp] {
				return inc
			}
		}
	}
	return nil
}

func (s *Store) List() []*Incident {
	if s.persistent() {
		out, err := s.disk.ListIncidents()
		if err != nil {
			log.Printf("не удалось прочитать инциденты: %v", err)
			return []*Incident{}
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Incident, 0, len(s.incidents))
	for _, inc := range s.incidents {
		out = append(out, inc)
	}
	sortIncidents(out)
	return out
}

var store = NewStore()

// ── Ingest ──────────────────────────────────────────────────────────────────

// toAlert разбирает обогащённый алерт в сущность потока.
func toAlert(ea EnrichedAlert) *Alert {
	alertname := ea.Labels["alertname"]
	if alertname == "" {
		alertname = "Unknown Alert"
	}
	ns := strVal(ea.K8sContext, "namespace")
	if ns == "" {
		ns = ea.Labels["namespace"]
	}
	status := ea.Status
	if status == "" {
		status = "firing"
	}
	return &Alert{
		ID:          store.NextAlertID(),
		Fingerprint: ea.Fingerprint,
		AlertName:   alertname,
		Severity:    normalizeSeverity(ea.Severity, ea.Labels["severity"]),
		Status:      status,
		Namespace:   ns,
		Source:      ea.Source,
		StartsAt:    ea.StartsAt,
		EndsAt:      ea.EndsAt,
		Summary:     ea.Annotations["summary"],
		Description: ea.Annotations["description"],
		Labels:      ea.Labels,
		ReceivedAt:  time.Now(),
	}
}

// ingestGroup принимает то, что enricher уже склеил в группу, и делает из неё
// ОДИН инцидент. Раньше здесь был цикл по алертам, и группа из 8 связанных
// алертов превращалась в 8 инцидентов — веб показывал не то, что Telegram.
func ingestGroup(enriched []EnrichedAlert, groupKey string) *Incident {
	if len(enriched) == 0 {
		return nil
	}

	firing := make([]EnrichedAlert, 0, len(enriched))
	fingerprints := make([]string, 0, len(enriched))
	for _, ea := range enriched {
		fingerprints = append(fingerprints, ea.Fingerprint)
		if ea.Status != "resolved" {
			firing = append(firing, ea)
		}
	}

	existing := store.FindIncidentForGroup(fingerprints)

	// Вся группа погасла — закрываем инцидент.
	if len(firing) == 0 {
		if existing != nil {
			existing.Status = "resolved"
			now := time.Now()
			existing.UpdatedAt = now
			dur := int(now.Sub(existing.CreatedAt).Seconds())
			existing.DurationSec = &dur
			for _, a := range existing.Alerts {
				a.Status = "resolved"
				store.PutAlert(a)
			}
			// В памяти хватило бы мутации по указателю, в Redis нужно записать.
			store.Put(existing)
			log.Printf("Resolved incident %s", existing.ID)
		}
		return existing
	}

	// Собираем алерты потока; повторные по fingerprint не дублируем.
	newAlerts := make([]*Alert, 0, len(firing))
	for _, ea := range firing {
		if prev := store.FindAlertByFingerprint(ea.Fingerprint); prev != nil {
			prev.ReceivedAt = time.Now()
			continue
		}
		a := toAlert(ea)
		store.PutAlert(a)
		newAlerts = append(newAlerts, a)
	}

	// Группа уже известна — дополняем существующий инцидент.
	if existing != nil {
		if existing.Fingerprints == nil {
			existing.Fingerprints = map[string]bool{}
		}
		for _, a := range newAlerts {
			a.IncidentID = &existing.ID
			existing.Alerts = append(existing.Alerts, a)
			existing.Fingerprints[a.Fingerprint] = true
			store.PutAlert(a)
		}
		existing.AlertCount = len(existing.Alerts)
		existing.UpdatedAt = time.Now()
		store.Put(existing)
		if len(newAlerts) > 0 {
			log.Printf("Incident %s: +%d alerts (total %d)", existing.ID, len(newAlerts), existing.AlertCount)
		}
		return existing
	}

	// Новая группа — новый инцидент.
	lead := firing[0]
	id := store.NextID()
	now := time.Now()

	ns := strVal(lead.K8sContext, "namespace")
	if ns == "" {
		ns = lead.Labels["namespace"]
	}
	pod := strVal(lead.K8sContext, "pod")
	podStatus := strVal(lead.K8sContext, "pod_status")
	if podStatus == "" {
		podStatus = "Unknown"
	}

	// Критичность инцидента — самая высокая среди его алертов.
	severity := "unknown"
	for _, a := range newAlerts {
		if severityRank(a.Severity) < severityRank(severity) {
			severity = a.Severity
		}
	}

	chain := make([]ChainEvent, 0, len(newAlerts))
	for i, a := range newAlerts {
		chain = append(chain, ChainEvent{
			ID:          fmt.Sprintf("evt-%s-%d", id, i+1),
			Type:        sourceToChainType(a.Source),
			Source:      a.Source,
			Timestamp:   a.StartsAt,
			Title:       a.AlertName,
			Data:        map[string]any{"labels": a.Labels},
			IsRootCause: i == 0,
		})
	}

	title := buildGroupTitle(newAlerts, ns, pod)

	inc := &Incident{
		ID:          id,
		Title:       title,
		Severity:    severity,
		Status:      "active",
		Namespace:   ns,
		Source:      lead.Source,
		CreatedAt:   now,
		UpdatedAt:   now,
		AlertCount:  len(newAlerts),
		ChainLength: len(chain),
		Labels:      lead.Labels,
		Alerts:      newAlerts,
		Chain:       chain,
		Context: &IncidentContext{
			Namespace:    ns,
			Pod:          pod,
			PodStatus:    podStatus,
			Node:         strVal(lead.K8sContext, "node"),
			NodeRAM:      strVal(lead.K8sContext, "node_ram"),
			NodeCPU:      strVal(lead.K8sContext, "node_cpu"),
			LastDeployAt: now.Format(time.RFC3339),
			Labels:       lead.Labels,
		},
		RawAlert: map[string]any{"source": lead.Source, "labels": lead.Labels, "annotations": lead.Annotations},
		EnrichedData: map[string]any{
			"prom_sample": lead.PromSample,
			"k8s_context": lead.K8sContext,
			"hints":       lead.IncidentHints,
			// Логи, которыми обогатился алерт: видно ровно то, что ушло в модель.
			"recent_logs": lead.RecentLogs,
		},
		GroupKey:     groupKey,
		Fingerprints: make(map[string]bool, len(newAlerts)),
	}
	if len(chain) > 0 {
		inc.RootCauseID = &chain[0].ID
	}
	for _, a := range newAlerts {
		a.IncidentID = &id
		inc.Fingerprints[a.Fingerprint] = true
	}

	store.Put(inc)
	log.Printf("Created incident %s from %d alerts: %s [%s]", id, len(newAlerts), title, severity)
	return inc
}

// buildGroupTitle: для одиночного алерта — его имя, для группы — сколько
// связанных и что их объединяет.
func buildGroupTitle(alerts []*Alert, ns, pod string) string {
	if len(alerts) == 0 {
		return "Пустая группа"
	}
	if len(alerts) == 1 {
		t := alerts[0].AlertName
		if pod != "" {
			return fmt.Sprintf("%s — %s/%s", t, ns, pod)
		}
		if ns != "" {
			return fmt.Sprintf("%s — %s", t, ns)
		}
		return t
	}

	// Если все алерты одного типа — называем инцидент по нему.
	same := true
	for _, a := range alerts[1:] {
		if a.AlertName != alerts[0].AlertName {
			same = false
			break
		}
	}
	scope := ns
	if scope == "" {
		scope = "несколько namespace"
	}
	if same {
		return fmt.Sprintf("%s — %s, %s", alerts[0].AlertName, pluralAlerts(len(alerts)), scope)
	}
	return fmt.Sprintf("%s — %s", pluralAlerts(len(alerts)), scope)
}

// pluralAlerts склоняет по-русски: 1 связанный алерт, 2 связанных алерта,
// 8 связанных алертов.
func pluralAlerts(n int) string {
	mod10, mod100 := n%10, n%100
	word := "связанных алертов"
	if mod10 == 1 && mod100 != 11 {
		word = "связанный алерт"
	} else if mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) {
		word = "связанных алерта"
	}
	return fmt.Sprintf("%d %s", n, word)
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 1
	case "warning":
		return 2
	case "info":
		return 3
	default:
		return 4
	}
}

func normalizeSeverity(sev string, labelSev string) string {
	s := strings.ToLower(sev)
	if s == "" {
		s = strings.ToLower(labelSev)
	}
	// Отдаём ровно ту метку, что стоит в алерте: инженер знает "warning",
	// а не "P2". Синонимы из других систем сводим к трём известным значениям.
	switch s {
	case "critical", "crit", "disaster", "high", "p1", "1":
		return "critical"
	case "warning", "warn", "average", "p2", "2":
		return "warning"
	case "info", "information", "low", "p3", "3":
		return "info"
	default:
		return "unknown"
	}
}

func sourceToChainType(src string) string {
	switch strings.ToLower(src) {
	case "prometheus", "alertmanager":
		return "prometheus_alert"
	case "kubernetes", "k8s":
		return "k8s_event"
	case "argocd":
		return "deploy"
	case "loki":
		return "log_event"
	default:
		return "prometheus_alert"
	}
}

func strVal(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

// ── HTTP Handlers ───────────────────────────────────────────────────────────

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// parseIngestPayload разбирает три формы, в которых к нам приходят алерты:
//
//	{"group_key":"…","window":"10m","alerts":[…]}  — группа от enricher (основной путь)
//	[{…},{…}]                                       — просто массив алертов
//	{…}                                             — одиночный алерт
//
// Порядок проверок важен: групповой объект нельзя отдавать на разбор как
// одиночный EnrichedAlert — поля не совпадут, ошибки не будет, и получится
// пустой алерт, из которого создастся мусорный инцидент «Unknown Alert».
func parseIngestPayload(body []byte) ([]EnrichedAlert, string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, "", fmt.Errorf("empty body")
	}

	if trimmed[0] == '[' {
		var alerts []EnrichedAlert
		if err := json.Unmarshal(trimmed, &alerts); err != nil {
			return nil, "", fmt.Errorf("parse array: %w", err)
		}
		return alerts, "", nil
	}

	// Объект: сначала смотрим, не групповая ли это пачка.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return nil, "", fmt.Errorf("parse object: %w", err)
	}

	if raw, ok := probe["alerts"]; ok {
		var alerts []EnrichedAlert
		if err := json.Unmarshal(raw, &alerts); err != nil {
			return nil, "", fmt.Errorf("parse group alerts: %w", err)
		}
		groupKey := ""
		if gk, ok := probe["group_key"]; ok {
			_ = json.Unmarshal(gk, &groupKey)
		}
		return alerts, groupKey, nil
	}

	var single EnrichedAlert
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return nil, "", fmt.Errorf("parse single: %w", err)
	}
	// Пустышка вместо алерта — признак того, что формат мы не узнали.
	if single.Fingerprint == "" && len(single.Labels) == 0 {
		return nil, "", fmt.Errorf("unrecognised payload shape")
	}
	return []EnrichedAlert{single}, "", nil
}

// POST /api/v1/ingest — from enricher
func handleIngest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	alerts, groupKey, err := parseIngestPayload(body)
	if err != nil {
		log.Printf("ingest: %v", err)
		http.Error(w, "invalid json", 400)
		return
	}
	if len(alerts) == 0 {
		writeJSON(w, 200, map[string]any{"ok": true, "ingested": 0})
		return
	}

	// Один POST от enricher = одна группа = один инцидент.
	for range alerts {
		store.TrackAlert()
	}
	ingestGroup(alerts, groupKey)

	// Forward to ai-worker asynchronously
	if forwardToAI == "true" && aiWorkerURL != "" {
		go func() {
			resp, err := http.Post(aiWorkerURL, "application/json", bytes.NewReader(body))
			if err != nil {
				log.Printf("Failed to forward to ai-worker: %v", err)
				return
			}
			resp.Body.Close()
			log.Printf("Forwarded to ai-worker: %d", resp.StatusCode)
		}()
	}

	writeJSON(w, 200, map[string]any{"ok": true, "ingested": len(alerts)})
}

// Also accept enricher's format on /api/enriched (backwards compat)
func handleEnrichedCompat(w http.ResponseWriter, r *http.Request) {
	handleIngest(w, r)
}

// GET /api/v1/incidents
func handleListIncidents(w http.ResponseWriter, r *http.Request) {
	all := store.List()

	// Filters
	status := r.URL.Query().Get("status")
	severity := r.URL.Query().Get("severity")
	ns := r.URL.Query().Get("namespace")
	q := strings.ToLower(r.URL.Query().Get("q"))

	filtered := []*Incident{}
	for _, inc := range all {
		if status != "" && inc.Status != status {
			continue
		}
		if severity != "" && !strings.Contains(severity, inc.Severity) {
			continue
		}
		if ns != "" && inc.Namespace != ns {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(inc.Title), q) {
			if inc.AnalysisSummary == nil || !strings.Contains(strings.ToLower(*inc.AnalysisSummary), q) {
				continue
			}
		}
		filtered = append(filtered, inc)
	}

	// Pagination
	limit := intQuery(r, "limit", 25)
	offset := intQuery(r, "offset", 0)
	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := filtered[offset:end]

	// Strip detail fields for list response
	type ListItem struct {
		ID              string  `json:"id"`
		Title           string  `json:"title"`
		Severity        string  `json:"severity"`
		Status          string  `json:"status"`
		Namespace       string  `json:"namespace"`
		Source          string  `json:"source"`
		CreatedAt       string  `json:"created_at"`
		UpdatedAt       string  `json:"updated_at"`
		AnalysisSummary *string `json:"analysis_summary"`
		AlertCount      int     `json:"alert_count"`
		DiscardedCount  int     `json:"discarded_count"`
		ChainLength     int     `json:"chain_length"`
		LogErrors       int     `json:"log_errors"`
		Confidence      *int    `json:"confidence"`
		DurationSec     *int    `json:"duration_sec"`
		HasPostmortem   bool    `json:"has_postmortem"`
	}

	items := make([]ListItem, len(page))
	for i, inc := range page {
		items[i] = ListItem{
			ID: inc.ID, Title: inc.Title, Severity: inc.Severity,
			Status: inc.Status, Namespace: inc.Namespace, Source: inc.Source,
			CreatedAt:       inc.CreatedAt.Format(time.RFC3339),
			UpdatedAt:       inc.UpdatedAt.Format(time.RFC3339),
			AnalysisSummary: inc.AnalysisSummary,
			AlertCount:      inc.AlertCount,
			DiscardedCount:  inc.DiscardedCount,
			ChainLength:     inc.ChainLength,
			LogErrors:       inc.LogErrors, Confidence: inc.Confidence,
			DurationSec: inc.DurationSec, HasPostmortem: inc.HasPostmortem,
		}
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, 200, map[string]any{"data": items, "total": total})
}

// GET /api/v1/incidents/{id}
func handleGetIncident(w http.ResponseWriter, r *http.Request) {
	// Логи лежат во внешнем хранилище и тянутся отдельным запросом: они тяжелее
	// самого инцидента, и грузить их вместе с ним ни к чему.
	if strings.HasSuffix(r.URL.Path, "/logs") {
		handleIncidentLogs(w, r)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
	// Strip sub-paths
	if idx := strings.Index(id, "/"); idx >= 0 {
		id = id[:idx]
	}

	inc := store.Get(id)
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, inc)
}

// PATCH /api/v1/incidents/{id} — действия дежурного над инцидентом.
//
// Кнопки «Принять в работу», «Заглушить», «Назначить» и «Закрыть» раньше были
// нарисованы, но ничего не делали. Каждое действие пишется в журнал вместе с
// именем пользователя — его отдаёт nginx после basic-авторизации.
func handleIncidentPatch(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/")
	if i := strings.Index(id, "/"); i >= 0 {
		id = id[:i]
	}

	inc := store.Get(id)
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}

	var body struct {
		Status   *string `json:"status"`
		Assignee *string `json:"assignee"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	user := requestUser(r)
	now := time.Now()
	changed := false

	if body.Status != nil {
		switch *body.Status {
		case "active", "acknowledged", "muted", "resolved":
		default:
			http.Error(w, "unknown status", 400)
			return
		}
		if *body.Status != inc.Status {
			inc.Status = *body.Status
			// Закрывая руками, фиксируем длительность — иначе таймер в списке
			// продолжит тикать у уже закрытого инцидента.
			if *body.Status == "resolved" && inc.DurationSec == nil {
				d := int(now.Sub(inc.CreatedAt).Seconds())
				inc.DurationSec = &d
			}
			changed = true
		}
	}

	if body.Assignee != nil {
		inc.Assignee = body.Assignee
		changed = true
	}

	if !changed {
		writeJSON(w, 200, inc)
		return
	}

	inc.UpdatedAt = now
	inc.History = append(inc.History, StatusChange{
		Status:   inc.Status,
		At:       now.Format(time.RFC3339),
		By:       user,
		Assignee: inc.Assignee,
	})
	store.Put(inc)
	log.Printf("Incident %s -> %s by %s", inc.ID, inc.Status, user)
	writeJSON(w, 200, inc)
}

// requestUser достаёт имя из заголовка, который проставляет nginx после
// basic-авторизации. Без него журнал был бы анонимным.
func requestUser(r *http.Request) string {
	if u := r.Header.Get("X-Auth-User"); u != "" {
		return u
	}
	if u, _, ok := r.BasicAuth(); ok && u != "" {
		return u
	}
	return "unknown"
}

// POST /api/v1/incidents/{id}/analysis — результат разбора от ai-worker.
//
// Ai-worker раньше умел только постить в Telegram, поэтому веб показывал вечное
// «Модель разбирает инцидент». Это приёмник обратного канала: сюда он присылает
// разбор, и тот же текст появляется в интерфейсе.
//
// Инцидент ищем по id из пути, а если ai-worker знает только ключ группы —
// по нему (`?group_key=...`).
func handleAnalysisCallback(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
	id := parts[0]

	var body struct {
		RootCause       string           `json:"root_cause"`
		Confidence      int              `json:"confidence"`
		Recommendations []Recommendation `json:"recommendations"`
		DiscardedNoise  []DiscardedAlert `json:"discarded_noise"`
		Summary         string           `json:"summary"`
		Raw             string           `json:"raw"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	inc := store.Get(id)
	if inc == nil {
		if gk := r.URL.Query().Get("group_key"); gk != "" {
			inc = findIncidentByGroupKey(gk)
		}
	}
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}

	// Разбор может прийти одним текстом — тогда кладём его в первопричину,
	// чтобы в интерфейсе было хоть что-то осмысленное.
	rootCause := body.RootCause
	if rootCause == "" {
		rootCause = body.Raw
	}

	// Уверенность приходит от модели. Если она её не указала, оставляем пусто —
	// раньше здесь стояла заглушка 75%, и выдуманное число выглядело как оценка.
	confidence := body.Confidence

	inc.Analysis = &AIAnalysis{
		RootCause:       rootCause,
		Confidence:      confidence,
		Recommendations: body.Recommendations,
		DiscardedNoise:  body.DiscardedNoise,
		GeneratedAt:     time.Now().Format(time.RFC3339),
	}
	if confidence > 0 {
		inc.Confidence = &confidence
	}

	summary := body.Summary
	if summary == "" {
		summary = firstLine(rootCause)
	}
	if summary != "" {
		inc.AnalysisSummary = &summary
	}

	// Отмечаем алерты, которые разбор счёл шумом.
	if len(body.DiscardedNoise) > 0 {
		noise := make(map[string]string, len(body.DiscardedNoise))
		for _, d := range body.DiscardedNoise {
			noise[d.AlertName] = d.Reason
		}
		discarded := 0
		for _, a := range inc.Alerts {
			if reason, ok := noise[a.AlertName]; ok {
				a.Discarded = true
				a.DiscardReason = reason
				store.PutAlert(a)
				discarded++
			}
		}
		inc.DiscardedCount = discarded
	}

	inc.UpdatedAt = time.Now()
	store.Put(inc)
	log.Printf("Analysis stored for %s (%d chars, %d recommendations)", inc.ID, len(rootCause), len(body.Recommendations))
	writeJSON(w, 200, map[string]any{"ok": true, "incident_id": inc.ID})
}

func findIncidentByGroupKey(gk string) *Incident {
	// Свежие инциденты идут первыми, поэтому берём первый активный с таким ключом.
	for _, inc := range store.List() {
		if inc.GroupKey == gk && inc.Status == "active" {
			return inc
		}
	}
	return nil
}

// firstLine — короткая выжимка для списка инцидентов.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "*# "))
		if len([]rune(line)) > 20 {
			r := []rune(line)
			if len(r) > 160 {
				return string(r[:160]) + "…"
			}
			return line
		}
	}
	r := []rune(s)
	if len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return s
}

// GET /api/v1/alerts — сырой поток алертов, отдельно от инцидентов.
func handleListAlerts(w http.ResponseWriter, r *http.Request) {
	all := store.ListAlerts()

	status := r.URL.Query().Get("status")
	severity := r.URL.Query().Get("severity")
	ns := r.URL.Query().Get("namespace")
	source := r.URL.Query().Get("source")
	grouped := r.URL.Query().Get("grouped") // "true" / "false"
	q := strings.ToLower(r.URL.Query().Get("q"))

	filtered := make([]*Alert, 0, len(all))
	for _, a := range all {
		if status != "" && a.Status != status {
			continue
		}
		if severity != "" && !strings.Contains(severity, a.Severity) {
			continue
		}
		if ns != "" && a.Namespace != ns {
			continue
		}
		if source != "" && a.Source != source {
			continue
		}
		if grouped == "true" && a.IncidentID == nil {
			continue
		}
		if grouped == "false" && a.IncidentID != nil {
			continue
		}
		if q != "" &&
			!strings.Contains(strings.ToLower(a.AlertName), q) &&
			!strings.Contains(strings.ToLower(a.Summary), q) {
			continue
		}
		filtered = append(filtered, a)
	}

	limit := intQuery(r, "limit", 50)
	offset := intQuery(r, "offset", 0)
	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	writeJSON(w, 200, map[string]any{"data": filtered[offset:end], "total": total})
}

// GET /api/v1/alerts/{id}
func handleGetAlert(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/alerts/")
	if idx := strings.Index(id, "/"); idx >= 0 {
		id = id[:idx]
	}
	a := store.GetAlert(id)
	if a == nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, 200, a)
}

// handleIncidentPost разводит POST-запросы по инциденту: net/http не даёт
// зарегистрировать два префиксных маршрута с разными хвостами.
func handleIncidentPost(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/analysis"):
		handleAnalysisCallback(w, r)
	case strings.HasSuffix(r.URL.Path, "/feedback"):
		handleFeedback(w, r)
	default:
		http.Error(w, "not found", 404)
	}
}

// POST /api/v1/incidents/{id}/feedback
func handleFeedback(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/incidents/"), "/")
	if len(parts) < 2 {
		http.Error(w, "bad request", 400)
		return
	}
	inc := store.Get(parts[0])
	if inc == nil {
		http.Error(w, "not found", 404)
		return
	}

	var fb struct {
		Helpful bool    `json:"helpful"`
		Comment *string `json:"comment"`
	}
	if err := readJSON(r, &fb); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	inc.Feedback = &Feedback{Helpful: fb.Helpful, Comment: fb.Comment, SubmittedAt: time.Now().Format(time.RFC3339)}
	store.Put(inc)

	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /api/v1/dashboard/stats
func handleDashboardStats(w http.ResponseWriter, _ *http.Request) {
	all := store.List()
	active := 0
	activeCritical := 0
	var resolvedDurations []float64

	for _, inc := range all {
		if inc.Status == "active" {
			active++
			if inc.Severity == "critical" {
				activeCritical++
			}
		}
		if inc.Status == "resolved" && inc.DurationSec != nil {
			resolvedDurations = append(resolvedDurations, float64(*inc.DurationSec))
		}
	}

	// MTTR Reduction: compare first half vs second half of resolved incidents
	// If not enough data, show 0%
	mttrReduction := 0
	if len(resolvedDurations) >= 4 {
		mid := len(resolvedDurations) / 2
		oldAvg := avg(resolvedDurations[:mid])
		newAvg := avg(resolvedDurations[mid:])
		if oldAvg > 0 {
			mttrReduction = int(math.Round((oldAvg - newAvg) / oldAvg * 100))
			if mttrReduction < 0 {
				mttrReduction = 0
			}
		}
	}

	// Noise Reduction: total alerts received vs unique incidents created
	totalAlerts, alertsToday := store.AlertStats()
	noiseReduction := 0
	if totalAlerts > 0 && len(all) > 0 {
		noiseReduction = int(math.Round((1 - float64(len(all))/float64(totalAlerts)) * 100))
		if noiseReduction < 0 {
			noiseReduction = 0
		}
	}

	// Avg analysis time: avg duration of resolved incidents (seconds)
	avgAnalysis := 0.0
	if len(resolvedDurations) > 0 {
		avgAnalysis = avg(resolvedDurations)
	}

	writeJSON(w, 200, map[string]any{
		"active_incidents":      active,
		"active_critical":       activeCritical,
		"mttr_reduction_pct":    mttrReduction,
		"noise_reduction_pct":   noiseReduction,
		"avg_analysis_time_sec": int(avgAnalysis),
		"alerts_today":          alertsToday,
		"alerts_sparkline":      []int{},
	})
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// GET /api/v1/dashboard/active
func handleDashboardActive(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 10)
	all := store.List()
	active := []*Incident{}
	for _, inc := range all {
		if inc.Status == "active" {
			active = append(active, inc)
			if len(active) >= limit {
				break
			}
		}
	}
	writeJSON(w, 200, active)
}

// GET /api/v1/analytics/volume
func handleAlertVolume(w http.ResponseWriter, _ *http.Request) {
	all := store.List()
	buckets := make(map[string]map[string]int)
	for _, inc := range all {
		day := inc.CreatedAt.Format("2006-01-02")
		if _, ok := buckets[day]; !ok {
			buckets[day] = map[string]int{"critical": 0, "warning": 0, "info": 0, "unknown": 0}
		}
		key := strings.ToLower(inc.Severity)
		buckets[day][key]++
	}

	type Bucket struct {
		Date     string `json:"date"`
		Critical int    `json:"critical"`
		Warning  int    `json:"warning"`
		Info     int    `json:"info"`
		Unknown  int    `json:"unknown"`
	}

	result := []Bucket{}
	for day, counts := range buckets {
		result = append(result, Bucket{Date: day, Critical: counts["critical"], Warning: counts["warning"], Info: counts["info"], Unknown: counts["unknown"]})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	writeJSON(w, 200, result)
}

// GET /api/v1/analytics/summary
func handleAnalyticsSummary(w http.ResponseWriter, _ *http.Request) {
	all := store.List()
	totalIncidents := len(all)
	var resolved int
	var totalDur float64
	var resolvedDurations []float64
	for _, inc := range all {
		if inc.Status == "resolved" && inc.DurationSec != nil {
			resolved++
			d := float64(*inc.DurationSec)
			totalDur += d
			resolvedDurations = append(resolvedDurations, d)
		}
	}

	avgMTTR := 0.0
	if resolved > 0 {
		avgMTTR = totalDur / float64(resolved) / 60 // minutes
	}

	// MTTR reduction: compare first half vs second half
	mttrReduction := 0
	mttrBefore := avgMTTR
	mttrAfter := avgMTTR
	if len(resolvedDurations) >= 4 {
		mid := len(resolvedDurations) / 2
		mttrBefore = avg(resolvedDurations[:mid]) / 60
		mttrAfter = avg(resolvedDurations[mid:]) / 60
		if mttrBefore > 0 {
			mttrReduction = int(math.Round((mttrBefore - mttrAfter) / mttrBefore * 100))
			if mttrReduction < 0 {
				mttrReduction = 0
			}
		}
	}

	// Noise reduction from real data
	totalAlerts, _ := store.AlertStats()
	noiseBefore := totalAlerts
	noiseAfter := totalIncidents
	noiseReduction := 0
	if noiseBefore > 0 && noiseAfter > 0 {
		noiseReduction = int(math.Round((1 - float64(noiseAfter)/float64(noiseBefore)) * 100))
		if noiseReduction < 0 {
			noiseReduction = 0
		}
	}

	// Time saved: assume each incident saves ~30min vs manual triage
	timeSavedMin := totalIncidents * 30

	writeJSON(w, 200, map[string]any{
		"mttr_before_min":     math.Round(mttrBefore),
		"mttr_after_min":      math.Round(mttrAfter),
		"mttr_reduction_pct":  mttrReduction,
		"mtta_before_min":     0,
		"mtta_after_min":      0,
		"mtta_reduction_pct":  0,
		"noise_before":        noiseBefore,
		"noise_after":         noiseAfter,
		"noise_reduction_pct": noiseReduction,
		"time_saved_hours":    timeSavedMin / 60,
	})
}

// GET /api/v1/analytics/mttr
// Динамика по дням: сколько проходит от алерта до разбора и до закрытия.
// Раньше эндпоинт отдавал пустой массив, и график с линиями не рисовался вовсе.
func handleMTTRTrend(w http.ResponseWriter, _ *http.Request) {
	type bucket struct {
		analysisSum, analysisCount float64
		closeSum, closeCount       float64
	}
	byDay := map[string]*bucket{}

	get := func(day string) *bucket {
		if _, ok := byDay[day]; !ok {
			byDay[day] = &bucket{}
		}
		return byDay[day]
	}

	for _, inc := range store.List() {
		day := inc.CreatedAt.Format("2006-01-02")

		// Время до разбора: от появления инцидента до ответа модели.
		if inc.Analysis != nil && inc.Analysis.GeneratedAt != "" {
			if at, err := time.Parse(time.RFC3339, inc.Analysis.GeneratedAt); err == nil {
				if d := at.Sub(inc.CreatedAt).Minutes(); d >= 0 {
					b := get(day)
					b.analysisSum += d
					b.analysisCount++
				}
			}
		}

		// Время до закрытия — только для закрытых, иначе среднее поедет.
		if inc.DurationSec != nil {
			b := get(day)
			b.closeSum += float64(*inc.DurationSec) / 60
			b.closeCount++
		}
	}

	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)

	data := make([]map[string]any, 0, len(days))
	for _, d := range days {
		b := byDay[d]
		point := map[string]any{"date": d}
		// Разные единицы: разбор занимает секунды, закрытие — минуты и часы.
		// На одной оси их мешать нельзя, поэтому отдаём раздельно.
		if b.analysisCount > 0 {
			point["analysis_sec"] = math.Round(b.analysisSum/b.analysisCount*600) / 10
		}
		if b.closeCount > 0 {
			point["close_min"] = math.Round(b.closeSum/b.closeCount*10) / 10
		}
		data = append(data, point)
	}

	writeJSON(w, 200, map[string]any{"data": data})
}

// GET /api/v1/analytics/quality
func handleAnalyticsQuality(w http.ResponseWriter, _ *http.Request) {
	all := store.List()
	total := 0
	helpful := 0
	for _, inc := range all {
		if inc.Feedback != nil {
			total++
			if inc.Feedback.Helpful {
				helpful++
			}
		}
	}
	helpfulPct := 0
	if total > 0 {
		helpfulPct = helpful * 100 / total
	}
	writeJSON(w, 200, map[string]any{
		"total_analyses":        len(all),
		"helpful_pct":           helpfulPct,
		"not_helpful_pct":       100 - helpfulPct,
		"edited_pct":            0,
		"avg_confidence":        85,
		"avg_response_time_sec": 8.0,
	})
}

// GET /api/v1/analytics/top-sources
func handleTopSources(w http.ResponseWriter, _ *http.Request) {
	all := store.List()
	counts := make(map[string]int)
	sevMap := make(map[string]string)
	for _, inc := range all {
		key := inc.Namespace
		if key == "" {
			key = inc.Source
		}
		counts[key]++
		sevMap[key] = inc.Severity
	}

	type Entry struct {
		Source   string `json:"source"`
		Count    int    `json:"count"`
		Severity string `json:"severity"`
	}
	result := []Entry{}
	for k, v := range counts {
		result = append(result, Entry{Source: k, Count: v, Severity: sevMap[k]})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	if len(result) > 10 {
		result = result[:10]
	}
	writeJSON(w, 200, result)
}

// GET /api/v1/predictions, /api/v1/predictions/summary
// GET /api/v1/license
func handleLicense(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"plan": "professional", "key": "CCII-PRO-2026-XXXX", "expires_at": "2027-02-18",
		"status":        "active",
		"usage":         map[string]any{"alerts_used": len(store.List()), "alerts_limit": 10000, "clusters_used": 1, "clusters_limit": 3, "servers_used": 10, "servers_limit": 100},
		"features":      map[string]bool{"enrichment": true, "correlation": true, "logs": true, "predictions": false, "postmortem": false},
		"support_level": "8x5_chat",
	})
}

// GET /api/v1/settings/llm, PUT, POST test
// Конфигурация модели живёт в окружении ai-worker — он ходит в LLM, не мы.
// Поэтому здесь read-only: спрашиваем ai-worker, что у него настроено, и
// показываем как есть. Менять модель из UI нельзя без передеплоя ai-worker,
// и делать вид, что кнопка «Сохранить» что-то меняет, — врать пользователю.
func handleLLMConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PUT" {
		http.Error(w, "конфигурация модели задаётся в окружении ai-worker", 405)
		return
	}

	cfg := map[string]any{
		"editable":   false,
		"managed_by": "ai-worker",
		"note":       "значения берутся из окружения ai-worker; чтобы изменить — обнови его values и передеплой",
	}

	st, err := fetchAIWorkerStatus()
	if err != nil {
		cfg["reachable"] = false
		cfg["error"] = err.Error()
		writeJSON(w, 200, cfg)
		return
	}

	cfg["reachable"] = true
	if or, ok := st["openrouter"].(map[string]any); ok {
		cfg["model"] = or["model"]
		cfg["max_tokens"] = or["max_tokens"]
		cfg["timeout"] = or["timeout"]
		cfg["base_url_configured"] = or["base_url_configured"]
	}
	if tg, ok := st["telegram"].(map[string]any); ok {
		cfg["telegram_channel_configured"] = tg["channel_configured"]
		cfg["telegram_parse_mode"] = tg["parse_mode"]
	}
	if rt, ok := st["runtime"].(map[string]any); ok {
		cfg["active_llm_slots"] = rt["active_llm_slots"]
	}
	writeJSON(w, 200, cfg)
}

// Реальная проверка: дёргаем ai-worker и меряем время ответа.
func handleLLMTest(w http.ResponseWriter, _ *http.Request) {
	start := time.Now()
	st, err := fetchAIWorkerStatus()
	latency := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, 200, map[string]any{
			"success": false, "error": err.Error(), "latency_ms": latency,
		})
		return
	}

	model := ""
	if or, ok := st["openrouter"].(map[string]any); ok {
		if m, ok := or["model"].(string); ok {
			model = m
		}
	}
	writeJSON(w, 200, map[string]any{
		"success": true, "model": model, "latency_ms": latency,
	})
}

var aiWorkerHTTP = &http.Client{Timeout: 8 * time.Second}

func fetchAIWorkerStatus() (map[string]any, error) {
	base := aiWorkerURL
	// В AI_WORKER_URL лежит путь приёма инцидентов; статус — на том же хосте.
	if i := strings.Index(base, "/incident"); i > 0 {
		base = base[:i]
	}
	base = strings.TrimRight(base, "/")
	if base == "" {
		return nil, fmt.Errorf("адрес ai-worker не задан")
	}

	resp, err := aiWorkerHTTP.Get(base + "/status")
	if err != nil {
		return nil, fmt.Errorf("нет связи с ai-worker: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ai-worker ответил %d", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("не разобрать ответ ai-worker: %w", err)
	}
	return out, nil
}

// GET /api/v1/settings/integrations
func handleIntegrations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, integrations.List())
}

// PUT /api/v1/settings/integrations/{type}
func handleIntegrationSave(w http.ResponseWriter, r *http.Request) {
	typ := integrationTypeFromPath(r.URL.Path)
	var body struct {
		Config  map[string]string `json:"config"`
		Enabled *bool             `json:"enabled"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	itg, err := integrations.Save(typ, body.Config, body.Enabled)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	writeJSON(w, 200, itg)
}

// POST /api/v1/settings/integrations/{type}/test
func handleIntegrationTest(w http.ResponseWriter, r *http.Request) {
	typ := integrationTypeFromPath(r.URL.Path)
	res := integrations.Test(typ)
	writeJSON(w, 200, res)
}

func integrationTypeFromPath(p string) string {
	rest := strings.TrimPrefix(p, "/api/v1/settings/integrations/")
	rest = strings.TrimSuffix(rest, "/test")
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// GET /api/v1/knowledge/search, patterns
func handleKnowledgeSearch(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"results": []any{}, "total": 0})
}
func handleKnowledgePatterns(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, []any{})
}

// GET /api/v1/services/graph, /api/v1/services/{id}
func handleServiceGraph(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"nodes": []any{}, "edges": []any{}})
}
func handleServiceDetail(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not found", 404)
}

// GET /healthz
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "incidents": len(store.List())})
}

func intQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// ── Router ──────────────────────────────────────────────────────────────────

func main() {
	mux := http.NewServeMux()

	// Ingest (from enricher)
	mux.HandleFunc("POST /api/v1/ingest", handleIngest)
	mux.HandleFunc("POST /api/enriched", handleEnrichedCompat)

	// Incidents (группы алертов с AI-разбором)
	mux.HandleFunc("GET /api/v1/incidents", handleListIncidents)
	mux.HandleFunc("GET /api/v1/incidents/", handleGetIncident)
	mux.HandleFunc("POST /api/v1/incidents/", handleIncidentPost)
	mux.HandleFunc("PATCH /api/v1/incidents/", handleIncidentPatch)

	// Alerts (сырой поток)
	mux.HandleFunc("GET /api/v1/alerts", handleListAlerts)
	mux.HandleFunc("GET /api/v1/alerts/", handleGetAlert)

	// Dashboard
	mux.HandleFunc("GET /api/v1/dashboard/stats", handleDashboardStats)
	mux.HandleFunc("GET /api/v1/dashboard/active", handleDashboardActive)

	// Analytics
	mux.HandleFunc("GET /api/v1/analytics/volume", handleAlertVolume)
	mux.HandleFunc("GET /api/v1/analytics/summary", handleAnalyticsSummary)
	mux.HandleFunc("GET /api/v1/analytics/mttr", handleMTTRTrend)
	mux.HandleFunc("GET /api/v1/analytics/quality", handleAnalyticsQuality)
	mux.HandleFunc("GET /api/v1/analytics/top-sources", handleTopSources)

	// Predictions
	mux.HandleFunc("GET /api/v1/predictions", handlePredictions)
	mux.HandleFunc("GET /api/v1/predictions/summary", handlePredictionsSummary)

	// License
	mux.HandleFunc("GET /api/v1/license", handleLicense)

	// Settings
	mux.HandleFunc("GET /api/v1/settings/llm", handleLLMConfig)
	mux.HandleFunc("PUT /api/v1/settings/llm", handleLLMConfig)
	mux.HandleFunc("POST /api/v1/settings/llm/test", handleLLMTest)
	mux.HandleFunc("GET /api/v1/settings/integrations", handleIntegrations)
	mux.HandleFunc("PUT /api/v1/settings/integrations/", handleIntegrationSave)
	mux.HandleFunc("POST /api/v1/settings/integrations/", handleIntegrationTest)

	// Knowledge
	mux.HandleFunc("GET /api/v1/knowledge/search", handleKnowledgeSearch)
	mux.HandleFunc("GET /api/v1/knowledge/patterns", handleKnowledgePatterns)

	// Service Map
	mux.HandleFunc("GET /api/v1/services/graph", handleServiceGraph)
	mux.HandleFunc("GET /api/v1/services/", handleServiceDetail)

	// Health
	mux.HandleFunc("GET /healthz", handleHealth)

	log.Printf("incidentgpt-backend listening on %s", listenAddr)
	log.Printf("AI Worker forward: %s (enabled=%s)", aiWorkerURL, forwardToAI)
	log.Fatal(http.ListenAndServe(listenAddr, corsMiddleware(mux)))
}
