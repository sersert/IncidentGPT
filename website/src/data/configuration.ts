// Только машинная часть таблиц: имена, дефолты и PromQL.
// Проза живёт в словарях i18n и ключуется этими же именами.

export const enricherEnv = [
  { name: "PROMETHEUS_URL", defaultValue: "http://kube-prometheus-stack-prometheus.monitoring:9090", example: "http://kube-prometheus-stack-prometheus.monitoring:9090" },
  { name: "PYTHON_BACKEND_URL", defaultValue: "http://incidentgpt-backend.incidentgpt.svc.cluster.local:8000/api/enriched", example: "http://ai-worker.incidentgpt.svc:8080/incident" },
  { name: "GROUP_BACKEND_URL", defaultValue: "http://ai-worker.incidentgpt.svc:8080/incident-group", example: "http://incidentgpt-backend:8080/api/v1/ingest" },
  { name: "RAW_BACKEND_URL", defaultValue: "http://ai-worker.incidentgpt.svc:8080/incident-raw", example: "http://ai-worker.incidentgpt.svc:8080/incident-raw" },
  { name: "CLUSTER_NAME", defaultValue: "unknown", example: "production-eu-1" },
  { name: "ENVIRONMENT", defaultValue: "unknown", example: "prod" },
  { name: "REDIS_ADDR", defaultValue: "redis.incidentgpt.svc.cluster.local:6379", example: "incidentgpt-redis:6379" },
  { name: "REDIS_PASSWORD", defaultValue: "—", example: "secretKeyRef" },
  { name: "CORR_WINDOW", defaultValue: "10m", example: "10m" },
  { name: "CORR_SETTLE", defaultValue: "20s", example: "40s" },
  { name: "RAW_DEDUP_TTL", defaultValue: "2m", example: "2m" },
  { name: "PROM_RANGE_BEFORE", defaultValue: "15m", example: "15m" },
  { name: "PROM_RANGE_AFTER", defaultValue: "5m", example: "5m" },
  { name: "LOGS_STORE_URL", defaultValue: "—", example: "http://opensearch.logging.svc:9200" },
  { name: "LOGS_STORE_USER", defaultValue: "—", example: "incidentgpt" },
  { name: "LOGS_STORE_INDEX", defaultValue: "logs-*", example: "logs-myapp-*" },
  { name: "LOGS_RANGE_BEFORE", defaultValue: "10m", example: "10m" },
  { name: "LOGS_RANGE_AFTER", defaultValue: "2m", example: "2m" },
  { name: "LOGS_MAX_LINES", defaultValue: "40", example: "40" },
  { name: "LOGS_BASE_URL", defaultValue: "—", example: "https://logs.example.com" },
  { name: "SANITIZER_URL", defaultValue: "http://incidentgpt-sanitizer.incidentgpt.svc:8080", example: "http://incidentgpt-sanitizer.incidentgpt.svc:8080" },
  { name: "SANITIZER_AUTH_SHARED_SECRET", defaultValue: "—", example: "Kubernetes Secret" },
  { name: "RUNBOOK_BASE_URL", defaultValue: "—", example: "https://runbooks.example.com/alerts" },
  { name: "ENRICH_CLUSTER_CONTEXT", defaultValue: "true", example: "true" },
  { name: "ENRICH_NODE_CONTEXT", defaultValue: "true", example: "true" },
  { name: "ENRICH_WORKLOAD_CONTEXT", defaultValue: "true", example: "true" },
  { name: "ENRICH_EXTERNAL_CONTEXT", defaultValue: "true", example: "true" },
  { name: "ENRICH_K8S_CONTEXT", defaultValue: "true", example: "true" },
] as const;

export const aiWorkerEnv = [
  { name: "LISTEN_ADDR", defaultValue: ":8080", example: ":8080" },
  { name: "OPENROUTER_API_KEY", defaultValue: "—", example: "Kubernetes Secret" },
  { name: "OPENROUTER_BASE_URL", defaultValue: "https://openrouter.ai/api/v1/chat/completions", example: "https://openrouter.ai/api/v1/chat/completions" },
  { name: "OPENROUTER_MODEL", defaultValue: "google/gemini-2.5-flash", example: "google/gemini-2.5-flash" },
  { name: "OPENROUTER_TIMEOUT_SECONDS", defaultValue: "300", example: "300" },
  { name: "OPENROUTER_MAX_TOKENS", defaultValue: "600", example: "2000" },
  { name: "TELEGRAM_BOT_TOKEN", defaultValue: "—", example: "Kubernetes Secret" },
  { name: "TELEGRAM_CHANNEL_ID", defaultValue: "—", example: "-1001234567890" },
  { name: "TELEGRAM_THREAD_CHAT_ID", defaultValue: "TELEGRAM_CHANNEL_ID", example: "-1009876543210" },
  { name: "TELEGRAM_PARSE_MODE", defaultValue: "Markdown", example: "Markdown" },
  { name: "ANALYSIS_CALLBACK_URL", defaultValue: "—", example: "http://incidentgpt-backend:8080/api/v1/incidents/by-group/analysis" },
  { name: "SANITIZER_URL", defaultValue: "http://incidentgpt-sanitizer.incidentgpt.svc:8080", example: "http://incidentgpt-sanitizer.incidentgpt.svc:8080" },
  { name: "SANITIZER_AUTH_SHARED_SECRET", defaultValue: "—", example: "Kubernetes Secret" },
] as const;

export const backendEnv = [
  { name: "LISTEN_ADDR", defaultValue: ":8080", example: ":8080" },
  { name: "AI_WORKER_URL", defaultValue: "http://ai-worker.incidentgpt.svc:8080/incident", example: "http://ai-worker.incidentgpt.svc:8080/incident-group" },
  { name: "FORWARD_TO_AI_WORKER", defaultValue: "true", example: "true" },
  { name: "REDIS_ADDR", defaultValue: "—", example: "incidentgpt-redis:6379" },
  { name: "PROMETHEUS_URL", defaultValue: "http://kps-kube-prometheus-stack-prometheus.monitoring:9090", example: "http://kube-prometheus-stack-prometheus.monitoring:9090" },
  { name: "LOGS_STORE_URL", defaultValue: "—", example: "http://opensearch.logging.svc:9200" },
  { name: "LOGS_STORE_INDEX", defaultValue: "logs-*", example: "logs-myapp-*" },
] as const;

export type EnricherEnvName = (typeof enricherEnv)[number]["name"];
export type AiWorkerEnvName = (typeof aiWorkerEnv)[number]["name"];
export type BackendEnvName = (typeof backendEnv)[number]["name"];

export const prometheusExamples = [
  {
    name: "workload_cpu_usage",
    promql: `sum by (pod) (
  rate(container_cpu_usage_seconds_total{
    namespace="{{ .Namespace }}",
    container!="",
    image!=""
  }[5m])
)`,
  },
  {
    name: "workload_memory_working_set",
    promql: `sum by (pod) (
  container_memory_working_set_bytes{
    namespace="{{ .Namespace }}",
    container!="",
    image!=""
  }
)`,
  },
  {
    name: "workload_pod_restarts",
    promql: `sum by (pod) (
  increase(kube_pod_container_status_restarts_total{
    namespace="{{ .Namespace }}"
  }[15m])
)`,
  },
] as const;

export type MetricName = (typeof prometheusExamples)[number]["name"];
