import { ConfigGenerator } from "../components/ConfigGenerator";
import { CodeBlock } from "../components/CodeBlock";
import type { Translation } from "../i18n/types";
import { aiWorkerEnv, enricherEnv, prometheusExamples } from "../data/configuration";

type PageProps = { t: Translation };

function EnvTable({ rows }: { rows: typeof enricherEnv }) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Variable</th>
            <th>Default</th>
            <th>Example</th>
            <th>Purpose</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.name}>
              <td><code>{row.name}</code></td>
              <td><code>{row.defaultValue}</code></td>
              <td><code>{row.example}</code></td>
              <td>{row.description}{row.note ? <p className="muted">{row.note}</p> : null}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function ConfigurationPage({ t }: PageProps) {
  return (
    <article className="doc-page">
      <section id="enricher-env">
        <h2>Enricher environment</h2>
        <EnvTable rows={enricherEnv} />
      </section>
      <section id="ai-worker-env">
        <h2>AI Worker environment</h2>
        <EnvTable rows={aiWorkerEnv} />
      </section>
      <section id="promql">
        <h2>Prometheus metrics configuration</h2>
        <p>Metric templates use Go template placeholders such as {"{{ .Namespace }}"}, {"{{ .Cluster }}"}, {"{{ .Service }}"}, {"{{ .Node }}"} and {"{{ .Instance }}"}.</p>
        <div className="metric-grid">
          {prometheusExamples.map((metric) => (
            <article className="metric-card" key={metric.name}>
              <h3>{metric.name}</h3>
              <p>{metric.description}</p>
              <CodeBlock code={metric.promql} language="promql" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
              <dl>
                <div><dt>Unit</dt><dd>{metric.unit}</dd></div>
                <div><dt>Trend</dt><dd>{metric.trend}</dd></div>
                <div><dt>Series</dt><dd>{metric.series}</dd></div>
                <div><dt>Cardinality</dt><dd>{metric.risk}</dd></div>
              </dl>
            </article>
          ))}
        </div>
      </section>
      <section id="runbooks">
        <h2>Runbooks</h2>
        <p>RUNBOOK_BASE_URL is trimmed on the right and the lowercase alertname is appended. KubePodCrashLooping becomes https://runbooks.example.com/alerts/kubepodcrashlooping.</p>
        <CodeBlock code={"# KubePodCrashLooping\n\n## Impact\n\n## Immediate checks\n\n## Common causes\n\n## Commands\n\n## Escalation\n\n## Recovery verification"} language="markdown" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <ConfigGenerator t={t} />
    </article>
  );
}
