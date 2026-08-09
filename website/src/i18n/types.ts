import type { AiWorkerEnvName, BackendEnvName, EnricherEnvName, MetricName } from "../data/configuration";

export type Language = "ru" | "en";
export type ThemeMode = "system" | "dark" | "light";

export type EnvText = {
  description: string;
  note?: string;
};

export type FlowStepText = {
  title: string;
  input: string;
  action: string;
  output: string;
  dependency: string;
  error: string;
};

export type ProblemCardText = {
  title: string;
  checks: readonly string[];
};

export type Translation = {
  meta: {
    title: string;
    description: string;
  };
  navigation: Record<"overview" | "architecture" | "installation" | "configuration" | "webUi" | "logs" | "examples" | "troubleshooting" | "limitations", string>;
  common: {
    copy: string;
    copied: string;
    download: string;
    reset: string;
    top: string;
    github: string;
    search: string;
    noResults: string;
    breadcrumbsHome: string;
    language: string;
    theme: string;
    dark: string;
    light: string;
    system: string;
    recommended: string;
    currentBehavior: string;
    potentialRisk: string;
  };
  hero: {
    title: string;
    eyebrow: string;
    description: string;
    installButton: string;
    githubButton: string;
    architectureLink: string;
    demoNote: string;
    steps: readonly string[];
  };
  home: {
    terminalTitle: string;
    resultTitle: string;
    featuresTitle: string;
    featuresSubtitle: string;
    compareTitle: string;
    regularApproach: string;
    incidentgptApproach: string;
    compareConclusion: string;
    reliabilityTitle: string;
    reliabilityBody: string;
    quickStartTitle: string;
    quickStartNote: string;
    humanNote: string;
    features: readonly { title: string; body: string }[];
    basicPromptItems: readonly string[];
    contextItems: readonly string[];
  };
  pages: Record<"architecture" | "installation" | "configuration" | "webUi" | "logs" | "examples" | "troubleshooting" | "limitations", {
    title: string;
    description: string;
  }>;
  architecture: {
    headings: Record<"systemMap" | "alertFlow" | "sequence" | "correlation" | "sanitization" | "failures", string>;
    diagramTitles: { system: string; sequence: string };
    alertFlowSteps: readonly string[];
    flowLabels: Record<"input" | "action" | "output" | "dependency" | "error", string>;
    flowSteps: readonly FlowStepText[];
    correlationFacts: readonly { term: string; value: string }[];
    correlationCallout: { title: string; body: string };
    sanitizationParagraphs: readonly string[];
    failureHeaders: { component: string; behavior: string };
    failures: readonly { component: string; behavior: string }[];
  };
  installation: {
    headings: Record<"prerequisites" | "telegram" | "openrouter" | "secrets" | "helm" | "images" | "alertmanager" | "verify", string>;
    prerequisites: readonly string[];
    redisCallout: { title: string; body: string };
    telegramSteps: readonly string[];
    telegramPrivacyNote: string;
    telegramSecretsCallout: { title: string; body: string };
    openrouterNote: string;
    secretsIntro: string;
    secretsNote: string;
    helmIntro: string;
    helmDisableNote: string;
    imagesIntro: string;
    imagesBuildNote: string;
    imagesArchNote: string;
    alertmanagerNote: string;
    verifyIntro: string;
  };
  configuration: {
    headings: Record<"upgrade" | "enricherEnv" | "aiWorkerEnv" | "backendEnv" | "promql" | "runbooks" | "generator", string>;
    tableHeaders: Record<"variable" | "default" | "example" | "purpose", string>;
    upgradeCallout: { title: string; body: string };
    backendIntro: string;
    promqlIntro: string;
    runbooksIntro: string;
    metricLabels: Record<"unit" | "trend" | "series" | "cardinality", string>;
    env: {
      enricher: Record<EnricherEnvName, EnvText>;
      aiWorker: Record<AiWorkerEnvName, EnvText>;
      backend: Record<BackendEnvName, EnvText>;
    };
    metrics: Record<MetricName, {
      description: string;
      unit: string;
      series: string;
      trend: string;
      risk: string;
    }>;
  };
  webUi: {
    headings: Record<"components" | "wiring" | "alertsVsIncidents" | "storage" | "access" | "screens" | "dataSources", string>;
    componentsIntro: string;
    components: readonly { name: string; body: string }[];
    componentsDisable: string;
    wiringIntro: string;
    wiringPassthrough: string;
    wiringNote: string;
    alertsVsIncidents: readonly string[];
    storageIntro: string;
    storageAddress: string;
    storageEviction: string;
    storageCallout: { title: string; body: string };
    accessIntro: string;
    accessProbe: string;
    screens: readonly { name: string; body: string }[];
    dataSourcesIntro: string;
    dataSourceHeaders: Record<"category" | "type" | "worksWith", string>;
    dataSources: readonly { category: string; type: string; worksWith: string }[];
    dataSourcesOutside: string;
  };
  logs: {
    headings: Record<"why" | "requirements" | "wiring" | "collector" | "prompt", string>;
    whyParagraphs: readonly string[];
    requirementsIntro: string;
    fieldHeaders: { field: string; content: string };
    fields: readonly { field: string; content: string }[];
    requirementsSource: string;
    mappingCallout: { title: string; body: string };
    wiringNote: string;
    baseUrlCallout: { title: string; body: string };
    collectorIntro: string;
    collectorNote: string;
    promptIntro: string;
    promptRules: readonly string[];
    promptResult: string;
    promptCounter: string;
  };
  examples: {
    headings: Record<"direct" | "cascade" | "prometheusRule" | "production" | "telegramMock", string>;
    cascadeNote: string;
    prometheusRuleNote: string;
    environments: readonly string[];
    productionItems: readonly string[];
    mockCallout: { title: string; body: string };
  };
  troubleshooting: {
    cards: readonly ProblemCardText[];
  };
  limitations: {
    headings: Record<"correlation" | "llm" | "storage" | "security" | "scaling", string>;
    correlation: { behavior: string; risk: string; recommended: string; items: readonly string[] };
    llmItems: readonly string[];
    llmCallout: { title: string };
    storage: { behavior: string; risk: string; recommended: string };
    securityCallout: { title: string; body: string };
    security: { behavior: string; risk: string; recommended: string };
    scaling: readonly { title: string; behavior: string; recommended: string }[];
  };
  generator: {
    title: string;
    description: string;
    umbrella: string;
    alertmanager: string;
    fields: Record<keyof import("../lib/generator").GeneratorValues, string>;
  };
};
