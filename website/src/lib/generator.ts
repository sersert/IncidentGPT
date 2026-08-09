export type GeneratorValues = {
  clusterName: string;
  prometheusUrl: string;
  redisAddress: string;
  corrWindow: string;
  corrSettle: string;
  openRouterModel: string;
  telegramChannelId: string;
  telegramThreadChatId: string;
  logsStoreUrl: string;
  uiHost: string;
  registry: string;
  imageVersion: string;
};

export const defaultGeneratorValues: GeneratorValues = {
  clusterName: "demo-cluster",
  prometheusUrl: "http://kube-prometheus-stack-prometheus.monitoring:9090",
  redisAddress: "incidentgpt-redis:6379",
  corrWindow: "10m",
  corrSettle: "40s",
  openRouterModel: "google/gemini-2.5-flash",
  telegramChannelId: "-1001234567890",
  telegramThreadChatId: "-1009876543210",
  logsStoreUrl: "",
  uiHost: "incidentgpt.example.com",
  registry: "ghcr.io/sersert",
  imageVersion: "0.2.0",
};

export function generateUmbrellaValues(values: GeneratorValues): string {
  return `incidentgpt-enricher:
  image:
    repository: ${values.registry}/incidentgpt-enricher
    tag: "${values.imageVersion}"
  env:
    prometheusUrl: "${values.prometheusUrl}"
    clusterName: "${values.clusterName}"
    redisAddr: "${values.redisAddress}"
    corrWindow: "${values.corrWindow}"
    corrSettle: "${values.corrSettle}"
    runbookBaseUrl: ""
    logsStoreUrl: "${values.logsStoreUrl}"
    logsStoreIndex: "logs-*"
    # Groups go to backend; it builds the incident and forwards the body to ai-worker.
    groupBackendUrl: "http://incidentgpt-backend:8080/api/v1/ingest"
  secrets:
    existingSecret: incidentgpt-sanitizer
    sanitizerAuthSharedSecretKey: auth-shared-secret

ai-worker:
  image:
    repository: ${values.registry}/incidentgpt-ai-worker
    tag: "${values.imageVersion}"
  env:
    OPENROUTER_MODEL: "${values.openRouterModel}"
    OPENROUTER_MAX_TOKENS: "2000"
    OPENROUTER_TIMEOUT_SECONDS: "300"
    TELEGRAM_CHANNEL_ID: "${values.telegramChannelId}"
    TELEGRAM_THREAD_CHAT_ID: "${values.telegramThreadChatId}"
    # Without this the analysis reaches Telegram only and the web UI keeps waiting.
    ANALYSIS_CALLBACK_URL: "http://incidentgpt-backend:8080/api/v1/incidents/by-group/analysis"
  secrets:
    existingSecret: incidentgpt-ai-worker
    openRouterApiKeyKey: OPENROUTER_API_KEY
    telegramBotTokenKey: TELEGRAM_BOT_TOKEN

incidentgpt-sanitizer:
  image:
    repository: ${values.registry}/incidentgpt-sanitizer
    tag: "${values.imageVersion}"
  secrets:
    existingSecret: incidentgpt-sanitizer

incidentgpt-backend:
  enabled: true
  image:
    repository: ${values.registry}/incidentgpt-backend
    tag: "${values.imageVersion}"
  env:
    AI_WORKER_URL: "http://ai-worker.incidentgpt.svc:8080/incident-group"
    FORWARD_TO_AI_WORKER: "true"
    REDIS_ADDR: "${values.redisAddress}"
    LOGS_STORE_URL: "${values.logsStoreUrl}"
    LOGS_STORE_INDEX: "logs-*"

incidentgpt-ui:
  enabled: true
  image:
    repository: ${values.registry}/incidentgpt-ui
    tag: "${values.imageVersion}"
  ingress:
    enabled: true
    className: nginx
    host: ${values.uiHost}
  auth:
    enabled: true
    username: admin
    existingSecret: incidentgpt-ui-auth

incidentgpt-redis:
  enabled: true
  persistence:
    enabled: true
    size: 2Gi
`;
}

export function generateAlertmanagerSnippet(): string {
  return `alertmanager:
  config:
    route:
      receiver: incident-enricher
      routes:
        - receiver: incident-enricher
          continue: true
    receivers:
      - name: incident-enricher
        webhook_configs:
          - url: >-
              http://incidentgpt-enricher.incidentgpt.svc:9099/alert
            send_resolved: true
`;
}
