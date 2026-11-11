import type { Translation } from "./types";

export const en = {
  meta: {
    title: "IncidentGPT — AI-assisted Kubernetes incident analysis",
    description:
      "Open-source AIOps assistant that enriches Prometheus alerts with Kubernetes context and logs, correlates them into incidents and delivers AI-assisted analysis to Telegram and a web UI.",
  },
  navigation: {
    overview: "Overview",
    architecture: "Architecture",
    installation: "Installation",
    configuration: "Configuration",
    webUi: "Web UI",
    logs: "Logs enrichment",
    examples: "Examples",
    troubleshooting: "Troubleshooting",
    limitations: "Limitations",
  },
  common: {
    tableOfContents: "On this page",
    mobileNavigation: "Navigation",
    signIn: "Sign in",
    navigation: "Navigation",
    openMenu: "Open menu",
    closeMenu: "Close menu",
    skip: "Skip to content",

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
    title: "From alert to a clear next step.",
    eyebrow: "Open source · Kubernetes incident response",
    description:
      "Bring alerts, logs and infrastructure context into one incident. Investigate with evidence, review the AI analysis and hand over a clear record of what happened.",
    installButton: "Deploy IncidentGPT",
    githubButton: "Source on GitHub",
    architectureLink: "Follow the alert path",
    demoNote:
      "Illustrative data. Explore the four incident views; available features depend on deployment settings.",
    caption: "Self-hosted with Helm. Telegram + an operator workspace.",
  },
  demo: {
    label: "Interactive example",
    queue: "Incident queue",
    severity: "Critical",
    status: "Investigating",
    title: "Checkout error rate is rising",
    summary: "3 related alerts · production-eu · checkout-api",
    tabsLabel: "Example incident views",
    tabs: ["Overview", "Investigation", "Runbooks", "Handoff"],
    evidence: "Example evidence",
    footer: "Evidence stays attached to the incident",
    panels: [
      {
        title: "Working hypothesis",
        body: "Errors began after the checkout rollout. A database connection limit may be contributing; verify the evidence before acting.",
        items: [
          "HighErrorRate · firing",
          "Connection pool exhausted · log excerpt",
          "checkout-api · rollout observed",
        ],
      },
      {
        title: "Build the timeline",
        body: "Review changes, run read-only checks and inspect declared dependencies alongside Kubernetes observations.",
        items: [
          "Deployment update · 14:32",
          "Database reachability · check result",
          "checkout-api → orders-db · declared",
        ],
      },
      {
        title: "Compare a confirmed solution",
        body: "Review the environment and version differences before applying a previous resolution.",
        items: [
          "Connection pool limits · runbook",
          "Previous resolution · confirmed by engineer",
          "Production configuration · review required",
        ],
      },
      {
        title: "Prepare the next shift",
        body: "Draft a postmortem or handoff from incident evidence, then review and edit it before export.",
        items: [
          "Impact and timeline · draft",
          "Open checks · owner to confirm",
          "Review and export · engineer action",
        ],
      },
    ],
  },
  home: {
    workflowLabel: "The response path",
    workflowTitle: "Context first. Then a decision.",
    workflow: [
      {
        title: "Collect the evidence",
        body: "Alertmanager triggers enrichment with metrics, Kubernetes events and optional log excerpts.",
      },
      {
        title: "Investigate one incident",
        body: "Group related signals, inspect the timeline and compare the analysis with its supporting evidence.",
      },
      {
        title: "Keep the resolution",
        body: "Record confirmed decisions, review a handoff and keep the next engineer in context.",
      },
    ],
    featuresLabel: "The operator workspace",
    featuresTitle: "One place to work through an incident.",
    featuresSubtitle:
      "Alerts, changes, checks and confirmed decisions stay connected throughout the response.",
    optInNote:
      "Advanced investigation, knowledge, reports and operations controls require explicit configuration. The default delivery path remains available.",
    reliabilityTitle: "Raw alerts keep moving.",
    reliabilityBody:
      "Model failure does not hide the original alert. With managed AI control enabled, pausing analysis preserves raw ingestion and ordinary Telegram notifications.",
    quickStartLabel: "Run it in your cluster",
    quickStartTitle: "Start with a configured deployment.",
    quickStartNote:
      "The installation guide covers required Secrets, reader credentials, Helm values and a first test alert. Prepare these before running the commands.",
    humanNote:
      "AI proposes an analysis. An engineer verifies the evidence and decides what to do.",
    features: [
      {
        title: "An actionable incident queue",
        body: "Review active incidents by severity, filter the queue and share a link with its search conditions.",
      },
      {
        title: "Investigation with context",
        body: "Inspect recent changes, read-only diagnostics, declared dependencies and observed Kubernetes relationships.",
      },
      {
        title: "Analysis you can inspect",
        body: "Keep evidence and analysis history together. Review missing data and hypotheses before taking action.",
      },
      {
        title: "Confirmed solutions",
        body: "Look up previous resolutions and runbooks, with environment and version differences visible during review.",
      },
      {
        title: "Postmortems and handoffs",
        body: "Create versioned drafts, edit the narrative, review the result and export it for the next shift.",
      },
      {
        title: "Pipeline visibility",
        body: "Check component status and explicitly pause or resume AI calls when managed operations are enabled.",
      },
    ],
  },
  pages: {
    architecture: {
      title: "Architecture",
      description:
        "Components, alert flow, Redis correlation, sanitization boundary and failure scenarios.",
    },
    installation: {
      title: "Installation",
      description:
        "A Kubernetes deployment guide covering the umbrella chart, Telegram, OpenRouter, secrets and Alertmanager.",
    },
    configuration: {
      title: "Configuration",
      description:
        "Environment variables for every component, PromQL templates, runbooks and a values generator.",
    },
    webUi: {
      title: "Web UI",
      description:
        "Backend and UI components, alerts versus incidents, storage, access control and the analysis callback.",
    },
    logs: {
      title: "Logs enrichment",
      description:
        "Attaching a log excerpt to an alert: store requirements, settings and what reaches the model.",
    },
    examples: {
      title: "Examples",
      description:
        "Webhook payloads, cascade tests, PrometheusRule usage and production configuration.",
    },
    troubleshooting: {
      title: "Troubleshooting",
      description:
        "How to inspect Alertmanager, Redis, Telegram, OpenRouter, RBAC, the web UI and service logs.",
    },
    limitations: {
      title: "Limitations",
      description:
        "Clear boundaries for correlation, LLM output, storage, security and scaling.",
    },
  },
  architecture: {
    headings: {
      systemMap: "System map",
      alertFlow: "Alert flow",
      sequence: "Request sequence",
      correlation: "Correlation",
      sanitization: "Sanitization boundary",
      failures: "Failure scenarios",
    },
    diagramTitles: {
      system: "Architecture diagram",
      sequence: "Sequence diagram",
    },
    alertFlowSteps: [
      "Prometheus evaluates an alerting rule.",
      "Alertmanager sends a webhook to /alert.",
      "Enricher validates the payload.",
      "Enricher queries metrics around startsAt.",
      "Enricher reads Kubernetes pods, nodes and events, including deployments scaled to zero.",
      "Enricher pulls a collapsed error excerpt from the log store, if one is configured.",
      "The raw alert is published to Telegram right away.",
      "The alert is added to a Redis group.",
      "After CORR_SETTLE the group is closed.",
      "The group goes to backend, which stores the incident and forwards the body.",
      "AI Worker calls the LLM through Sanitizer.",
      "The analysis draft is published as a separate message.",
      "ANALYSIS_CALLBACK_URL returns the analysis to backend, so the web UI shows it too.",
    ],
    flowLabels: {
      input: "Input",
      action: "Action",
      output: "Output",
      dependency: "Dependency",
      error: "Possible error",
    },
    flowSteps: [
      {
        title: "Alert",
        input: "Alertmanager webhook payload",
        action: "Validate and normalize alerts",
        output: "EnrichedAlert candidate",
        dependency: "Alertmanager route and service DNS",
        error: "Bad JSON or empty alerts",
      },
      {
        title: "Enrichment",
        input: "Labels, annotations, startsAt, generatorURL",
        action:
          "Run Prometheus range queries, Kubernetes API reads and a log store search",
        output:
          "PromSample, K8sContext, collapsed log excerpt, notes and hints",
        dependency: "Prometheus API, ServiceAccount RBAC, optional log store",
        error: "Partial context with warnings in Enricher logs",
      },
      {
        title: "Correlation",
        input: "Enriched alert with namespace and fingerprint",
        action: "HSet into grp:{namespace}; wait CORR_SETTLE",
        output:
          "One group payload for backend, or for ai-worker when backend is disabled",
        dependency: "Redis",
        error: "Fallback to direct single-alert send",
      },
      {
        title: "Raw notification",
        input: "Enriched alert",
        action: "POST /incident-raw and Telegram sendMessage",
        output: "Raw alert visible in channel",
        dependency: "Telegram bot and channel permissions",
        error: "telegram_error response and ai-worker log entry",
      },
      {
        title: "LLM analysis",
        input: "Prometheus, Kubernetes and log context",
        action: "Chat completions request through Sanitizer",
        output: "Probable root cause, fix steps and confidence",
        dependency: "OpenRouter/OpenAI-compatible API",
        error: "Raw alert remains, analysis is skipped",
      },
      {
        title: "Final message",
        input: "Model answer",
        action:
          "Telegram reply or separate message, plus a callback to backend",
        output: "Engineering draft in the channel and in the web UI",
        dependency: "Telegram API, ANALYSIS_CALLBACK_URL for the web UI",
        error: "Log error; human still has raw alert",
      },
    ],
    correlationFacts: [
      {
        term: "Default",
        value: "namespace + time window",
      },
      {
        term: "Deduplication",
        value: "Alert fingerprint",
      },
      {
        term: "Durable mode",
        value: "Redis buffer and delivery, opt-in",
      },
      {
        term: "Dependencies",
        value: "One declared hop, opt-in",
      },
      {
        term: "Membership",
        value: "Manual alert regrouping, opt-in",
      },
    ],
    correlationCallout: {
      title: "Correlation modes",
      body: "The example above shows the baseline legacy algorithm. The current version also supports durable correlation, a declared dependency graph and manual incident membership. These require opt-in, compatible components and Redis. A relationship graph does not prove the cause of a failure.",
    },
    sanitizationParagraphs: [
      "Data Sanitizer is a separate Go service that masks secrets and personal data before anything reaches the external LLM, Telegram, logs or webhooks: sensitive keys, Bearer/Basic/JWT tokens, connection string credentials, Kubernetes Secrets, optionally emails, phones and IP addresses. The model answer passes through it again on the way back.",
      "Enricher and ai-worker authenticate to it with a shared HMAC key (SANITIZER_AUTH_SHARED_SECRET, identical in all three components). Production keeps SANITIZER_FAIL_CLOSED=true: if Sanitizer rejects a payload or is unavailable, the original data is not sent onwards.",
    ],
    failureHeaders: {
      component: "Component unavailable",
      behavior: "Current behavior from code or chart",
    },
    failures: [
      {
        component: "OpenRouter or LLM",
        behavior:
          "Raw alert is already posted; ai-worker logs the error and skips analysis.",
      },
      {
        component: "Redis",
        behavior:
          "Legacy mode can fall back to individual delivery and in-memory state. Durable mode requires Redis: queue guarantees are unavailable during an outage; check readiness and recovery.",
      },
      {
        component: "Prometheus API",
        behavior:
          "Metric query warnings are logged; enrichment continues with remaining context.",
      },
      {
        component: "Kubernetes API",
        behavior:
          "Client init or reads can fail; Enricher continues without K8s context.",
      },
      {
        component: "Log store",
        behavior:
          "The enrichment step is skipped silently; the alert travels without a log excerpt.",
      },
      {
        component: "Sanitizer",
        behavior:
          "With SANITIZER_FAIL_CLOSED=true the original payload is not sent to the LLM or Telegram.",
      },
      {
        component: "Telegram",
        behavior:
          "ai-worker returns telegram_error for sync sends and logs async send failures.",
      },
      {
        component: "Enricher",
        behavior: "Alertmanager cannot deliver to IncidentGPT.",
      },
      {
        component: "Backend",
        behavior:
          "The group path breaks: no incident in the web UI, and the body is not forwarded to ai-worker.",
      },
      {
        component: "AI Worker",
        behavior:
          "Enricher or backend cannot post the prepared incident; the failure is logged.",
      },
      {
        component: "Web UI",
        behavior:
          "Only the interface is lost; alerts and analysis keep flowing to Telegram.",
      },
    ],
  },
  installation: {
    browserAccessNote:
      "Browser access uses the backend reader Secret with username/password, not a legacy htpasswd file. Use compatible UI and backend images from the same verified version.",
    headings: {
      prerequisites: "Step 0. Prerequisites",
      telegram: "Step 1. Telegram",
      openrouter: "Step 2. OpenRouter",
      secrets: "Step 3. Secrets",
      helm: "Step 4. Deploy the umbrella chart",
      images: "Step 5. Images",
      alertmanager: "Step 6. Alertmanager",
      verify: "Step 7. Open the web UI",
    },
    prerequisites: [
      "Kubernetes cluster",
      "kubectl",
      "Helm 3",
      "kube-prometheus-stack",
      "Telegram bot",
      "Telegram channel",
      "Telegram discussion group",
      "OpenRouter or OpenAI-compatible API key",
      "Ingress controller for the web UI (optional)",
      "Log store with an OpenSearch or Elasticsearch API (optional)",
    ],
    redisCallout: {
      title: "Redis ships with the chart",
      body: "Since 0.2.0 Redis is a subchart: one node, 2 Gi disk, eviction disabled. Nothing to install separately. An existing Redis is used with incidentgpt-redis.enabled: false plus its address in incidentgpt-backend.env.REDIS_ADDR and incidentgpt-enricher.env.redisAddr.",
    },
    telegramSteps: [
      "Create a bot via @BotFather and store the token as TELEGRAM_BOT_TOKEN.",
      "Create a channel and a linked discussion group.",
      "Add the bot as an administrator in both places.",
      "Send a test message and inspect getUpdates for channel and group IDs.",
    ],
    telegramPrivacyNote:
      "Privacy mode can stay enabled: ai-worker does not read ordinary group messages.",
    telegramSecretsCallout: {
      title: "Secrets",
      body: "Never publish a real TELEGRAM_BOT_TOKEN in Git, Helm values or documentation.",
    },
    openrouterNote:
      "Any provider must expose an OpenAI-compatible chat completions endpoint because the code posts model, max_tokens and messages to OPENROUTER_BASE_URL.",
    secretsIntro:
      "Sanitizer and its clients share one HMAC key, so the sanitizer Secret is created once and referenced by enricher and ai-worker.",
    secretsNote:
      "The values generator on the Configuration page produces values that reference these Secrets through existingSecret and never contain the secrets themselves.",
    helmIntro:
      "All five components install with one release: enricher, ai-worker, sanitizer, backend and ui, plus Redis.",
    helmDisableNote:
      "Backend and ui are disabled with enabled: false on both. In that case return the enricher group path to ai-worker: incidentgpt-enricher.env.groupBackendUrl pointing at /incident-group.",
    imagesIntro:
      "Images are published to ghcr.io on every v* tag, so nothing needs to be built to try the project out.",
    imagesBuildNote:
      "Building your own is still supported — point the image repositories in values at your registry.",
    imagesArchNote:
      "Multi-arch builds can be attempted with linux/amd64,linux/arm64, but test the resulting images in your cluster.",
    alertmanagerNote:
      "route.receiver and receivers[].name must match exactly. continue: true keeps existing Slack, PagerDuty or other receivers active.",
    verifyIntro:
      "The UI retains nginx Basic authentication by default. Session sign-in is opt-in through authMode=session and browserSessions.enabled; it requires a backend reader Secret and HTTPS.",
  },
  configuration: {
    headings: {
      upgrade: "Upgrading from 0.1.0",
      enricherEnv: "Enricher environment",
      aiWorkerEnv: "AI Worker environment",
      backendEnv: "Backend environment",
      promql: "Prometheus metrics configuration",
      runbooks: "Runbooks",
      generator: "Helm values generator",
    },
    tableHeaders: {
      variable: "Variable",
      default: "Default",
      example: "Example",
      purpose: "Purpose",
    },
    upgradeCallout: {
      title: "Log store settings were renamed",
      body: "ELASTICSEARCH_URL, ELASTICSEARCH_USER, ELASTICSEARCH_PASSWORD and ELASTICSEARCH_INDEX became LOGS_STORE_*; in values elasticsearchUrl became logsStoreUrl and the elasticsearch block became logsStore. There are no aliases: the old keys are not read and log enrichment silently turns off. Image defaults also moved to ghcr.io/sersert/incidentgpt-* and tag 0.2.0.",
    },
    backendIntro:
      "Backend and UI are optional. With both disabled the delivery path stays exactly as in 0.1.0: enricher posts groups straight to ai-worker and everything lands in Telegram.",
    promqlIntro:
      "Metric templates use Go template placeholders such as {{ .Namespace }}, {{ .Cluster }}, {{ .Service }}, {{ .Node }}, {{ .Instance }} and, since 0.2.0, {{ .Pod }} and {{ .Container }} — so a query can be pinned to the failing pod instead of the whole namespace.",
    runbooksIntro:
      "RUNBOOK_BASE_URL is trimmed on the right and the lowercase alertname is appended. KubePodCrashLooping becomes https://runbooks.example.com/alerts/kubepodcrashlooping.",
    metricLabels: {
      unit: "Unit",
      trend: "Trend",
      series: "Series",
      cardinality: "Cardinality",
    },
    env: {
      enricher: {
        PROMETHEUS_URL: {
          description:
            "Prometheus HTTP API base URL. Enricher calls /api/v1/query_range.",
        },
        PYTHON_BACKEND_URL: {
          description:
            "Single-alert fallback endpoint used when Redis correlation is disabled or fails.",
          note: "The code name is legacy; Helm points it to ai-worker.",
        },
        GROUP_BACKEND_URL: {
          description: "Endpoint for a flushed Redis group.",
          note: "Since 0.2.0 the umbrella chart points it at backend, which stores the incident and forwards the body to ai-worker. With backend disabled, point it back at /incident-group.",
        },
        RAW_BACKEND_URL: {
          description:
            "Endpoint used for immediate raw alert delivery while the group window is still open.",
        },
        CLUSTER_NAME: {
          description:
            "Cluster label included in enriched payloads and metric templates.",
        },
        ENVIRONMENT: {
          description: "Environment label added to the incident context.",
        },
        REDIS_ADDR: {
          description: "Redis address for namespace-window correlation.",
          note: "Redis ships inside the chart since 0.2.0, where this is incidentgpt-redis:6379. An external one is set here and in incidentgpt-backend.env.REDIS_ADDR: the two components reach Redis independently.",
        },
        REDIS_PASSWORD: {
          description:
            "Redis password. The chart renders it as a plain env value.",
          note: "Recommended improvement: secretKeyRef support for production.",
        },
        CORR_WINDOW: { description: "Redis TTL for grp:{namespace}." },
        CORR_SETTLE: {
          description: "Debounce interval before a group is flushed.",
          note: "Helm values ship 40s, which covers the spread between rule types.",
        },
        RAW_DEDUP_TTL: {
          description:
            "TTL for Redis SetNX based raw-alert deduplication by fingerprint. 0s disables the dedup key.",
        },
        PROM_RANGE_BEFORE: {
          description: "Metrics window before alert startsAt.",
        },
        PROM_RANGE_AFTER: {
          description: "Metrics window after alert startsAt.",
        },
        LOGS_STORE_URL: {
          description:
            "Search API of the log store. Empty value silently skips the log enrichment step.",
          note: "Renamed from ELASTICSEARCH_URL in 0.2.0. The old names are not read, and there are no aliases.",
        },
        LOGS_STORE_USER: {
          description:
            "Log store user. The password comes from LOGS_STORE_PASSWORD via logsStore.existingSecret.",
        },
        LOGS_STORE_INDEX: {
          description: "Index pattern searched for alert-related log lines.",
        },
        LOGS_RANGE_BEFORE: {
          description: "How far before the alert log lines are collected.",
        },
        LOGS_RANGE_AFTER: {
          description: "How far after the alert log lines are collected.",
        },
        LOGS_MAX_LINES: {
          description:
            "How many collapsed log groups are placed into the prompt.",
        },
        LOGS_BASE_URL: {
          description:
            "Base for a human-facing log UI link added to the alert.",
          note: "Not the same as LOGS_STORE_URL: this one is only a link for an engineer, not an API the enricher calls.",
        },
        SANITIZER_URL: {
          description:
            "Sanitizer service that masks secrets and personal data before the payload leaves the cluster.",
        },
        SANITIZER_AUTH_SHARED_SECRET: {
          description:
            "HMAC key for requests to Sanitizer. Must match the key configured in sanitizer and ai-worker.",
        },
        RUNBOOK_BASE_URL: {
          description:
            "Base URL used to create runbook_url as base/lowercase-alertname.",
        },
        ENRICH_CLUSTER_CONTEXT: {
          description: "Enables cluster-level PromQL templates.",
        },
        ENRICH_NODE_CONTEXT: {
          description: "Enables node-level PromQL templates.",
        },
        ENRICH_WORKLOAD_CONTEXT: {
          description: "Enables workload-level PromQL templates.",
        },
        ENRICH_EXTERNAL_CONTEXT: {
          description: "Enables external dependency PromQL templates.",
        },
        ENRICH_K8S_CONTEXT: {
          description: "Enables in-cluster Kubernetes API enrichment.",
        },
      },
      aiWorker: {
        LISTEN_ADDR: {
          description:
            "HTTP listen address for /healthz, /incident, /incident-raw and /incident-group.",
        },
        OPENROUTER_API_KEY: {
          description:
            "Required. API key for OpenRouter or another OpenAI-compatible chat completions endpoint.",
        },
        OPENROUTER_BASE_URL: { description: "Chat completions endpoint." },
        OPENROUTER_MODEL: {
          description: "Model sent in the chat completions payload.",
        },
        OPENROUTER_TIMEOUT_SECONDS: {
          description: "HTTP timeout for the LLM request.",
          note: "The older values key REQUEST_TIMEOUT_SECONDS is not read by the Go code.",
        },
        OPENROUTER_MAX_TOKENS: {
          description: "Maximum model response tokens.",
          note: "Helm values ship 2000.",
        },
        TELEGRAM_BOT_TOKEN: {
          description: "Required. Bot token from @BotFather.",
        },
        TELEGRAM_CHANNEL_ID: {
          description:
            "Required. Channel receiving raw alerts, summaries and analysis messages.",
        },
        TELEGRAM_THREAD_CHAT_ID: {
          description:
            "Discussion group ID stored in config. Replies are sent to the channel post and appear in the linked discussion group when Telegram is configured that way.",
        },
        TELEGRAM_PARSE_MODE: {
          description: "Telegram parse mode used for sendMessage.",
        },
        ANALYSIS_CALLBACK_URL: {
          description:
            "Where the finished analysis is posted back so the web UI can show it.",
          note: "New in 0.2.0. Without it the analysis goes to Telegram only and the incident stays in the waiting state in the web UI.",
        },
        SANITIZER_URL: {
          description:
            "Sanitizer masks the prompt before the LLM call and the model answer before Telegram.",
        },
        SANITIZER_AUTH_SHARED_SECRET: {
          description:
            "HMAC key shared with sanitizer and enricher. All three components must carry the same value.",
          note: "Sanitizer itself keeps SANITIZER_FAIL_CLOSED=true by default: if it rejects a payload or is unavailable, the original data is not sent to the LLM or Telegram.",
        },
      },
      backend: {
        LISTEN_ADDR: {
          description:
            "HTTP listen address for the incident API consumed by the UI.",
        },
        AI_WORKER_URL: {
          description:
            "Where the group body is forwarded after the incident is stored.",
          note: "The path is required. Without it the forward hits the root and gets a 404; Helm values set /incident-group.",
        },
        FORWARD_TO_AI_WORKER: {
          description:
            "Keeps the Telegram path working: backend passes the group body through unchanged.",
        },
        REDIS_ADDR: {
          description:
            "Incident storage. Without Redis the service keeps state in memory and any pod restart wipes the history.",
          note: "Helm values ship incidentgpt-redis:6379.",
        },
        PROMETHEUS_URL: {
          description:
            "Metrics source for the dashboard and per-node forecasts.",
        },
        LOGS_STORE_URL: {
          description:
            "Log store for the Logs tab of an incident. Empty value leaves the tab unfilled.",
        },
        LOGS_STORE_INDEX: {
          description: "Index pattern used by the Logs tab.",
        },
      },
    },
    metrics: {
      workload_cpu_usage: {
        description: "CPU usage around the alert start time.",
        unit: "CPU cores",
        series: "One or more series per namespace/pod after aggregation.",
        trend:
          "rising means the second half of the sampled range is at least 15% above the first half.",
        risk: "High cardinality if pod/container labels are not aggregated before storage in the alert context.",
      },
      workload_memory_working_set: {
        description: "Working set memory for pods in the alert namespace.",
        unit: "bytes",
        series: "Usually one series per pod after sum by pod.",
        trend:
          "falling means the later samples are at least 15% lower than the earlier samples.",
        risk: "Memory metrics can be noisy during pod churn; keep namespace scoped.",
      },
      workload_pod_restarts: {
        description: "Recent container restarts in the namespace.",
        unit: "restart count",
        series: "One series per pod/container unless aggregated.",
        trend:
          "spike is detected when the last point is more than twice the average.",
        risk: "Container-level output can be large in busy namespaces; aggregate when possible.",
      },
    },
  },
  webUi: {
    headings: {
      components: "Components",
      wiring: "Where backend fits",
      alertsVsIncidents: "Alerts and incidents are different objects",
      storage: "Storage",
      access: "Access",
      screens: "What the interface shows",
      dataSources: "Data sources",
    },
    componentsIntro:
      "Incidents can be inspected in a browser, not only in Telegram. Two components handle that, both part of the umbrella chart:",
    components: [
      {
        name: "backend",
        body: "accepts alert groups from enricher, stores incidents and forwards the body to ai-worker.",
      },
      {
        name: "ui",
        body: "the interface itself: nginx serving static files and proxying /api/ to backend.",
      },
    ],
    componentsDisable:
      "Disable backend and UI with enabled: false for Telegram-only delivery.",
    wiringIntro:
      "Default legacy path: backend accepts groups while raw alerts go directly to ai-worker. The advanced durable mode is enabled separately.",
    wiringPassthrough:
      "Backend passes the body through unchanged, to the same endpoint enricher used to call directly, so Telegram analysis keeps working. Without ANALYSIS_CALLBACK_URL the analysis reaches Telegram only and the incident stays in the waiting state in the UI.",
    wiringNote:
      "The umbrella values already wire both settings; they matter when components are installed separately.",
    alertsVsIncidents: [
      "An alert is a single Alertmanager signal. An incident brings related signals, context and analysis history together.",
      "Group membership depends on the configured correlation mode. With manual regrouping enabled, an engineer can move one alert while preserving history.",
    ],
    storageIntro:
      "Incidents live in the same Redis the enricher uses for correlation. It ships as the incidentgpt-redis subchart: one node, no replication or sentinel, 2 Gi disk.",
    storageAddress:
      "The address is set in two places: enricher and backend reach Redis independently.",
    storageEviction:
      "Eviction is deliberately off — maxmemory-policy stays noeviction. Any allkeys-* policy would drop incidents under load, and this is storage, not a cache.",
    storageCallout: {
      title: "Without Redis history does not survive a restart",
      body: "The service still runs and keeps state in memory, but any pod restart wipes the history. For anything beyond a demo an archive needs a real database.",
    },
    accessIntro:
      "Basic/htpasswd remains the default. Enable eight-hour server sessions with authMode=session, browserSessions.enabled=true and a backend reader Secret. Upgrade UI and backend together.",
    accessProbe:
      "/healthz remains public for kubelet — otherwise it would get a 401 and the pod would never become ready. Nginx clears incoming X-Auth-User: Basic identity is verified by nginx, session identity is restored by the backend for the action log. This is a shared administrator account, without individual roles or SSO.",
    screens: [
      {
        name: "On-call overview",
        body: "Available measurements and up to 10 active incidents. API failures are shown as unknown state.",
      },
      {
        name: "Incident queue",
        body: "Search, status and severity filters, and pages of 25 records. Conditions stay in the URL.",
      },
      {
        name: "Incident detail",
        body: "Four views: overview and analysis, investigation, solutions and runbooks, reports and handoff.",
      },
      {
        name: "Alerts, analytics and forecasts",
        body: "Raw signals, noisy sources, analysis quality and resource trends. Forecasts do not guarantee future load.",
      },
      {
        name: "Component status",
        body: "With opt-in enabled: source and component checks, AI state and a Redis-backed pause.",
      },
      {
        name: "Guide and themes",
        body: "An embedded operator guide, light and dark modes, mobile navigation and keyboard support.",
      },
    ],
    dataSourcesIntro:
      "Integration forms describe sources by API type. Saving a form does not enable collection: a compatible, configured backend/enricher and a reachable source are required.",
    dataSourceHeaders: {
      category: "Category",
      type: "Type",
      worksWith: "Works with",
    },
    dataSources: [
      {
        category: "Metrics",
        type: "Prometheus API",
        worksWith: "Prometheus, VictoriaMetrics, Thanos, Mimir",
      },
      { category: "Metrics", type: "Zabbix API", worksWith: "Zabbix" },
      {
        category: "Logs",
        type: "OpenSearch / Elasticsearch API",
        worksWith: "OpenSearch, ELK",
      },
      { category: "Logs", type: "Loki API", worksWith: "Loki" },
    ],
    dataSourcesOutside:
      "Kubernetes and Telegram are configured outside the interface: the first through the pod service account, the second through the ai-worker environment. The UI only shows their state.",
  },
  logs: {
    headings: {
      why: "Why logs change the analysis",
      requirements: "What the store must provide",
      wiring: "Connecting a store",
      collector: "Collector example: fluent-bit",
      prompt: "What reaches the model",
    },
    whyParagraphs: [
      'Besides metrics and Kubernetes context, enricher can attach a log excerpt to an alert. Without logs the model writes "check the pod logs"; with logs it names the actual error. Since 0.2.0 the analysis is not allowed to hand data collection back to the engineer: collecting data is the system\'s job, and when it is missing the model writes "Не хватает данных:" — a signal to widen collection, not a task for a human.',
      "Log collection is not part of the chart: clusters usually already run a stack, and a second one is pointless. Any store with an OpenSearch or Elasticsearch compatible search API works.",
    ],
    requirementsIntro:
      "Enricher sends a plain _search and expects documents with these fields:",
    fieldHeaders: { field: "Field", content: "Content" },
    fields: [
      { field: "@timestamp", content: "line timestamp" },
      { field: "log or message", content: "the text itself" },
      { field: "kubernetes.namespace_name", content: "pod namespace" },
      { field: "kubernetes.pod_name", content: "pod name" },
    ],
    requirementsSource:
      "That structure comes from fluent-bit with the kubernetes filter, and equally from filebeat or fluentd with similar enrichment.",
    mappingCallout: {
      title: "Mapping pitfall",
      body: "String fields are indexed as text with a keyword subfield, and an exact term match only works against .keyword. A query without the suffix silently returns zero documents — no error, just an empty result. Enricher already queries .keyword.",
    },
    wiringNote:
      "Without logsStoreUrl the enrichment step is silently skipped and the component keeps working without logs. The same address is what backend needs for the Logs tab of an incident.",
    baseUrlCallout: {
      title: "Not the same as logsBaseUrl",
      body: "logsBaseUrl is the base for a link to a log web interface that an engineer opens by hand. logsStoreUrl is the API address the enricher calls itself.",
    },
    collectorIntro:
      "A DaemonSet reads logs from the nodes. Filtering by file path is better than filtering later — unwanted logs are never read from disk:",
    collectorNote:
      "For Elasticsearch use Name es instead of Name opensearch. The collector needs a hostPath to /var/log; policies such as Kyverno disallow-host-path will complain, and for a log-collecting DaemonSet that is unavoidable.",
    promptIntro:
      "Hundreds of lines cannot go into the prompt: it inflates, costs more and drowns what matters. So enricher:",
    promptRules: [
      "drops INFO — causes are found in errors;",
      "collapses repeats — timestamps, numbers and hashes are stripped, and the rest counts as one error;",
      "sorts FATAL → ERROR → WARN, and by frequency inside a level;",
      "takes logs from the whole namespace but puts the alert pod first: the cause often sits with a neighbour.",
    ],
    promptResult: "Five hundred lines turn into a couple of dozen:",
    promptCounter:
      "The ×N counter is a signal in itself: a single failure or a storm.",
  },
  examples: {
    headings: {
      direct: "Direct Enricher test",
      cascade: "Cascade test",
      prometheusRule: "PrometheusRule test",
      production: "Production configuration",
      telegramMock: "Telegram output mockup",
    },
    cascadeNote:
      "All alerts share the same namespace and are sent inside CORR_SETTLE, so they should be grouped into one Redis key.",
    prometheusRuleNote:
      "The existing sample uses release: kps. Adjust it to the label selected by your Prometheus.",
    environments: ["Development", "Staging", "Production"],
    productionItems: [
      "Use non-latest image tags and pinned chart versions; the release tag is 0.2.0.",
      "Use Kubernetes Secrets for OpenRouter, Telegram and the Sanitizer HMAC key.",
      "Create a backend reader Secret with a dedicated username and strong password for sign-in.",
      "Enable Redis authentication and persistence; keep maxmemory-policy at noeviction, since incidents are storage rather than cache.",
      "Set resource requests and limits; probes already exist in every chart.",
      "Keep SANITIZER_FAIL_CLOSED=true so an unavailable Sanitizer never leaks raw payloads.",
      "Recommended improvement: PodDisruptionBudget and Redis secretKeyRef support.",
      "Keep logs free of secrets; do not enable DEBUG_PROMPT or other debug payload logging in production.",
    ],
    mockCallout: {
      title: "Mockup only",
      body: "This visual example does not call Telegram API and does not contain real chat IDs or tokens. The analysis is shown in Russian because the ai-worker system prompt is written in Russian; change the prompt in the source to answer in another language.",
    },
  },
  troubleshooting: {
    cards: [
      {
        title: "Alertmanager does not send webhook",
        checks: [
          "receiver name mismatch",
          "wrong service DNS or namespace",
          "port 9099 or path /alert",
          "NetworkPolicy or missing endpoints",
        ],
      },
      {
        title: "PrometheusRule does not fire",
        checks: [
          "wrong release label",
          "Prometheus rule selector does not match",
          "PromQL returns no series",
          "for duration too long",
          "wrong namespace",
        ],
      },
      {
        title: "Enricher receives no Prometheus metrics",
        checks: [
          "Prometheus URL",
          "service endpoints",
          "NetworkPolicy",
          "PromQL syntax",
          "auth proxy in front of Prometheus",
        ],
      },
      {
        title: "No Kubernetes context",
        checks: [
          "ServiceAccount name",
          "ClusterRoleBinding",
          "pods/events/nodes permissions",
          "ENRICH_K8S_CONTEXT",
        ],
      },
      {
        title: "Redis connection refused",
        checks: [
          "redis service name",
          "endpoints",
          "password",
          "NetworkPolicy",
          "address set in both enricher and backend",
        ],
      },
      {
        title: "Alerts are not grouped",
        checks: [
          "same namespace",
          "fingerprint present or stable labels",
          "Redis works",
          "sent inside CORR_SETTLE",
          "look for ALERT_BUFFERED and GROUP_SENT",
        ],
      },
      {
        title: "Telegram returns an error",
        checks: [
          "bot token",
          "channel ID starts with -100",
          "discussion group linked",
          "bot administrator rights",
          "Telegram API response in logs",
        ],
      },
      {
        title: "Web UI answers 502 on every request",
        checks: [
          "backend deployment is running",
          "backend service is named incidentgpt-backend",
          "nginx in ui proxies /api/ to that exact name",
          "fullnameOverride was not changed",
        ],
      },
      {
        title: "Incidents stay in the waiting-for-analysis state",
        checks: [
          "ANALYSIS_CALLBACK_URL is set on ai-worker",
          "enricher groupBackendUrl points at backend ingest",
          "backend forwards to ai-worker with FORWARD_TO_AI_WORKER",
          "AI_WORKER_URL includes the /incident-group path",
        ],
      },
      {
        title: "No logs in the incident or in the prompt",
        checks: [
          "LOGS_STORE_URL is empty, so the step is skipped silently",
          "index pattern does not match",
          "old ELASTICSEARCH_* names still in values",
          "store credentials",
          "term queries need the .keyword subfield",
        ],
      },
      {
        title: "OpenRouter returns 401",
        checks: [
          "wrong key",
          "Secret not mounted",
          "newline in value",
          "wrong endpoint",
          "variable missing in pod",
        ],
      },
    ],
  },
  limitations: {
    headings: {
      correlation: "Correlation",
      llm: "LLM",
      storage: "Storage",
      security: "Security",
      scaling: "Scaling",
    },
    correlation: {
      behavior:
        "The default uses namespace and a time window. Durable correlation and grouping by declared dependencies are separate opt-ins.",
      risk: "Declared dependencies can become stale; ownership and placement do not prove causality. Missing data triggers a bounded fallback.",
      recommended:
        "Review declaration freshness and group membership. Enable new modes only after verifying an isolated candidate.",
      items: [
        "Declared-dependency traversal is limited to one hop.",
        "Graph and manual regrouping are disabled by default.",
        "Confirmed solutions and MCP require separate configuration.",
      ],
    },
    llmItems: [
      "The model can be wrong or hallucinate.",
      "Output depends on context quality.",
      "Commands must be reviewed before execution.",
      "AI analysis must not be the only source for incident decisions.",
    ],
    llmCallout: { title: "Human decision" },
    storage: {
      behavior:
        "incidents and alerts live in Redis — a single node, no replication or sentinel, 2 Gi disk, eviction disabled on purpose.",
      risk: "without Redis the backend keeps state in memory and any pod restart wipes the history; a full disk stops accepting new incidents rather than dropping old ones.",
      recommended:
        "a real database for an archive and for training on past incidents.",
    },
    securityCallout: {
      title: "Sensitive data",
      body: "Sanitizer masks secrets, tokens and personal data before anything leaves the cluster, but it is a filter with rules, not a guarantee. Treat the model as an external party: do not widen enrichment to full configuration dumps or Kubernetes Secrets and expect masking to catch everything.",
    },
    security: {
      behavior:
        "Basic/htpasswd remains the default. Opt-in sign-in creates a server session: nginx checks access to /api/ and /ws/, and the backend restores the verified identity for the action log.",
      risk: "one shared account, no roles, no SSO, no per-user audit beyond that header.",
      recommended:
        "put an authenticating proxy in front of the Ingress when the incident history needs real access control.",
    },
    scaling: [
      {
        title: "Legacy and durable modes",
        behavior:
          "Legacy timers are process-local. Durable mode stores buffering and delivery state in Redis.",
        recommended:
          "Select the mode explicitly; drain older writers and queues before switching.",
      },
      {
        title: "Redis without HA",
        behavior:
          "The bundled Redis is a single primary. History, queues and AI control depend on its availability and persistence.",
        recommended: "Test backup restoration and define an archive policy.",
      },
      {
        title: "Throughput and cost",
        behavior:
          "Telegram and provider limits, plus context size, constrain throughput.",
        recommended:
          "Bound context size and verify queues, retries and spending under load.",
      },
    ],
  },
  generator: {
    title: "Helm values generator",
    description:
      "Secrets are not requested. Create a Kubernetes Secret separately and reference it with existingSecret.",
    umbrella: "values-incidentgpt.yaml",
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
      logsStoreUrl: "Log store URL",
      uiHost: "Web UI hostname",
      registry: "Container registry",
      imageVersion: "Image version",
    },
  },
} as const satisfies Translation;
