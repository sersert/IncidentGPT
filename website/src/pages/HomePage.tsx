import { BellRing, Bot, Boxes, GitBranch, MessageSquareText, ShieldCheck } from "lucide-react";
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
[ALERT_ENRICHED] metrics=4 pods=3 events=2
[ALERT_BUFFERED] group=grp:monitoring fingerprint=7c4e...
[GROUP_SENT] alerts=3 destination=ai-worker`;

const incidentDemo = `Incident #42

Likely root cause:
CPU saturation on synthetic-pod-1 caused request latency
and triggered dependent availability alerts.

Check first:
1. Pod CPU throttling
2. Recent deployment
3. Node pressure
4. Upstream latency`;

const featureText = [
  {
    icon: BellRing,
    title: "Alert enrichment",
    body: "Prometheus range queries, Kubernetes objects, pod and node status, events and optional runbook links.",
  },
  {
    icon: GitBranch,
    title: "Incident correlation",
    body: "Группировка каскада алертов одного namespace в ограниченном временном окне.",
  },
  {
    icon: Bot,
    title: "AI-assisted analysis",
    body: "LLM получает не только название алерта, но и технический контекст, необходимый для более предметного разбора.",
  },
  {
    icon: MessageSquareText,
    title: "Telegram workflow",
    body: "Сначала публикуются исходные алерты, затем сводка и AI-разбор.",
  },
  {
    icon: ShieldCheck,
    title: "Graceful degradation",
    body: "При недоступности Redis алерты продолжают обрабатываться отдельно. Недоступность LLM не скрывает исходный алерт.",
  },
  {
    icon: Boxes,
    title: "Kubernetes-native delivery",
    body: "Оба сервиса поставляются с Dockerfile и Helm-чартами.",
  },
];

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
          <p>{language === "ru" ? "Контекст, корреляция и доставка в привычный on-call канал." : "Context, correlation and delivery into the on-call channel engineers already use."}</p>
        </div>
        <div className="feature-grid">
          {featureText.map((feature) => (
            <FeatureCard key={feature.title} icon={feature.icon} title={feature.title}>
              {language === "en"
                ? feature.body
                    .replace("Группировка каскада алертов одного namespace в ограниченном временном окне.", "Groups alert cascades from the same namespace inside a bounded time window.")
                    .replace("LLM получает не только название алерта, но и технический контекст, необходимый для более предметного разбора.", "The LLM receives labels, annotations, metrics and Kubernetes context, not just an alert name.")
                    .replace("Сначала публикуются исходные алерты, затем сводка и AI-разбор.", "Raw alerts are published first, followed by a grouped summary and AI-assisted analysis.")
                    .replace("При недоступности Redis алерты продолжают обрабатываться отдельно. Недоступность LLM не скрывает исходный алерт.", "If Redis is unavailable, alerts fall back to single-alert delivery. LLM failure does not hide the raw alert.")
                    .replace("Оба сервиса поставляются с Dockerfile и Helm-чартами.", "Both services ship with Dockerfiles and Helm charts.")
                : feature.body}
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
              <li>Alert name</li>
              <li>Summary</li>
              <li>Severity</li>
            </ul>
          </article>
          <article>
            <h3>{t.home.incidentgptApproach}</h3>
            <ul className="context-list">
              {[
                "Alert labels",
                "Annotations",
                "Prometheus metrics",
                "Metric trends",
                "Pod state",
                "Container status",
                "Node state",
                "Kubernetes Events",
                "Related alerts",
                "Runbook URL",
                "Cluster name",
              ].map((item) => (
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
          {[
            "git clone https://github.com/sersert/IncidentGPT.git\ncd IncidentGPT",
            "helm upgrade --install redis bitnami/redis \\\n  -n incidentgpt \\\n  --create-namespace \\\n  --set architecture=standalone \\\n  --set auth.enabled=false \\\n  --set master.persistence.enabled=false",
            "helm upgrade --install ai-worker ./ai-worker/chart -n incidentgpt",
            "helm upgrade --install incidentgpt-enricher ./enricher/chart/incidentgpt-enricher -n incidentgpt",
          ].map((code, index) => (
            <CodeBlock key={index} code={code} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
          ))}
        </div>
        <p className="muted">
          {language === "ru"
            ? "Перед установкой заполните Helm values и создайте Kubernetes Secrets."
            : "Before deploying, fill Helm values and create Kubernetes Secrets."}{" "}
          <a href={buildRoute(language, "installation")}>{t.hero.installButton}</a>.
        </p>
      </section>
    </>
  );
}
