import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";
import { CodeBlock } from "../components/CodeBlock";
import { Hero } from "../components/Hero";

type HomePageProps = {
  t: Translation;
  language: Language;
};

const quickStartCommands = [
  "git clone https://github.com/sersert/IncidentGPT.git\ncd IncidentGPT",
  "kubectl create namespace incidentgpt\n\nkubectl create secret generic incidentgpt-ai-worker \\\n  -n incidentgpt \\\n  --from-literal=OPENROUTER_API_KEY='replace-me' \\\n  --from-literal=TELEGRAM_BOT_TOKEN='replace-me'",
  "helm dependency build ./deploy/incidentgpt\n\nhelm upgrade --install incidentgpt ./deploy/incidentgpt \\\n  -n incidentgpt \\\n  --values values-incidentgpt.yaml",
  "kubectl get deploy,pod,svc -n incidentgpt",
];

export function HomePage({ t, language }: HomePageProps) {
  return (
    <>
      <Hero t={t} language={language} />
      <section className="home-docs">
        <h2>{language === "ru" ? "Документация" : "Documentation"}</h2>
        <nav aria-label={language === "ru" ? "С чего начать" : "Get started"}>
          {(["installation", "configuration", "webUi"] as const).map((page) => (
            <a
              key={page}
              href={buildRoute(language, page === "webUi" ? "web-ui" : page)}
            >
              <strong>{t.navigation[page]}</strong>
              <span>{t.pages[page].description}</span>
            </a>
          ))}
        </nav>
      </section>
      <section id="features">
        <div className="section-heading">
          <h2>{t.home.featuresTitle}</h2>
          <p>{t.home.featuresSubtitle}</p>
        </div>
        <div className="feature-grid">
          {t.home.features.map((feature) => (
            <article key={feature.title}>
              <h3>{feature.title}</h3>
              <p>{feature.body}</p>
            </article>
          ))}
        </div>
      </section>
      <section id="quick-start">
        <div className="section-heading">
          <h2>{t.home.quickStartTitle}</h2>
        </div>
        <div className="quick-grid">
          {quickStartCommands.map((code, index) => (
            <CodeBlock
              key={index}
              code={code}
              language="bash"
              copyLabel={t.common.copy}
              copiedLabel={t.common.copied}
            />
          ))}
        </div>
        <p className="muted">
          {t.home.quickStartNote}{" "}
          <a href={buildRoute(language, "installation")}>
            {t.hero.installButton}
          </a>
          .
        </p>
      </section>
    </>
  );
}
