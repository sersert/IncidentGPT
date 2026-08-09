/** Метка severity как в исходном алерте — ровно то, что стоит в PrometheusRule. */
export type Severity = 'critical' | 'warning' | 'info' | 'unknown'
export type IncidentStatus = 'active' | 'resolved' | 'muted' | 'acknowledged'
export type ChainEventType = 'deploy' | 'pod_event' | 'prometheus_alert' | 'log_event' | 'k8s_event'
export type LLMProvider = 'ollama' | 'openrouter' | 'openai' | 'custom'
export type OutputMode = 'brief' | 'standard' | 'expert'
export type IntegrationStatus = 'connected' | 'disconnected' | 'not_configured'

export type AlertStatus = 'firing' | 'resolved'

/**
 * Алерт — единица сырого потока от Alertmanager/Zabbix/Loki.
 * Инцидент — группа связанных алертов с одним AI-разбором. Это разные сущности:
 * у одного инцидента может быть 8 алертов, часть из которых разбор отбросил.
 */
export interface Alert {
  id: string
  fingerprint: string
  alertname: string
  severity: Severity
  status: AlertStatus
  namespace: string
  source: string
  starts_at: string
  ends_at: string
  summary: string
  description: string
  labels?: Record<string, string>
  received_at: string
  /** null — алерт пришёл, но в группу ещё не попал. */
  incident_id: string | null
  /** Разбор счёл алерт не относящимся к первопричине. */
  discarded: boolean
  discard_reason?: string
}

export interface Incident {
  id: string
  title: string
  severity: Severity
  status: IncidentStatus
  namespace: string
  source: string
  created_at: string
  updated_at: string
  analysis_summary: string | null
  /** Сколько алертов склеено в этот инцидент. */
  alert_count: number
  /** Сколько из них разбор отбросил как шум. */
  discarded_count: number
  chain_length: number
  log_errors: number
  confidence: number | null
  duration_sec: number | null
  has_postmortem: boolean
}

export interface ChainEvent {
  id: string
  type: ChainEventType
  source: string
  timestamp: string
  title: string
  data: Record<string, unknown>
  is_root_cause: boolean
}

export interface AIAnalysis {
  root_cause: string
  confidence: number
  recommendations: Recommendation[]
  similar_incidents: SimilarIncident[]
  /** Какие алерты группы отброшены и почему — объясняет, почему из 8 в разбор пошло 6. */
  discarded_noise?: DiscardedAlert[]
  generated_at: string
}

export interface DiscardedAlert {
  alertname: string
  reason: string
}

export interface Recommendation {
  title: string
  description: string
  command: string | null
  category: 'immediate' | 'fix' | 'prevent'
}

export interface SimilarIncident {
  id: string
  title: string
  date: string
  resolution: string
  similarity: number
}

export interface StatusChange {
  status: IncidentStatus
  at: string
  by: string
  assignee?: string | null
}

export interface IncidentDetail extends Incident {
  /** Кто взял инцидент. */
  assignee: string | null
  /** Журнал действий: кто что сделал и когда. */
  history?: StatusChange[]
  /** Алерты, из которых собран инцидент. */
  alerts: Alert[]
  chain: ChainEvent[]
  root_cause_event_id: string | null
  logs: LogEntry[]
  context: IncidentContext
  analysis: AIAnalysis | null
  raw_alert: Record<string, unknown>
  enriched_data: Record<string, unknown>
  metrics: MetricSeries[]
  feedback: Feedback | null
}

export interface LogEntry {
  timestamp: string
  level: 'INFO' | 'WARN' | 'ERROR' | 'FATAL'
  message: string
  is_stacktrace: boolean
}

export interface IncidentContext {
  namespace: string
  pod: string
  pod_status: string
  node: string
  node_ram: string
  node_cpu: string
  last_deploy_version: string
  last_deploy_at: string
  last_deploy_source: string
  labels: Record<string, string>
}

export interface MetricSeries {
  name: string
  unit: string
  data: MetricPoint[]
  incident_start: string
}

export interface MetricPoint {
  timestamp: string
  value: number
}

export interface Feedback {
  helpful: boolean
  comment: string | null
  submitted_at: string
}

export interface DashboardStats {
  active_incidents: number
  active_critical: number
  mttr_reduction_pct: number
  noise_reduction_pct: number
  avg_analysis_time_sec: number
  alerts_today: number
  alerts_sparkline: number[]
}

export interface AlertVolumeBucket {
  date: string
  critical: number
  warning: number
  info: number
  unknown: number
}

export interface Prediction {
  id: string
  resource: string
  resource_type: string
  current_value: number
  current_unit: string
  predicted_value: number
  time_to_critical: string
  growth_rate: string
  severity: 'warning' | 'critical'
  recommendation: string
  sparkline: number[]
  node: string
  namespace: string
}

export interface NodeUsage {
  node: string
  cpu: number
  memory: number
  disk: number
}

export interface PredictionSummary {
  healthy_count: number
  /** Сколько файловых систем и нод под наблюдением. */
  watched_count: number
  warning_count: number
  critical_count: number
  /** Разбивка по нодам: среднее по кластеру прячет забитую ноду. */
  nodes: NodeUsage[]
  network_status: 'stable' | 'unstable'
}

export interface PredictionTrend {
  actual: MetricPoint[]
  predicted: MetricPoint[]
  threshold: number
  threshold_label: string
}

export interface LicenseInfo {
  plan: 'starter' | 'professional' | 'enterprise'
  key: string
  expires_at: string
  status: 'active' | 'expiring_soon' | 'expired'
  usage: {
    alerts_used: number
    alerts_limit: number
    clusters_used: number
    clusters_limit: number
    servers_used: number
    servers_limit: number
  }
  features: {
    enrichment: boolean
    correlation: boolean
    logs: boolean
    predictions: boolean
    postmortem: boolean
  }
  support_level: 'email' | '8x5_chat' | '24x7'
}

export interface LLMConfig {
  provider: LLMProvider
  base_url: string
  api_key: string
  model: string
  temperature: number
  max_tokens: number
  system_prompt: string
}

export interface IntegrationField {
  key: string
  label: string
  kind: 'text' | 'password' | 'number' | 'select'
  required?: boolean
  hint?: string
  /** Для kind: 'select' — какие типы источника доступны. */
  options?: { value: string; label: string }[]
}

export interface Integration {
  type: string
  name: string
  category: 'data_source' | 'output_channel'
  status: IntegrationStatus
  config: Record<string, string>
  /** Описание полей формы приходит с бэкенда — он же знает, что нужно интеграции. */
  fields: IntegrationField[]
  last_heartbeat: string | null
  last_error?: string
  /** Справочные данные, которые нельзя править: версия кластера, канал и т.п. */
  info?: Record<string, string>
  enabled: boolean
}

export interface IntegrationTestResult {
  success: boolean
  message: string
  latency_ms: number
}

/**
 * Конфигурация модели задаётся в окружении ai-worker, поэтому здесь она
 * только для чтения: бэкенд спрашивает ai-worker и отдаёт как есть.
 */
export interface LLMStatus {
  editable: false
  managed_by: string
  note: string
  reachable: boolean
  error?: string
  model?: string
  max_tokens?: number
  timeout?: string
  base_url_configured?: boolean
  telegram_channel_configured?: boolean
  telegram_parse_mode?: string
  active_llm_slots?: number
}

export interface PostMortem {
  markdown: string
  generated_at: string
  status: 'draft' | 'ready' | 'generating'
  last_edited_by: string | null
  last_edited_at: string | null
}

export interface KnowledgeResult {
  incident_id: string
  title: string
  severity: Severity
  date: string
  root_cause: string
  resolution: string
  namespace: string
  service: string
  similarity: number
  has_postmortem: boolean
  resolved_in_min: number
}

export interface ServiceNode {
  id: string
  name: string
  type: 'gateway' | 'service' | 'database' | 'cache' | 'queue'
  namespace: string
  status: 'healthy' | 'incident' | 'degraded' | 'warning' | 'unknown'
}

export interface ServiceEdge {
  source: string
  target: string
  protocol?: string
}

export interface ServiceDetail {
  id: string
  name: string
  namespace: string
  status: string
  metrics: { cpu_pct: number; memory_pct: number; replicas: string }
  active_incidents: number
  last_deploy: { version: string; at: string; source: string }
  inbound: string[]
  outbound: string[]
}

export interface AnalyticsSummary {
  mttr_before_min: number
  mttr_after_min: number
  mttr_reduction_pct: number
  mtta_before_min: number
  mtta_after_min: number
  mtta_reduction_pct: number
  noise_before: number
  noise_after: number
  noise_reduction_pct: number
  time_saved_hours: number
}

export interface MTTRDataPoint {
  date: string
  /** Секунды от появления инцидента до ответа модели. */
  analysis_sec?: number
  /** Минуты от появления до закрытия. */
  close_min?: number
}

export interface AnalyticsQuality {
  total_analyses: number
  helpful_pct: number
  not_helpful_pct: number
  edited_pct: number
  avg_confidence: number
  avg_response_time_sec: number
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
}

export interface WSMessage {
  type: 'incident_created' | 'incident_updated' | 'analysis_ready' | 'postmortem_ready'
  payload: Record<string, unknown>
}
