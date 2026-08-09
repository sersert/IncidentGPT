import { Callout } from "../components/Callout";
import { CodeBlock } from "../components/CodeBlock";
import type { Translation } from "../i18n/types";

type PageProps = { t: Translation };

const enricherValues = `incidentgpt-enricher:
  env:
    logsStoreUrl: "http://opensearch.logging.svc:9200"
    logsStoreUser: ""
    logsStoreIndex: "logs-*"
    logsRangeBefore: "10m"
    logsRangeAfter: "2m"
    logsMaxLines: "40"
  logsStore:
    existingSecret: ""
    passwordKey: password

incidentgpt-backend:
  env:
    LOGS_STORE_URL: "http://opensearch.logging.svc:9200"
    LOGS_STORE_INDEX: "logs-*"`;

const fluentBit = `[INPUT]
    Name tail
    Tag kube.myapp.*
    Path /var/log/containers/*_myapp_*.log
    multiline.parser docker, cri
    Mem_Buf_Limit 32MB
    Skip_Long_Lines On
    Inotify_Watcher Off

[FILTER]
    Name kubernetes
    Match kube.*
    Kube_Tag_Prefix kube.myapp.var.log.containers.
    Merge_Log On
    Keep_Log Off

[OUTPUT]
    Name opensearch
    Match kube.*
    Host opensearch.logging.svc
    Port 9200
    Logstash_Format On
    Logstash_Prefix logs-myapp
    Time_Key @timestamp
    Replace_Dots On
    Suppress_Type_Name On`;

const excerpt = `[ERROR ×156] catalogue-674f789b65: circuit breaker 'List' is open
[ERROR ×16]  catalogue-674f789b65: database connection error
[WARN ×3]    session-db-7979d74bb: Read-only file system`;

export function LogsPage({ t }: PageProps) {
  const page = t.logs;
  const code = (value: string, language: string) => (
    <CodeBlock code={value} language={language} copyLabel={t.common.copy} copiedLabel={t.common.copied} />
  );
  return (
    <article className="doc-page">
      <section id="why">
        <h2>{page.headings.why}</h2>
        {page.whyParagraphs.map((paragraph) => (
          <p key={paragraph}>{paragraph}</p>
        ))}
      </section>
      <section id="requirements">
        <h2>{page.headings.requirements}</h2>
        <p>{page.requirementsIntro}</p>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>{page.fieldHeaders.field}</th>
                <th>{page.fieldHeaders.content}</th>
              </tr>
            </thead>
            <tbody>
              {page.fields.map((row) => (
                <tr key={row.field}>
                  <td><code>{row.field}</code></td>
                  <td>{row.content}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p>{page.requirementsSource}</p>
        <Callout title={page.mappingCallout.title} tone="warning">
          <p>{page.mappingCallout.body}</p>
        </Callout>
      </section>
      <section id="wiring">
        <h2>{page.headings.wiring}</h2>
        {code(enricherValues, "yaml")}
        <p>{page.wiringNote}</p>
        <Callout title={page.baseUrlCallout.title}>
          <p>{page.baseUrlCallout.body}</p>
        </Callout>
      </section>
      <section id="collector">
        <h2>{page.headings.collector}</h2>
        <p>{page.collectorIntro}</p>
        {code(fluentBit, "ini")}
        <p>{page.collectorNote}</p>
      </section>
      <section id="prompt">
        <h2>{page.headings.prompt}</h2>
        <p>{page.promptIntro}</p>
        <ul>
          {page.promptRules.map((rule) => (
            <li key={rule}>{rule}</li>
          ))}
        </ul>
        <p>{page.promptResult}</p>
        {code(excerpt, "text")}
        <p>{page.promptCounter}</p>
      </section>
    </article>
  );
}
