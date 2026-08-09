import { CodeBlock } from "../components/CodeBlock";
import { Callout } from "../components/Callout";
import type { Translation } from "../i18n/types";
import { cascadeScript, directAlertPayload, telegramMock } from "../data/examples";

type PageProps = { t: Translation };

export function ExamplesPage({ t }: PageProps) {
  const page = t.examples;
  const code = (value: string, language: string) => (
    <CodeBlock code={value} language={language} copyLabel={t.common.copy} copiedLabel={t.common.copied} />
  );
  return (
    <article className="doc-page">
      <section id="direct-enricher">
        <h2>{page.headings.direct}</h2>
        {code("kubectl port-forward \\\n  -n incidentgpt \\\n  deployment/incidentgpt-enricher \\\n  9099:9099", "bash")}
        {code(
          `curl --fail-with-body \\
  --request POST \\
  --header "Content-Type: application/json" \\
  http://localhost:9099/alert \\
  --data '${directAlertPayload}'`,
          "bash",
        )}
      </section>
      <section id="cascade">
        <h2>{page.headings.cascade}</h2>
        <p>{page.cascadeNote}</p>
        {code(cascadeScript, "bash")}
      </section>
      <section id="prometheusrule">
        <h2>{page.headings.prometheusRule}</h2>
        {code(
          "kubectl apply -f ai-worker/cpu-alert.yaml\nkubectl get prometheusrules --all-namespaces\nkubectl describe prometheusrule resource-usage-alerts -n monitoring",
          "bash",
        )}
        <p>{page.prometheusRuleNote}</p>
      </section>
      <section id="production">
        <h2>{page.headings.production}</h2>
        <div className="tabs static-tabs">
          {page.environments.map((environment) => (
            <span key={environment}>{environment}</span>
          ))}
        </div>
        <ul>
          {page.productionItems.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>
      <section id="telegram-mock">
        <h2>{page.headings.telegramMock}</h2>
        <Callout title={page.mockCallout.title}>
          <p>{page.mockCallout.body}</p>
        </Callout>
        {code(telegramMock, "text")}
      </section>
    </article>
  );
}
