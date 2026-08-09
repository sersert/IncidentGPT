export const directAlertPayload = `{
  "receiver": "incident-enricher",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "SyntheticHighCPU",
        "severity": "critical",
        "namespace": "monitoring",
        "pod": "synthetic-pod-1",
        "node": "worker-1",
        "instance": "10.0.0.1:9100"
      },
      "annotations": {
        "summary": "Synthetic test alert",
        "description": "CPU usage exceeded the test threshold"
      },
      "startsAt": "2026-08-04T07:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "fingerprint": "test-high-cpu-001"
    }
  ]
}`;

export const cascadeScript = `#!/usr/bin/env bash

set -euo pipefail

ENDPOINT="\${ENDPOINT:-http://localhost:9099/alert}"
NAMESPACE="\${NAMESPACE:-incident-demo}"

send_alert() {
  local alert_name="$1"
  local fingerprint="$2"
  local summary="$3"

  curl --fail-with-body \\
    --silent \\
    --show-error \\
    --request POST \\
    --header "Content-Type: application/json" \\
    "\${ENDPOINT}" \\
    --data "{
      \\"receiver\\": \\"incident-enricher\\",
      \\"status\\": \\"firing\\",
      \\"alerts\\": [{
        \\"status\\": \\"firing\\",
        \\"labels\\": {
          \\"alertname\\": \\"\${alert_name}\\",
          \\"severity\\": \\"critical\\",
          \\"namespace\\": \\"\${NAMESPACE}\\"
        },
        \\"annotations\\": {
          \\"summary\\": \\"\${summary}\\"
        },
        \\"startsAt\\": \\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\\",
        \\"endsAt\\": \\"0001-01-01T00:00:00Z\\",
        \\"fingerprint\\": \\"\${fingerprint}\\"
      }]
    }"

  echo
}

send_alert "SyntheticDatabaseUnavailable" "demo-database" "Database is unavailable"
send_alert "SyntheticBackendErrors" "demo-backend" "Backend error rate increased"
send_alert "SyntheticFrontendLatency" "demo-frontend" "Frontend latency increased"`;

export const alertmanagerSnippet = `alertmanager:
  config:
    route:
      receiver: incident-enricher
      routes:
        - receiver: incident-enricher
          continue: true
    receivers:
      - name: incident-enricher
        webhook_configs:
          - url: >-
              http://incidentgpt-enricher.incidentgpt.svc:9099/alert
            send_resolved: true`;

export const telegramMock = `🔥 [FIRING] KubePodCrashLooping (critical)

Labels:
 - alertname = KubePodCrashLooping
 - namespace = payments
 - pod = payment-api-6d9bbd9df7-k4g2p
 - severity = critical

Annotations:
 - summary = Pod is restarting repeatedly

Starts: 2026-08-09T10:34:12Z

---

🔥 [FIRING] 3 связанных алертов (grp:payments)

1. [FIRING] KubePodCrashLooping (critical) → pod=payment-api-6d9bbd9df7-k4g2p
   Pod is restarting repeatedly
2. [FIRING] KubeDeploymentReplicasMismatch (warning) → deployment=payment-api
   Deployment has not matched the expected replicas
3. [FIRING] HTTP5xxRateHigh (critical) → service=payment-api
   Error rate above 5% for 5m

---

🤖 AI-разбор

**Корень:** после выката payment-api под падает на старте — в логах ×212 повторов
«config key "DB_DSN" is missing». Расхождение реплик и рост 5xx — следствия.

**Цепочка:** выкат → падение пода → нехватка реплик → ошибки на входе.

**Исправление:**
1. Откатить последний выкат payment-api
2. Вернуть ключ DB_DSN в ConfigMap payments-api
3. Дождаться, пока число реплик вернётся к желаемому

**Профилактика:** проверка обязательных ключей конфигурации на старте пода,
readiness-проба до приёма трафика.

**Уверенность:** 78

Это гипотеза, финальное решение за инженером.`;
