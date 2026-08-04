import type { Translation } from "../i18n/types";
import { Callout } from "../components/Callout";
import { CodeBlock } from "../components/CodeBlock";
import { alertmanagerSnippet } from "../data/examples";

type PageProps = { t: Translation };

export function InstallationPage({ t }: PageProps) {
  const checks = ["Kubernetes cluster", "kubectl", "Helm 3", "kube-prometheus-stack", "Redis", "Container registry", "Telegram bot", "Telegram channel", "Telegram discussion group", "OpenRouter or OpenAI-compatible API key"];
  return (
    <article className="doc-page">
      <section id="prerequisites">
        <h2>Step 0. Prerequisites</h2>
        <ul className="check-list">{checks.map((item) => <li key={item}>{item}</li>)}</ul>
        <CodeBlock code={"kubectl version --client\nhelm version\nkubectl get nodes\nkubectl get pods -n monitoring"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="telegram">
        <h2>Step 1. Telegram</h2>
        <ol className="steps">
          <li>Create a bot via @BotFather and store the token as TELEGRAM_BOT_TOKEN.</li>
          <li>Disable privacy mode if the bot must see discussion messages.</li>
          <li>Create a channel and a linked discussion group.</li>
          <li>Add the bot as an administrator in both places.</li>
          <li>Send a test message and inspect getUpdates for channel and group IDs.</li>
        </ol>
        <CodeBlock code={"https://api.telegram.org/bot<TELEGRAM_BOT_TOKEN>/getUpdates"} language="text" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <CodeBlock code={'{\n  "message": {\n    "chat": {\n      "id": -1001234567890,\n      "type": "channel",\n      "title": "IncidentGPT"\n    }\n  }\n}'} language="json" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <Callout title="Secrets" tone="warning">
          <p>Never publish a real TELEGRAM_BOT_TOKEN in Git, Helm values or documentation.</p>
        </Callout>
      </section>
      <section id="openrouter">
        <h2>Step 2. OpenRouter</h2>
        <CodeBlock code={'OPENROUTER_API_KEY: ""\nOPENROUTER_MODEL: "google/gemini-2.5-flash"\nOPENROUTER_MAX_TOKENS: "2000"'} language="yaml" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <p>Any provider must expose an OpenAI-compatible chat completions endpoint because the code posts model, max_tokens and messages to OPENROUTER_BASE_URL.</p>
      </section>
      <section id="redis">
        <h2>Step 3. Redis</h2>
        <CodeBlock code={"helm repo add bitnami https://charts.bitnami.com/bitnami\nhelm repo update\n\nhelm upgrade --install redis bitnami/redis \\\n  --namespace incidentgpt \\\n  --create-namespace \\\n  --set architecture=standalone \\\n  --set auth.enabled=false \\\n  --set master.persistence.enabled=false"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <CodeBlock code={"kubectl get pods -n incidentgpt\nkubectl get svc -n incidentgpt"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <Callout title="Demo only" tone="warning">
          <p>Redis without authentication and persistence is for demonstrations. Production should use auth, persistence, NetworkPolicy, backups and controlled upgrades.</p>
        </Callout>
      </section>
      <section id="images">
        <h2>Step 4. Build images</h2>
        <CodeBlock code={"export REGISTRY=ghcr.io/your-user\nexport VERSION=0.1.0\n\ndocker buildx build \\\n  --platform linux/amd64 \\\n  -t ${REGISTRY}/incidentgpt-ai-worker:${VERSION} \\\n  ./ai-worker \\\n  --push\n\ndocker buildx build \\\n  --platform linux/amd64 \\\n  -t ${REGISTRY}/incidentgpt-enricher:${VERSION} \\\n  ./enricher \\\n  --push"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <p className="muted">Multi-arch builds can be attempted with linux/amd64,linux/arm64, but test the resulting images in your cluster.</p>
      </section>
      <section id="secrets">
        <h2>Step 5. Secrets</h2>
        <CodeBlock code={"kubectl create namespace incidentgpt\n\nkubectl create secret generic incidentgpt-ai-worker \\\n  -n incidentgpt \\\n  --from-literal=OPENROUTER_API_KEY='replace-me' \\\n  --from-literal=TELEGRAM_BOT_TOKEN='replace-me'"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="helm">
        <h2>Step 6-7. Deploy services</h2>
        <CodeBlock code={"helm upgrade --install ai-worker \\\n  ./ai-worker/chart \\\n  --namespace incidentgpt \\\n  --values values-ai-worker.yaml\n\nhelm upgrade --install incidentgpt-enricher \\\n  ./enricher/chart/incidentgpt-enricher \\\n  --namespace incidentgpt \\\n  --values values-enricher.yaml"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <CodeBlock code={"kubectl get deploy,pod,svc -n incidentgpt\nkubectl logs -n incidentgpt deploy/ai-worker\nkubectl logs -n incidentgpt deploy/incidentgpt-enricher"} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
      </section>
      <section id="alertmanager">
        <h2>Step 8. Alertmanager</h2>
        <CodeBlock code={alertmanagerSnippet} language="yaml" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        <p>route.receiver and receivers[].name must match exactly. continue: true keeps existing Slack, PagerDuty or other receivers active.</p>
      </section>
    </article>
  );
}
