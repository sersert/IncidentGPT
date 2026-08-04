export type EnvVar = {
  name: string;
  defaultValue: string;
  example: string;
  description: string;
  note?: string;
};

export const enricherEnv: EnvVar[] = [
  {
    name: "PROMETHEUS_URL",
    defaultValue: "http://kube-prometheus-stack-prometheus.monitoring:9090",
    example: "http://kube-prometheus-stack-prometheus.monitoring:9090",
    description: "Prometheus HTTP API base URL. Enricher calls /api/v1/query_range.",
  },
  {
    name: "PYTHON_BACKEND_URL",
    defaultValue: "http://incidentgpt-backend.incidentgpt.svc.cluster.local:8000/api/enriched",
    example: "http://ai-worker.incidentgpt.svc:8080/incident",
    description: "Single-alert fallback endpoint used when Redis correlation is disabled or fails.",
    note: "The code name is legacy; Helm points it to ai-worker.",
  },
  {
    name: "GROUP_BACKEND_URL",
    defaultValue: "http://ai-worker.incidentgpt.svc:8080/incident-group",
    example: "http://ai-worker.incidentgpt.svc:8080/incident-group",
    description: "Endpoint for a flushed Redis group.",
  },
  {
    name: "RAW_BACKEND_URL",
    defaultValue: "http://ai-worker.incidentgpt.svc:8080/incident-raw",
    example: "http://ai-worker.incidentgpt.svc:8080/incident-raw",
    description: "Endpoint used for immediate raw alert delivery while the group window is still open.",
  },
  {
    name: "CLUSTER_NAME",
    defaultValue: "unknown",
    example: "production-eu-1",
    description: "Cluster label included in enriched payloads and metric templates.",
  },
  {
    name: "ENVIRONMENT",
    defaultValue: "unknown",
    example: "prod",
    description: "Environment label added to the incident context.",
  },
  {
    name: "REDIS_ADDR",
    defaultValue: "redis.incidentgpt.svc.cluster.local:6379",
    example: "redis-master.incidentgpt.svc.cluster.local:6379",
    description: "Redis address for namespace-window correlation.",
  },
  {
    name: "REDIS_PASSWORD",
    defaultValue: "",
    example: "secretKeyRef recommended",
    description: "Redis password. Current chart renders it as a plain env value.",
    note: "Recommended improvement: secretKeyRef support for production.",
  },
  {
    name: "CORR_WINDOW",
    defaultValue: "10m",
    example: "10m",
    description: "Redis TTL for grp:{namespace}.",
  },
  {
    name: "CORR_SETTLE",
    defaultValue: "20s in code, 40s in Helm values",
    example: "40s",
    description: "Debounce interval before a group is flushed to ai-worker.",
  },
  {
    name: "RAW_DEDUP_TTL",
    defaultValue: "2m",
    example: "2m",
    description: "TTL for Redis SetNX based raw-alert deduplication by fingerprint. 0s disables the dedup key.",
  },
  {
    name: "PROM_RANGE_BEFORE",
    defaultValue: "15m",
    example: "15m",
    description: "Metrics window before alert startsAt.",
  },
  {
    name: "PROM_RANGE_AFTER",
    defaultValue: "5m",
    example: "5m",
    description: "Metrics window after alert startsAt.",
  },
  {
    name: "RUNBOOK_BASE_URL",
    defaultValue: "",
    example: "https://runbooks.example.com/alerts",
    description: "Base URL used to create runbook_url as base/lowercase-alertname.",
  },
  {
    name: "ENRICH_CLUSTER_CONTEXT",
    defaultValue: "true",
    example: "true",
    description: "Enables cluster-level PromQL templates.",
  },
  {
    name: "ENRICH_NODE_CONTEXT",
    defaultValue: "true",
    example: "true",
    description: "Enables node-level PromQL templates.",
  },
  {
    name: "ENRICH_WORKLOAD_CONTEXT",
    defaultValue: "true",
    example: "true",
    description: "Enables workload-level PromQL templates.",
  },
  {
    name: "ENRICH_EXTERNAL_CONTEXT",
    defaultValue: "true",
    example: "true",
    description: "Enables external dependency PromQL templates.",
  },
  {
    name: "ENRICH_K8S_CONTEXT",
    defaultValue: "true",
    example: "true",
    description: "Enables in-cluster Kubernetes API enrichment.",
  },
];

export const aiWorkerEnv: EnvVar[] = [
  {
    name: "LISTEN_ADDR",
    defaultValue: ":8080",
    example: ":8080",
    description: "HTTP listen address for /healthz, /incident, /incident-raw and /incident-group.",
  },
  {
    name: "OPENROUTER_API_KEY",
    defaultValue: "required",
    example: "Kubernetes Secret",
    description: "API key for OpenRouter or another OpenAI-compatible chat completions endpoint.",
  },
  {
    name: "OPENROUTER_BASE_URL",
    defaultValue: "https://openrouter.ai/api/v1/chat/completions",
    example: "https://openrouter.ai/api/v1/chat/completions",
    description: "Chat completions endpoint.",
  },
  {
    name: "OPENROUTER_MODEL",
    defaultValue: "google/gemini-2.5-flash",
    example: "google/gemini-2.5-flash",
    description: "Model sent in the chat completions payload.",
  },
  {
    name: "OPENROUTER_TIMEOUT_SECONDS",
    defaultValue: "300",
    example: "300",
    description: "HTTP timeout for the LLM request.",
    note: "The previous values key REQUEST_TIMEOUT_SECONDS is not read by the Go code.",
  },
  {
    name: "OPENROUTER_MAX_TOKENS",
    defaultValue: "600 in code, 2000 in Helm values",
    example: "2000",
    description: "Maximum model response tokens.",
  },
  {
    name: "TELEGRAM_BOT_TOKEN",
    defaultValue: "required",
    example: "Kubernetes Secret",
    description: "Bot token from @BotFather.",
  },
  {
    name: "TELEGRAM_CHANNEL_ID",
    defaultValue: "required",
    example: "-1001234567890",
    description: "Channel receiving raw alerts, summaries and analysis messages.",
  },
  {
    name: "TELEGRAM_THREAD_CHAT_ID",
    defaultValue: "TELEGRAM_CHANNEL_ID",
    example: "-1009876543210",
    description: "Discussion group ID stored in config. Replies are sent to the channel post and appear in the linked discussion group when Telegram is configured that way.",
  },
  {
    name: "TELEGRAM_PARSE_MODE",
    defaultValue: "Markdown",
    example: "Markdown",
    description: "Telegram parse mode used for sendMessage.",
  },
];

export const prometheusExamples = [
  {
    name: "workload_cpu_usage",
    unit: "CPU cores",
    series: "One or more series per namespace/pod after aggregation.",
    trend: "rising means the second half of the sampled range is at least 15% above the first half.",
    risk: "High cardinality if pod/container labels are not aggregated before storage in the alert context.",
    description: "CPU usage around the alert start time.",
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
    unit: "bytes",
    series: "Usually one series per pod after sum by pod.",
    trend: "falling means the later samples are at least 15% lower than the earlier samples.",
    risk: "Memory metrics can be noisy during pod churn; keep namespace scoped.",
    description: "Working set memory for pods in the alert namespace.",
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
    unit: "restart count",
    series: "One series per pod/container unless aggregated.",
    trend: "spike is detected when the last point is more than twice the average.",
    risk: "Container-level output can be large in busy namespaces; aggregate when possible.",
    description: "Recent container restarts in the namespace.",
    promql: `sum by (pod) (
  increase(kube_pod_container_status_restarts_total{
    namespace="{{ .Namespace }}"
  }[15m])
)`,
  },
];
