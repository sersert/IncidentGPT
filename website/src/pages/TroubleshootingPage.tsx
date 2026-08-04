import { CodeBlock } from "../components/CodeBlock";
import type { Translation } from "../i18n/types";

type PageProps = { t: Translation };

const cards = [
  {
    title: "Alertmanager does not send webhook",
    checks: ["receiver name mismatch", "wrong service DNS or namespace", "port 9099 or path /alert", "NetworkPolicy or missing endpoints"],
    commands: "kubectl get svc -n incidentgpt\nkubectl get endpoints -n incidentgpt\nkubectl logs -n monitoring alertmanager-<pod>\nkubectl logs -n incidentgpt deploy/incidentgpt-enricher",
  },
  {
    title: "PrometheusRule does not fire",
    checks: ["wrong release label", "Prometheus rule selector does not match", "PromQL returns no series", "for duration too long", "wrong namespace"],
    commands: "kubectl get prometheusrules --all-namespaces\nkubectl get prometheuses --all-namespaces\nkubectl describe prometheusrule <name> -n <namespace>",
  },
  {
    title: "Enricher receives no Prometheus metrics",
    checks: ["Prometheus URL", "service endpoints", "NetworkPolicy", "PromQL syntax", "auth proxy in front of Prometheus"],
    commands: "kubectl run network-debug \\\n  --rm -it \\\n  --restart=Never \\\n  --image=curlimages/curl \\\n  -n incidentgpt \\\n  -- curl -v http://<prometheus-service>:9090/-/ready",
  },
  {
    title: "No Kubernetes context",
    checks: ["ServiceAccount name", "ClusterRoleBinding", "pods/events/nodes permissions", "ENRICH_K8S_CONTEXT"],
    commands: "kubectl auth can-i list pods \\\n  --as=system:serviceaccount:incidentgpt:<service-account> \\\n  --all-namespaces\n\nkubectl auth can-i list events \\\n  --as=system:serviceaccount:incidentgpt:<service-account> \\\n  --all-namespaces",
  },
  {
    title: "Redis connection refused",
    checks: ["redis service name", "endpoints", "password", "NetworkPolicy", "wrong Bitnami service name"],
    commands: "kubectl get svc,endpoints,pod -n incidentgpt | grep redis\n\nkubectl run redis-debug \\\n  --rm -it \\\n  --restart=Never \\\n  --image=redis:7-alpine \\\n  -n incidentgpt \\\n  -- redis-cli -h redis-master.incidentgpt.svc.cluster.local ping",
  },
  {
    title: "Alerts are not grouped",
    checks: ["same namespace", "fingerprint present or stable labels", "Redis works", "sent inside CORR_SETTLE", "look for ALERT_BUFFERED and GROUP_SENT"],
    commands: "kubectl logs -n incidentgpt deploy/incidentgpt-enricher | grep -E 'ALERT_BUFFERED|GROUP_SENT|redis'",
  },
  {
    title: "Telegram returns an error",
    checks: ["bot token", "channel ID starts with -100", "discussion group linked", "bot administrator rights", "Telegram API response in logs"],
    commands: "kubectl logs -n incidentgpt deploy/ai-worker",
  },
  {
    title: "OpenRouter returns 401",
    checks: ["wrong key", "Secret not mounted", "newline in value", "wrong endpoint", "variable missing in pod"],
    commands: "kubectl exec -n incidentgpt deploy/ai-worker -- \\\n  sh -c 'test -n \"$OPENROUTER_API_KEY\" && echo configured || echo missing'",
  },
];

export function TroubleshootingPage({ t }: PageProps) {
  return (
    <article className="doc-page">
      <div className="troubleshooting-grid">
        {cards.map((card) => (
          <section className="problem-card" id={card.title.toLowerCase().replaceAll(" ", "-")} key={card.title}>
            <h2>{card.title}</h2>
            <ul>{card.checks.map((check) => <li key={check}>{check}</li>)}</ul>
            <CodeBlock code={card.commands} language="bash" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
          </section>
        ))}
      </div>
    </article>
  );
}
