import { CodeBlock } from "../components/CodeBlock";
import { Callout } from "../components/Callout";
import type { Translation } from "../i18n/types";
import { cascadeScript, directAlertPayload, telegramMock } from "../data/examples";

type PageProps = { t: Translation };

export function ExamplesPage({ t }: PageProps) {
  return (
    <article className="doc-page">
      <section id="direct-enricher">
        <h2>Direct Enricher test</h2>
        <CodeBlock code={"kubectl port-forward \\\n  -n incidentgpt \\\n  deployment/incidentgpt-enricher \\\n  9099:9099"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <CodeBlock code={`curl --fail-with-body \\
  --request POST \\
  --header "Content-Type: application/json" \\
  http://localhost:9099/alert \\
  --data '${directAlertPayload}'`} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="cascade">
        <h2>Cascade test</h2>
        <p>All alerts share the same namespace and are sent inside CORR_SETTLE, so they should be grouped into one Redis key.</p>
        <CodeBlock code={cascadeScript} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="prometheusrule">
        <h2>PrometheusRule test</h2>
        <CodeBlock code={"kubectl apply -f ai-worker/cpu-alert.yaml\nkubectl get prometheusrules --all-namespaces\nkubectl describe prometheusrule resource-usage-alerts -n monitoring"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <p>The existing sample uses release: kps. Adjust it to the label selected by your Prometheus.</p>
      </section>
      <section id="production">
        <h2>Production configuration</h2>
        <div className="tabs static-tabs">
          <span>Development</span>
          <span>Staging</span>
          <span>Production</span>
        </div>
        <ul>
          <li>Use non-latest image tags and pinned chart versions.</li>
          <li>Use Kubernetes Secrets for OpenRouter and Telegram tokens.</li>
          <li>Enable Redis authentication and persistence.</li>
          <li>Set resource requests and limits; probes already exist in both charts.</li>
          <li>Recommended improvement: NetworkPolicy, PodDisruptionBudget and Redis secretKeyRef support.</li>
          <li>Keep logs free of secrets; do not enable debug payload logging in production.</li>
        </ul>
      </section>
      <section id="telegram-mock">
        <h2>Telegram output mockup</h2>
        <Callout title="Mockup only">
          <p>This visual example does not call Telegram API and does not contain real chat IDs or tokens.</p>
        </Callout>
        <CodeBlock code={telegramMock} language="text" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
    </article>
  );
}
