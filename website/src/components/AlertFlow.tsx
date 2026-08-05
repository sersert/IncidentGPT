import { useState } from "react";

type FlowStep = {
  title: string;
  input: string;
  action: string;
  output: string;
  dependency: string;
  error: string;
};

const steps: FlowStep[] = [
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
    action: "Run Prometheus range queries and Kubernetes API reads",
    output: "PromSample, K8sContext, notes and hints",
    dependency: "Prometheus API, ServiceAccount RBAC",
    error: "Partial context with warnings in Enricher logs",
  },
  {
    title: "Correlation",
    input: "Enriched alert with namespace and fingerprint",
    action: "HSet into grp:{namespace}; wait CORR_SETTLE",
    output: "One group payload for ai-worker",
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
    input: "Prometheus and Kubernetes context",
    action: "Chat completions request",
    output: "Probable root cause and checks",
    dependency: "OpenRouter/OpenAI-compatible API",
    error: "Raw alert remains, analysis is skipped",
  },
  {
    title: "Final message",
    input: "Model answer",
    action: "Telegram reply or separate message",
    output: "Engineering draft in channel",
    dependency: "Telegram API",
    error: "Log error; human still has raw alert",
  },
];

export function AlertFlow() {
  const [active, setActive] = useState(0);
  const step = steps[active];

  return (
    <section className="flow-explorer" aria-label="Incident flow explorer">
      <div className="flow-tabs" role="tablist">
        {steps.map((item, index) => (
          <button key={item.title} type="button" className={index === active ? "selected" : ""} onClick={() => setActive(index)}>
            {item.title}
          </button>
        ))}
      </div>
      <div className="flow-detail">
        <h3>{step.title}</h3>
        <dl>
          <div>
            <dt>Input</dt>
            <dd>{step.input}</dd>
          </div>
          <div>
            <dt>Action</dt>
            <dd>{step.action}</dd>
          </div>
          <div>
            <dt>Output</dt>
            <dd>{step.output}</dd>
          </div>
          <div>
            <dt>Dependency</dt>
            <dd>{step.dependency}</dd>
          </div>
          <div>
            <dt>Possible error</dt>
            <dd>{step.error}</dd>
          </div>
        </dl>
      </div>
    </section>
  );
}
