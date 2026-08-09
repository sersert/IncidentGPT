import { ConfigGenerator } from "../components/ConfigGenerator";
import { CodeBlock } from "../components/CodeBlock";
import { Callout } from "../components/Callout";
import type { EnvText, Translation } from "../i18n/types";
import { aiWorkerEnv, backendEnv, enricherEnv, prometheusExamples } from "../data/configuration";

type PageProps = { t: Translation };

type EnvRow = { name: string; defaultValue: string; example: string };

function EnvTable({ t, rows, text }: { t: Translation; rows: readonly EnvRow[]; text: Record<string, EnvText> }) {
  const headers = t.configuration.tableHeaders;
  return (
    <div className="table-wrap env-table">
      <table>
        <colgroup>
          <col style={{ width: "22%" }} />
          <col style={{ width: "24%" }} />
          <col style={{ width: "20%" }} />
          <col style={{ width: "34%" }} />
        </colgroup>
        <thead>
          <tr>
            <th>{headers.variable}</th>
            <th>{headers.default}</th>
            <th>{headers.example}</th>
            <th>{headers.purpose}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.name}>
              <td><code>{row.name}</code></td>
              <td><code>{row.defaultValue}</code></td>
              <td><code>{row.example}</code></td>
              <td>
                {text[row.name].description}
                {text[row.name].note ? <p className="muted">{text[row.name].note}</p> : null}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function ConfigurationPage({ t }: PageProps) {
  const page = t.configuration;
  return (
    <article className="doc-page">
      <section id="upgrade-0-2-0">
        <h2>{page.headings.upgrade}</h2>
        <Callout title={page.upgradeCallout.title} tone="warning">
          <p>{page.upgradeCallout.body}</p>
        </Callout>
      </section>
      <section id="enricher-env">
        <h2>{page.headings.enricherEnv}</h2>
        <EnvTable t={t} rows={enricherEnv} text={page.env.enricher} />
      </section>
      <section id="ai-worker-env">
        <h2>{page.headings.aiWorkerEnv}</h2>
        <EnvTable t={t} rows={aiWorkerEnv} text={page.env.aiWorker} />
      </section>
      <section id="backend-env">
        <h2>{page.headings.backendEnv}</h2>
        <p>{page.backendIntro}</p>
        <EnvTable t={t} rows={backendEnv} text={page.env.backend} />
      </section>
      <section id="promql">
        <h2>{page.headings.promql}</h2>
        <p>{page.promqlIntro}</p>
        <div className="metric-grid">
          {prometheusExamples.map((metric) => {
            const text = page.metrics[metric.name];
            return (
              <article className="metric-card" key={metric.name}>
                <h3>{metric.name}</h3>
                <p>{text.description}</p>
                <CodeBlock code={metric.promql} language="promql" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
                <dl>
                  <div><dt>{page.metricLabels.unit}</dt><dd>{text.unit}</dd></div>
                  <div><dt>{page.metricLabels.trend}</dt><dd>{text.trend}</dd></div>
                  <div><dt>{page.metricLabels.series}</dt><dd>{text.series}</dd></div>
                  <div><dt>{page.metricLabels.cardinality}</dt><dd>{text.risk}</dd></div>
                </dl>
              </article>
            );
          })}
        </div>
      </section>
      <section id="runbooks">
        <h2>{page.headings.runbooks}</h2>
        <p>{page.runbooksIntro}</p>
        <CodeBlock
          code={"# KubePodCrashLooping\n\n## Impact\n\n## Immediate checks\n\n## Common causes\n\n## Commands\n\n## Escalation\n\n## Recovery verification"}
          language="markdown"
          copyLabel={t.common.copy}
          copiedLabel={t.common.copied}
        />
      </section>
      <ConfigGenerator t={t} />
    </article>
  );
}
