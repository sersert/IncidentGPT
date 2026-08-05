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

export const telegramMock = `🔥 FIRING: KubePodCrashLooping

Cluster: production-eu-1
Namespace: payments
Pod: payment-api-6d9bbd9df7-k4g2p
Severity: critical
Started: 10:34:12 UTC

Summary:
Pod is restarting repeatedly.

---

🧩 Incident summary: 3 related alerts

1. KubePodCrashLooping
2. KubeDeploymentReplicasMismatch
3. HTTP5xxRateHigh

---

🤖 AI-assisted incident analysis

Probable root cause:
The payment-api pod started crashing after a new deployment.
The replica mismatch and elevated HTTP 5xx rate are likely consequences.

Check first:
1. kubectl logs --previous
2. deployment rollout history
3. container exit code
4. recent ConfigMap and Secret changes
5. node memory pressure

Confidence:
Medium

This is an engineering draft. Verify before taking action.`;
