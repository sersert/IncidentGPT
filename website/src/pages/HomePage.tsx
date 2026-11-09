import {
  ArrowRight,
  BellRing,
  GitBranch,
  SearchCheck,
  BookOpen,
  FileText,
  Activity,
} from "lucide-react";
import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";
import { CodeBlock } from "../components/CodeBlock";
import { Hero } from "../components/Hero";

const icons = [BellRing, GitBranch, SearchCheck, BookOpen, FileText, Activity];
const commands = `git clone https://github.com/sersert/IncidentGPT.git
cd IncidentGPT
helm dependency build ./deploy/incidentgpt

# values-incidentgpt.yaml + Kubernetes Secrets
helm upgrade --install incidentgpt ./deploy/incidentgpt \\
  --namespace incidentgpt --create-namespace \\
  --values values-incidentgpt.yaml`;

export function HomePage({
  t,
  language,
}: {
  t: Translation;
  language: Language;
}) {
  return (
    <>
      <Hero t={t} language={language} />
      <section className="workflow-section" aria-labelledby="workflow-title">
        <div className="section-heading">
          <p className="eyebrow">{t.home.workflowLabel}</p>
          <h2 id="workflow-title">{t.home.workflowTitle}</h2>
        </div>
        <ol className="workflow-list">
          {t.home.workflow.map((step, i) => (
            <li key={step.title}>
              <span className="step-number">0{i + 1}</span>
              <div>
                <h3>{step.title}</h3>
                <p>{step.body}</p>
              </div>
            </li>
          ))}
        </ol>
        <a className="text-link" href={buildRoute(language, "architecture")}>
          {t.hero.architectureLink}
          <ArrowRight size={15} />
        </a>
      </section>
      <section id="features" className="capabilities-section">
        <div className="section-heading">
          <p className="eyebrow">{t.home.featuresLabel}</p>
          <h2>{t.home.featuresTitle}</h2>
          <p>{t.home.featuresSubtitle}</p>
        </div>
        <div className="capability-grid">
          {t.home.features.map((feature, i) => {
            const Icon = icons[i];
            return (
              <article key={feature.title}>
                <Icon size={21} strokeWidth={1.6} />
                <div>
                  <h3>{feature.title}</h3>
                  <p>{feature.body}</p>
                </div>
              </article>
            );
          })}
        </div>
        <p className="feature-note">{t.home.optInNote}</p>
      </section>
      <section className="reliability-band">
        <span className="status-dot" />
        <div>
          <h2>{t.home.reliabilityTitle}</h2>
          <p>{t.home.reliabilityBody}</p>
        </div>
        <a href={buildRoute(language, "limitations")} className="text-link">
          {t.navigation.limitations}
          <ArrowRight size={15} />
        </a>
      </section>
      <section id="quick-start" className="start-section">
        <div className="section-heading">
          <p className="eyebrow">{t.home.quickStartLabel}</p>
          <h2>{t.home.quickStartTitle}</h2>
          <p>{t.home.quickStartNote}</p>
          <a
            className="button primary"
            href={buildRoute(language, "installation")}
          >
            {t.hero.installButton}
            <ArrowRight size={16} />
          </a>
          <a className="text-link" href={buildRoute(language, "configuration")}>
            {t.generator.title}
            <ArrowRight size={15} />
          </a>
        </div>
        <CodeBlock
          code={commands}
          title="Helm / Kubernetes"
          language="bash"
          copyLabel={t.common.copy}
          copiedLabel={t.common.copied}
        />
      </section>
    </>
  );
}
