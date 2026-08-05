import type { Translation } from "./types";

export const en = {
  meta: {
    title: "IncidentGPT — AI-assisted Kubernetes incident analysis",
    description:
      "Open-source AIOps assistant that enriches Prometheus alerts with Kubernetes context, correlates incidents and sends AI-assisted analysis to Telegram.",
  },
  navigation: {
    overview: "Overview",
    architecture: "Architecture",
    installation: "Installation",
    configuration: "Configuration",
    examples: "Examples",
    troubleshooting: "Troubleshooting",
    limitations: "Limitations",
  },
  common: {
    copy: "Copy",
    copied: "Copied",
    download: "Download",
    reset: "Reset",
    top: "Back to top",
    github: "GitHub",
    search: "Search documentation",
    noResults: "No results",
    breadcrumbsHome: "Documentation",
    language: "Language",
    theme: "Theme",
    dark: "Dark",
    light: "Light",
    system: "System",
    recommended: "Recommended improvement",
    currentBehavior: "Current behavior",
    potentialRisk: "Potential risk",
  },
  hero: {
    title: "Understand incidents before the alert storm ends",
    description:
      "IncidentGPT enriches Prometheus alerts with Kubernetes context, groups an event cascade into one incident, and sends an engineering AI-assisted analysis to Telegram.",
    installButton: "Start installation",
    githubButton: "Open GitHub",
    architectureLink: "View architecture",
    demoNote:
      "This is a demonstration. Real output depends on alerts, metrics, Kubernetes context and the selected model.",
  },
  home: {
    terminalTitle: "Enricher logs",
    resultTitle: "Analysis draft",
    featuresTitle: "Core capabilities",
    compareTitle: "Why not just paste an alert into ChatGPT",
    regularApproach: "Basic prompt",
    incidentgptApproach: "IncidentGPT",
    compareConclusion:
      "AI-assisted analysis is only as useful as the context it receives. IncidentGPT collects that context automatically.",
    reliabilityTitle: "Raw alert first. AI analysis second.",
    reliabilityBody:
      "IncidentGPT publishes the raw alert first. Only then does it call the model and publish analysis. The LLM must not become an alerting failure point.",
    quickStartTitle: "Quick start",
    humanNote:
      "IncidentGPT creates an engineering draft. The final decision always belongs to a human.",
  },
  pages: {
    architecture: {
      title: "Architecture",
      description: "Components, alert flow, Redis correlation and failure scenarios.",
    },
    installation: {
      title: "Installation",
      description: "A Kubernetes deployment guide covering Telegram, OpenRouter, Redis, Helm and Alertmanager.",
    },
    configuration: {
      title: "Configuration",
      description: "Environment variables, PromQL templates, runbooks and a values generator.",
    },
    examples: {
      title: "Examples",
      description: "Webhook payloads, cascade tests, PrometheusRule usage and production configuration.",
    },
    troubleshooting: {
      title: "Troubleshooting",
      description: "How to inspect Alertmanager, Redis, Telegram, OpenRouter, RBAC and service logs.",
    },
    limitations: {
      title: "Limitations",
      description: "Clear boundaries for correlation, LLM output, security and scaling.",
    },
  },
  generator: {
    title: "Helm values generator",
    description: "Secrets are not requested. Create a Kubernetes Secret separately and reference it with existingSecret.",
    aiWorker: "values-ai-worker.yaml",
    enricher: "values-enricher.yaml",
    alertmanager: "Alertmanager snippet",
    fields: {
      clusterName: "Cluster name",
      prometheusUrl: "Prometheus URL",
      redisAddress: "Redis address",
      corrWindow: "Correlation window",
      corrSettle: "Correlation settle",
      openRouterModel: "OpenRouter model",
      telegramChannelId: "Telegram channel ID",
      telegramThreadChatId: "Telegram discussion group ID",
      registry: "Container registry",
      imageVersion: "Image version",
    },
  },
} as const satisfies Translation;
