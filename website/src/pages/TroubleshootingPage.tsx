import { CodeBlock } from "../components/CodeBlock";
import type { Translation } from "../i18n/types";

type PageProps = { t: Translation };

// Порядок команд совпадает с порядком карточек в словарях i18n.
const commands = [
  "kubectl get svc -n incidentgpt\nkubectl get endpoints -n incidentgpt\nkubectl logs -n monitoring alertmanager-<pod>\nkubectl logs -n incidentgpt deploy/incidentgpt-enricher",
  "kubectl get prometheusrules --all-namespaces\nkubectl get prometheuses --all-namespaces\nkubectl describe prometheusrule <name> -n <namespace>",
  "kubectl run network-debug \\\n  --rm -it \\\n  --restart=Never \\\n  --image=curlimages/curl \\\n  -n incidentgpt \\\n  -- curl -v http://<prometheus-service>:9090/-/ready",
  "kubectl auth can-i list pods \\\n  --as=system:serviceaccount:incidentgpt:<service-account> \\\n  --all-namespaces\n\nkubectl auth can-i list events \\\n  --as=system:serviceaccount:incidentgpt:<service-account> \\\n  --all-namespaces",
  "kubectl get svc,endpoints,pod -n incidentgpt | grep redis\n\nkubectl run redis-debug \\\n  --rm -it \\\n  --restart=Never \\\n  --image=redis:7-alpine \\\n  -n incidentgpt \\\n  -- redis-cli -h incidentgpt-redis ping",
  "kubectl logs -n incidentgpt deploy/incidentgpt-enricher | grep -E 'ALERT_BUFFERED|GROUP_SENT|redis'",
  "kubectl logs -n incidentgpt deploy/ai-worker",
  "kubectl get svc,endpoints -n incidentgpt | grep backend\nkubectl logs -n incidentgpt deploy/incidentgpt-ui\nkubectl logs -n incidentgpt deploy/incidentgpt-backend",
  "kubectl exec -n incidentgpt deploy/ai-worker -- \\\n  sh -c 'echo \"$ANALYSIS_CALLBACK_URL\"'\n\nkubectl logs -n incidentgpt deploy/incidentgpt-backend | grep -Ei 'ingest|analysis|forward'",
  "kubectl exec -n incidentgpt deploy/incidentgpt-enricher -- \\\n  sh -c 'echo \"$LOGS_STORE_URL $LOGS_STORE_INDEX\"'\n\nkubectl logs -n incidentgpt deploy/incidentgpt-enricher | grep -i logs",
  "kubectl exec -n incidentgpt deploy/ai-worker -- \\\n  sh -c 'test -n \"$OPENROUTER_API_KEY\" && echo configured || echo missing'",
];

export function TroubleshootingPage({ t }: PageProps) {
  return (
    <article className="doc-page">
      <div className="troubleshooting-grid">
        {t.troubleshooting.cards.map((card, index) => (
          <section className="problem-card" id={`problem-${index + 1}`} key={card.title}>
            <h2>{card.title}</h2>
            <ul>{card.checks.map((check) => <li key={check}>{check}</li>)}</ul>
            <CodeBlock code={commands[index]} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
          </section>
        ))}
      </div>
    </article>
  );
}
