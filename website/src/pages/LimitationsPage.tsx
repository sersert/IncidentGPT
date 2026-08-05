import { Callout } from "../components/Callout";
import type { Translation } from "../i18n/types";

type PageProps = { t: Translation };

export function LimitationsPage({ t }: PageProps) {
  return (
    <article className="doc-page">
      <section id="correlation-limits">
        <h2>Correlation</h2>
        <p><strong>{t.common.currentBehavior}:</strong> same namespace + time window. Redis key is grp:namespace and field is fingerprint.</p>
        <p><strong>{t.common.potentialRisk}:</strong> independent incidents in one namespace can be grouped; cross-namespace cascades can be missed.</p>
        <p><strong>{t.common.recommended}:</strong> add topology, service catalog data, deployment correlation, traces or causal graph support when the project needs stronger grouping.</p>
        <ul>
          <li>No service dependency graph.</li>
          <li>No semantic similarity.</li>
          <li>No alert inhibition awareness.</li>
          <li>No historical similarity search.</li>
          <li>No ownership model beyond labels.</li>
        </ul>
      </section>
      <section id="llm">
        <h2>LLM</h2>
        <ul>
          <li>The model can be wrong or hallucinate.</li>
          <li>Output depends on context quality.</li>
          <li>Commands must be reviewed before execution.</li>
          <li>AI analysis must not be the only source for incident decisions.</li>
        </ul>
        <Callout title="Human decision" tone="warning">
          <p>{t.home.humanNote}</p>
        </Callout>
      </section>
      <section id="security">
        <h2>Security</h2>
        <Callout title="Sensitive data" tone="warning">
          <p>Do not send passwords, tokens, personal data, Kubernetes Secrets, full configuration dumps or other sensitive information to the LLM.</p>
        </Callout>
      </section>
      <section id="scaling">
        <h2>Scaling</h2>
        <div className="limitation-list">
          {[
            ["Local timers", "groupTimers is process-local; multiple Enricher replicas can schedule independent flushes for the same Redis key.", "Use a distributed lock, queue or stream consumer."],
            ["Redis group competition", "HSet deduplicates by fingerprint, but flush reads and deletes the whole key.", "Make flush idempotent and observable."],
            ["Webhook redelivery", "Fingerprint dedupe helps, but there is no full delivery idempotency contract.", "Track processed alert/group IDs."],
            ["Rate limits", "Telegram and LLM calls can be rate-limited under large storms.", "Add queueing, backoff and rate controls."],
            ["Payload size", "/incident-group reads up to 8 MiB; high-cardinality metrics can produce large prompts.", "Cap metric series and summarize before sending."],
            ["Cost", "One LLM call per group can still be expensive during noisy incidents.", "Budget model choice and max_tokens per environment."],
          ].map(([title, behavior, recommendation]) => (
            <article key={title}>
              <h3>{title}</h3>
              <p><strong>{t.common.currentBehavior}:</strong> {behavior}</p>
              <p><strong>{t.common.recommended}:</strong> {recommendation}</p>
            </article>
          ))}
        </div>
      </section>
    </article>
  );
}
