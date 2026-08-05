export type GeneratorValues = {
  clusterName: string;
  prometheusUrl: string;
  redisAddress: string;
  corrWindow: string;
  corrSettle: string;
  openRouterModel: string;
  telegramChannelId: string;
  telegramThreadChatId: string;
  registry: string;
  imageVersion: string;
};

export const defaultGeneratorValues: GeneratorValues = {
  clusterName: "demo-cluster",
  prometheusUrl: "http://kube-prometheus-stack-prometheus.monitoring:9090",
  redisAddress: "redis-master.incidentgpt.svc.cluster.local:6379",
  corrWindow: "10m",
  corrSettle: "40s",
  openRouterModel: "google/gemini-2.5-flash",
  telegramChannelId: "-1001234567890",
  telegramThreadChatId: "-1009876543210",
  registry: "ghcr.io/your-user",
  imageVersion: "0.1.0",
};

export function generateAiWorkerValues(values: GeneratorValues): string {
  return `image:
  repository: ${values.registry}/incidentgpt-ai-worker
  tag: "${values.imageVersion}"
  pullPolicy: IfNotPresent

imagePullSecret: ""

env:
  OPENROUTER_MODEL: "${values.openRouterModel}"
  OPENROUTER_MAX_TOKENS: "2000"
  OPENROUTER_TIMEOUT_SECONDS: "300"
  TELEGRAM_CHANNEL_ID: "${values.telegramChannelId}"
  TELEGRAM_THREAD_CHAT_ID: "${values.telegramThreadChatId}"

secrets:
  existingSecret: incidentgpt-ai-worker
  openRouterApiKeyKey: OPENROUTER_API_KEY
  telegramBotTokenKey: TELEGRAM_BOT_TOKEN
`;
}

export function generateEnricherValues(values: GeneratorValues): string {
  return `image:
  repository: ${values.registry}/incidentgpt-enricher
  tag: "${values.imageVersion}"
  pullPolicy: IfNotPresent

env:
  prometheusUrl: "${values.prometheusUrl}"
  clusterName: "${values.clusterName}"
  redisAddr: "${values.redisAddress}"
  corrWindow: "${values.corrWindow}"
  corrSettle: "${values.corrSettle}"
  runbookBaseUrl: ""
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
