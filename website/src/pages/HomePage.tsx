import { BellRing, Bot, GitBranch, LayoutDashboard, ScrollText, ShieldCheck } from "lucide-react";
import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";
import { CodeBlock } from "../components/CodeBlock";
import { FeatureCard } from "../components/FeatureCard";
import { Hero } from "../components/Hero";
import { Callout } from "../components/Callout";

type HomePageProps = {
  t: Translation;
  language: Language;
};

const terminalDemo = `$ kubectl logs -n incidentgpt deploy/incidentgpt-enricher

[ALERT_RAW] alert=SyntheticHighCPU namespace=monitoring
[ALERT_ENRICHED] metrics=4 pods=3 events=2 logs=12
[ALERT_BUFFERED] group=grp:monitoring fingerprint=7c4e...
[GROUP_SENT] alerts=3 destination=backend`;

const incidentDemo = `Инцидент #42 — 3 алерта

**Корень:** насыщение CPU на synthetic-pod-1 подняло задержки
и потянуло за собой алерты доступности.

**Из логов:**
[ERROR ×156] catalogue-674f789b65: circuit breaker 'List' is open

**Исправление:**
1. Поднять лимиты CPU для synthetic-pod-1
2. Откатить последний выкат
3. Добавить реплику, пока не спадёт нагрузка

**Уверенность:** 72`;

const quickStartCommands = [
  "git clone https://github.com/sersert/IncidentGPT.git\ncd IncidentGPT",
  "kubectl create namespace incidentgpt\n\nkubectl create secret generic incidentgpt-ai-worker \\\n  -n incidentgpt \\\n  --from-literal=OPENROUTER_API_KEY='replace-me' \\\n  --from-literal=TELEGRAM_BOT_TOKEN='replace-me'",
  "helm dependency build ./deploy/incidentgpt\n\nhelm upgrade --install incidentgpt ./deploy/incidentgpt \\\n  -n incidentgpt \\\n  --values values-incidentgpt.yaml",
  "kubectl get deploy,pod,svc -n incidentgpt",
];

const featureIcons = [BellRing, ScrollText, GitBranch, Bot, LayoutDashboard, ShieldCheck];

export function HomePage({ t, language }: HomePageProps) {
  return (
    <>
      <Hero t={t} language={language} />
      <Callout title={t.home.humanNote} tone="warning">
        <p>{t.home.humanNote}</p>
      </Callout>
      <section className="two-column">
        <div>
          <h2>{t.home.terminalTitle}</h2>
          <CodeBlock code={terminalDemo} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        </div>
        <div>
          <h2>{t.home.resultTitle}</h2>
          <CodeBlock code={incidentDemo} language="text" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
          <p className="muted">{t.hero.demoNote}</p>
        </div>
      </section>
      <section id="features">
        <div className="section-heading">
          <h2>{t.home.featuresTitle}</h2>
          <p>{t.home.featuresSubtitle}</p>
        </div>
        <div className="feature-grid">
          {t.home.features.map((feature, index) => (
            <FeatureCard key={feature.title} icon={featureIcons[index]} title={feature.title}>
              {feature.body}
            </FeatureCard>
          ))}
        </div>
      </section>
      <section className="comparison">
        <div className="section-heading">
          <h2>{t.home.compareTitle}</h2>
          <p>{t.home.compareConclusion}</p>
        </div>
        <div className="compare-grid">
          <article>
            <h3>{t.home.regularApproach}</h3>
            <ul>
              {t.home.basicPromptItems.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </article>
          <article>
            <h3>{t.home.incidentgptApproach}</h3>
            <ul className="context-list">
              {t.home.contextItems.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </article>
        </div>
      </section>
      <section className="reliability-band">
        <h2>{t.home.reliabilityTitle}</h2>
        <p>{t.home.reliabilityBody}</p>
      </section>
      <section id="quick-start">
        <div className="section-heading">
          <h2>{t.home.quickStartTitle}</h2>
        </div>
        <div className="quick-grid">
          {quickStartCommands.map((code, index) => (
            <CodeBlock key={index} code={code} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
          ))}
        </div>
        <p className="muted">
          {t.home.quickStartNote} <a href={buildRoute(language, "installation")}>{t.hero.installButton}</a>.
        </p>
      </section>
    </>
  );
}
