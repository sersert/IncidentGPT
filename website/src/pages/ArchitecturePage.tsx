import type { Translation } from "../i18n/types";
import { AlertFlow } from "../components/AlertFlow";
import { ArchitectureDiagram } from "../components/ArchitectureDiagram";
import { Callout } from "../components/Callout";
import { CodeBlock } from "../components/CodeBlock";

type PageProps = { t: Translation };

const architectureChart = `flowchart TD
    K8s[Kubernetes workloads] --> Prometheus[Prometheus]
    Prometheus --> Alertmanager[Alertmanager]
    Alertmanager -->|Webhook /alert| Enricher[IncidentGPT Enricher]
    Enricher -->|Range queries| Prometheus
    Enricher -->|Pods, Nodes, Events| K8sAPI[Kubernetes API]
    Enricher -->|Buffer and correlation| Redis[(Redis)]
    Enricher -->|POST /incident| AIWorker[IncidentGPT AI Worker]
    AIWorker -->|OpenAI-compatible API| LLM[OpenRouter / LLM]
    AIWorker -->|Raw alerts and analysis| Telegram[Telegram Channel]
    Telegram --> Discussion[Linked Discussion Group]`;

const sequenceChart = `sequenceDiagram
    participant AM as Alertmanager
    participant EN as Enricher
    participant PR as Prometheus
    participant K8S as Kubernetes API
    participant RD as Redis
    participant AI as AI Worker
    participant LLM as OpenRouter
    participant TG as Telegram
    AM->>EN: POST /alert
    EN->>PR: query_range
    PR-->>EN: metric samples
    EN->>K8S: pods, nodes, events
    K8S-->>EN: cluster context
    EN->>RD: save incident candidate
    EN->>RD: wait CORR_SETTLE
    RD-->>EN: grouped alerts
    EN->>AI: POST /incident-group
    AI->>TG: publish raw alerts
    AI->>LLM: request incident analysis
    LLM-->>AI: probable cause and checks
    AI->>TG: publish AI analysis`;

const pseudoCode = `key := "grp:" + alert.Namespace

redis.HSet(ctx, key, alert.Fingerprint, payload)
redis.Expire(ctx, key, corrWindow)

if firstAlertInGroup {
    scheduleFlush(key, corrSettle)
}`;

export function ArchitecturePage({ t }: PageProps) {
  return (
    <article className="doc-page">
      <section id="system-map">
        <h2>System map</h2>
        <ArchitectureDiagram chart={architectureChart} fallbackTitle="Architecture diagram" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="alert-flow">
        <h2>Alert flow</h2>
        <ol className="steps">
          {[
            "Prometheus evaluates an alerting rule.",
            "Alertmanager sends a webhook to /alert.",
            "Enricher validates the payload.",
            "Enricher queries metrics around startsAt.",
            "Enricher reads Kubernetes pods, nodes and events.",
            "The alert is added to a Redis group.",
            "After CORR_SETTLE the group is closed.",
            "The incident is sent to ai-worker.",
            "Raw alert is published to Telegram.",
            "AI Worker calls the LLM.",
            "The analysis draft is published as a separate message.",
          ].map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ol>
        <AlertFlow />
      </section>
      <section id="sequence">
        <h2>Request sequence</h2>
        <ArchitectureDiagram chart={sequenceChart} fallbackTitle="Sequence diagram" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="correlation">
        <h2>Correlation</h2>
        <div className="fact-grid">
          <div><strong>Correlation key</strong><span>grp:{"{namespace}"}</span></div>
          <div><strong>Membership</strong><span>same namespace + received during settle interval</span></div>
          <div><strong>Redis field</strong><span>alert fingerprint</span></div>
          <div><strong>Lifetime</strong><span>CORR_WINDOW</span></div>
          <div><strong>Debounce</strong><span>CORR_SETTLE</span></div>
        </div>
        <CodeBlock code={pseudoCode} language="go" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <Callout title="Deterministic, not causal" tone="warning">
          <p>
            This is not a dependency graph. Two unrelated incidents in the same namespace may be grouped, and cross-namespace cascades may be missed. The model chooses the probable root after the group is formed.
          </p>
        </Callout>
      </section>
      <section id="failures">
        <h2>Failure scenarios</h2>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Component unavailable</th>
                <th>Current behavior from code or chart</th>
              </tr>
            </thead>
            <tbody>
              {[
                ["OpenRouter or LLM", "Raw alert is already posted; ai-worker logs the error and skips analysis."],
                ["Redis", "Enricher logs a warning and falls back to single-alert delivery."],
                ["Prometheus API", "Metric query warnings are logged; enrichment continues with remaining context."],
                ["Kubernetes API", "Client init or reads can fail; Enricher continues without K8s context."],
                ["Telegram", "ai-worker returns telegram_error for sync sends and logs async send failures."],
                ["Enricher", "Alertmanager cannot deliver to IncidentGPT."],
                ["AI Worker", "Enricher cannot post the prepared incident; the failure is logged."],
              ].map(([component, behavior]) => (
                <tr key={component}>
                  <td>{component}</td>
                  <td>{behavior}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </article>
  );
}
