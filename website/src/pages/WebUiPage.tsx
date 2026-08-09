import { Callout } from "../components/Callout";
import { CodeBlock } from "../components/CodeBlock";
import type { Translation } from "../i18n/types";

type PageProps = { t: Translation };

const groupPath = `raw   → ai-worker                            → Telegram
group → backend → ai-worker /incident-group  → Telegram + Web UI
       ↑                                   ↓
       enricher                     ANALYSIS_CALLBACK_URL`;

const wiringValues = `incidentgpt-enricher:
  env:
    groupBackendUrl: "http://incidentgpt-backend:8080/api/v1/ingest"

ai-worker:
  env:
    ANALYSIS_CALLBACK_URL: "http://incidentgpt-backend:8080/api/v1/incidents/by-group/analysis"`;

const externalRedis = `incidentgpt-redis:
  enabled: false

incidentgpt-backend:
  env:
    REDIS_ADDR: "redis-master.infra.svc.cluster.local:6379"
incidentgpt-enricher:
  env:
    redisAddr: "redis-master.infra.svc.cluster.local:6379"`;

const authValues = `incidentgpt-ui:
  ingress:
    enabled: true
    className: nginx
    host: incidentgpt.example.com
  auth:
    enabled: true
    username: admin
    existingSecret: incidentgpt-ui-auth`;

export function WebUiPage({ t }: PageProps) {
  const page = t.webUi;
  const code = (value: string, language: string) => (
    <CodeBlock code={value} language={language} copyLabel={t.common.copy} copiedLabel={t.common.copied} />
  );
  return (
    <article className="doc-page">
      <section id="components">
        <h2>{page.headings.components}</h2>
        <p>{page.componentsIntro}</p>
        <ul>
          {page.components.map((item) => (
            <li key={item.name}>
              <strong>{item.name}</strong> — {item.body}
            </li>
          ))}
        </ul>
        <p>{page.componentsDisable}</p>
      </section>
      <section id="wiring">
        <h2>{page.headings.wiring}</h2>
        <p>{page.wiringIntro}</p>
        {code(groupPath, "text")}
        <p>{page.wiringPassthrough}</p>
        {code(wiringValues, "yaml")}
        <p className="muted">{page.wiringNote}</p>
      </section>
      <section id="alerts-vs-incidents">
        <h2>{page.headings.alertsVsIncidents}</h2>
        {page.alertsVsIncidents.map((paragraph) => (
          <p key={paragraph}>{paragraph}</p>
        ))}
      </section>
      <section id="storage">
        <h2>{page.headings.storage}</h2>
        <p>{page.storageIntro}</p>
        {code(externalRedis, "yaml")}
        <p>{page.storageAddress}</p>
        <p>{page.storageEviction}</p>
        <Callout title={page.storageCallout.title} tone="warning">
          <p>{page.storageCallout.body}</p>
        </Callout>
      </section>
      <section id="access">
        <h2>{page.headings.access}</h2>
        <p>{page.accessIntro}</p>
        {code(authValues, "yaml")}
        <p>{page.accessProbe}</p>
      </section>
      <section id="screens">
        <h2>{page.headings.screens}</h2>
        <ul>
          {page.screens.map((screen) => (
            <li key={screen.name}>
              <strong>{screen.name}</strong> — {screen.body}
            </li>
          ))}
        </ul>
      </section>
      <section id="data-sources">
        <h2>{page.headings.dataSources}</h2>
        <p>{page.dataSourcesIntro}</p>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{page.dataSourceHeaders.category}</th>
                <th>{page.dataSourceHeaders.type}</th>
                <th>{page.dataSourceHeaders.worksWith}</th>
              </tr>
            </thead>
            <tbody>
              {page.dataSources.map((row) => (
                <tr key={row.type}>
                  <td>{row.category}</td>
                  <td>{row.type}</td>
                  <td>{row.worksWith}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p>{page.dataSourcesOutside}</p>
      </section>
    </article>
  );
}
