import { Callout } from "../components/Callout";
import type { Translation } from "../i18n/types";

type PageProps = { t: Translation };

export function LimitationsPage({ t }: PageProps) {
  const page = t.limitations;
  return (
    <article className="doc-page">
      <section id="correlation-limits">
        <h2>{page.headings.correlation}</h2>
        <p><strong>{t.common.currentBehavior}:</strong> {page.correlation.behavior}</p>
        <p><strong>{t.common.potentialRisk}:</strong> {page.correlation.risk}</p>
        <p><strong>{t.common.recommended}:</strong> {page.correlation.recommended}</p>
        <ul>
          {page.correlation.items.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>
      <section id="llm">
        <h2>{page.headings.llm}</h2>
        <ul>
          {page.llmItems.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
        <Callout title={page.llmCallout.title} tone="warning">
          <p>{t.home.humanNote}</p>
        </Callout>
      </section>
      <section id="storage">
        <h2>{page.headings.storage}</h2>
        <p><strong>{t.common.currentBehavior}:</strong> {page.storage.behavior}</p>
        <p><strong>{t.common.potentialRisk}:</strong> {page.storage.risk}</p>
        <p><strong>{t.common.recommended}:</strong> {page.storage.recommended}</p>
      </section>
      <section id="security">
        <h2>{page.headings.security}</h2>
        <Callout title={page.securityCallout.title} tone="warning">
          <p>{page.securityCallout.body}</p>
        </Callout>
        <p><strong>{t.common.currentBehavior}:</strong> {page.security.behavior}</p>
        <p><strong>{t.common.potentialRisk}:</strong> {page.security.risk}</p>
        <p><strong>{t.common.recommended}:</strong> {page.security.recommended}</p>
      </section>
      <section id="scaling">
        <h2>{page.headings.scaling}</h2>
        <div className="limitation-list">
          {page.scaling.map((item) => (
            <article key={item.title}>
              <h3>{item.title}</h3>
              <p><strong>{t.common.currentBehavior}:</strong> {item.behavior}</p>
              <p><strong>{t.common.recommended}:</strong> {item.recommended}</p>
            </article>
          ))}
        </div>
      </section>
    </article>
  );
}
