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
    Enricher -->|Error excerpt| LogStore[(Log store)]
    Enricher -->|Buffer and correlation| Redis[(Redis)]
    Enricher -->|Masking| Sanitizer[Data Sanitizer]
    Enricher -->|POST group /api/v1/ingest| Backend[IncidentGPT Backend]
    Backend --> Redis
    Backend -->|POST /incident-group| AIWorker[IncidentGPT AI Worker]
    Backend --> UI[IncidentGPT Web UI]
    AIWorker -->|Masking| Sanitizer
    AIWorker -->|OpenAI-compatible API| LLM[OpenRouter / LLM]
    AIWorker -->|Raw alerts and analysis| Telegram[Telegram Channel]
    AIWorker -->|ANALYSIS_CALLBACK_URL| Backend
    Telegram --> Discussion[Linked Discussion Group]`;

const sequenceChart = `sequenceDiagram
    participant AM as Alertmanager
    participant EN as Enricher
    participant PR as Prometheus
    participant K8S as Kubernetes API
    participant LG as Log store
    participant RD as Redis
    participant BE as Backend
    participant AI as AI Worker
    participant LLM as OpenRouter
    participant TG as Telegram
    AM->>EN: POST /alert
    EN->>PR: query_range
    PR-->>EN: metric samples
    EN->>K8S: pods, nodes, events
    K8S-->>EN: cluster context
    EN->>LG: _search around startsAt
    LG-->>EN: collapsed error excerpt
    EN->>AI: POST /incident-raw
    AI->>TG: publish raw alert
    EN->>RD: save incident candidate
    EN->>RD: wait CORR_SETTLE
    RD-->>EN: grouped alerts
    EN->>BE: POST /api/v1/ingest
    BE->>RD: store incident
    BE->>AI: POST /incident-group
    AI->>LLM: request incident analysis
    LLM-->>AI: probable cause and checks
    AI->>TG: publish AI analysis
    AI->>BE: ANALYSIS_CALLBACK_URL`;

const pseudoCode = `key := "grp:" + alert.Namespace

redis.HSet(ctx, key, alert.Fingerprint, payload)
redis.Expire(ctx, key, corrWindow)

if firstAlertInGroup {
    scheduleFlush(key, corrSettle)
}`;

export function ArchitecturePage({ t }: PageProps) {
  const page = t.architecture;
  return (
    <article className="doc-page">
      <section id="system-map">
        <h2>{page.headings.systemMap}</h2>
        <ArchitectureDiagram chart={architectureChart} fallbackTitle={page.diagramTitles.system} copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="alert-flow">
        <h2>{page.headings.alertFlow}</h2>
        <ol className="steps">
          {page.alertFlowSteps.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ol>
        <AlertFlow t={t} />
      </section>
      <section id="sequence">
        <h2>{page.headings.sequence}</h2>
        <ArchitectureDiagram chart={sequenceChart} fallbackTitle={page.diagramTitles.sequence} copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="correlation">
        <h2>{page.headings.correlation}</h2>
        <div className="fact-grid">
          {page.correlationFacts.map((fact) => (
            <div key={fact.term}>
              <strong>{fact.term}</strong>
              <span>{fact.value}</span>
            </div>
          ))}
        </div>
        <CodeBlock code={pseudoCode} language="go" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <Callout title={page.correlationCallout.title} tone="warning">
          <p>{page.correlationCallout.body}</p>
        </Callout>
      </section>
      <section id="sanitization">
        <h2>{page.headings.sanitization}</h2>
        {page.sanitizationParagraphs.map((paragraph) => (
          <p key={paragraph}>{paragraph}</p>
        ))}
      </section>
      <section id="failures">
        <h2>{page.headings.failures}</h2>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{page.failureHeaders.component}</th>
                <th>{page.failureHeaders.behavior}</th>
              </tr>
            </thead>
            <tbody>
              {page.failures.map((row) => (
                <tr key={row.component}>
                  <td>{row.component}</td>
                  <td>{row.behavior}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </article>
  );
}
