import type { Translation } from "../i18n/types";
import { Callout } from "../components/Callout";
import { CodeBlock } from "../components/CodeBlock";
import { alertmanagerSnippet } from "../data/examples";

type PageProps = { t: Translation };

export function InstallationPage({ t }: PageProps) {
  const page = t.installation;
  const code = (value: string, language: string) => (
    <CodeBlock code={value} language={language} copyLabel={t.common.copy} copiedLabel={t.common.copied} />
  );
  return (
    <article className="doc-page">
      <section id="prerequisites">
        <h2>{page.headings.prerequisites}</h2>
        <ul className="check-list">
          {page.prerequisites.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
        {code("kubectl version --client\nhelm version\nkubectl get nodes\nkubectl get pods -n monitoring", "bash")}
        <Callout title={page.redisCallout.title}>
          <p>{page.redisCallout.body}</p>
        </Callout>
      </section>
      <section id="telegram">
        <h2>{page.headings.telegram}</h2>
        <ol className="steps">
          {page.telegramSteps.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ol>
        {code("https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/getUpdates", "text")}
        {code('{\n  "message": {\n    "chat": {\n      "id": -1001234567890,\n      "type": "channel",\n      "title": "IncidentGPT"\n    }\n  }\n}', "json")}
        <p className="muted">{page.telegramPrivacyNote}</p>
        <Callout title={page.telegramSecretsCallout.title} tone="warning">
          <p>{page.telegramSecretsCallout.body}</p>
        </Callout>
      </section>
      <section id="openrouter">
        <h2>{page.headings.openrouter}</h2>
        {code('OPENROUTER_API_KEY: ""\nOPENROUTER_MODEL: "google/gemini-2.5-flash"\nOPENROUTER_MAX_TOKENS: "2000"', "yaml")}
        <p>{page.openrouterNote}</p>
      </section>
      <section id="secrets">
        <h2>{page.headings.secrets}</h2>
        <p>{page.secretsIntro}</p>
        {code(
          "kubectl create namespace incidentgpt\n\nkubectl create secret generic incidentgpt-ai-worker \\\n  -n incidentgpt \\\n  --from-literal=OPENROUTER_API_KEY='replace-me' \\\n  --from-literal=TELEGRAM_BOT_TOKEN='replace-me'\n\nkubectl create secret generic incidentgpt-sanitizer \\\n  -n incidentgpt \\\n  --from-literal=auth-shared-secret=\"$(openssl rand -hex 32)\" \\\n  --from-literal=hash-key=\"$(openssl rand -hex 32)\"",
          "bash",
        )}
        <p className="muted">{page.secretsNote}</p>
      </section>
      <section id="helm">
        <h2>{page.headings.helm}</h2>
        <p>{page.helmIntro}</p>
        {code(
          "helm dependency build ./deploy/incidentgpt\n\nhelm upgrade --install incidentgpt \\\n  ./deploy/incidentgpt \\\n  --namespace incidentgpt \\\n  --values values-incidentgpt.yaml",
          "bash",
        )}
        <p>{page.helmDisableNote}</p>
        {code(
          "kubectl get deploy,pod,svc -n incidentgpt\nkubectl logs -n incidentgpt deploy/incidentgpt-enricher\nkubectl logs -n incidentgpt deploy/ai-worker\nkubectl logs -n incidentgpt deploy/incidentgpt-backend",
          "bash",
        )}
      </section>
      <section id="images">
        <h2>{page.headings.images}</h2>
        <p>{page.imagesIntro}</p>
        {code(
          "ghcr.io/sersert/incidentgpt-enricher:0.2.0\nghcr.io/sersert/incidentgpt-ai-worker:0.2.0\nghcr.io/sersert/incidentgpt-sanitizer:0.2.0\nghcr.io/sersert/incidentgpt-backend:0.2.0\nghcr.io/sersert/incidentgpt-ui:0.2.0",
          "text",
        )}
        <p>{page.imagesBuildNote}</p>
        {code(
          "export REGISTRY=ghcr.io/your-user\nexport VERSION=0.2.0\n\ndocker buildx build \\\n  --platform linux/amd64 \\\n  -t ${REGISTRY}/incidentgpt-ai-worker:${VERSION} \\\n  ./ai-worker \\\n  --push\n\ndocker buildx build \\\n  --platform linux/amd64 \\\n  -t ${REGISTRY}/incidentgpt-enricher:${VERSION} \\\n  ./enricher \\\n  --push",
          "bash",
        )}
        <p className="muted">{page.imagesArchNote}</p>
      </section>
      <section id="alertmanager">
        <h2>{page.headings.alertmanager}</h2>
        {code(alertmanagerSnippet, "yaml")}
        <p>{page.alertmanagerNote}</p>
      </section>
      <section id="verify">
        <h2>{page.headings.verify}</h2>
        <p>{page.verifyIntro}</p>
        {code(
          "incidentgpt-ui:\n  ingress:\n    enabled: true\n    className: nginx\n    host: incidentgpt.example.com\n    tls:\n      secretName: incidentgpt-ui-tls\n  auth:\n    enabled: true\n    username: admin\n    existingSecret: incidentgpt-ui-auth",
          "yaml",
        )}
        {code("kubectl port-forward -n incidentgpt svc/incidentgpt-ui 8080:80", "bash")}
      </section>
    </article>
  );
}
