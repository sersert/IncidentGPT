#!/usr/bin/env bash
# Секреты, opt-in и receiver grants мониторинга.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
for chart in backend/chart ai-worker/chart enricher/chart/incidentgpt-enricher; do
  helm template incidentgpt "$chart" >"$scratch/default"
  if grep -E 'name: (OPERATIONS_ENABLED|AI_CONTROL_URL)' "$scratch/default"; then
    echo 'Operations unexpectedly enabled by default' >&2
    exit 1
  fi
  if helm template incidentgpt "$chart" --set operations.enabled=true >"$scratch/out" 2>"$scratch/error"; then
    echo 'Enabled operations without existingSecret' >&2
    exit 1
  fi
  grep -F existingSecret "$scratch/error" >/dev/null
done
if helm template incidentgpt backend/chart --set operations.enabled=true --set operations.existingSecret=ops >"$scratch/out" 2>"$scratch/error"; then
  echo 'Backend operations enabled without shared administrator credentials' >&2
  exit 1
fi
grep -F 'reader credentials' "$scratch/error" >/dev/null
helm dependency build --skip-refresh deploy/incidentgpt >"$scratch/dependencies"
helm template incidentgpt deploy/incidentgpt --namespace preview -f docs/examples/values-operations.yaml >"$scratch/enabled"
test "$(grep -c 'name: OPERATIONS_SERVICE_TOKEN' "$scratch/enabled")" = 3
test "$(grep -c 'key: operations-service-token' "$scratch/enabled")" = 3
grep -F 'name: AI_CONTROL_URL' "$scratch/enabled" >/dev/null
grep -F 'name: incidentgpt-backend-to-ai-worker' "$scratch/enabled" >/dev/null
grep -F 'app: incidentgpt-backend' "$scratch/enabled" >/dev/null
helm template incidentgpt backend/chart --set operations.enabled=true --set operations.existingSecret=ops --set readerAuth.existingSecret=reader --set operations.mcpHealthURL=http://knowledge-mcp:8080/healthz >"$scratch/mcp"
grep -F 'app.kubernetes.io/component: knowledge-mcp' "$scratch/mcp" >/dev/null
echo 'Operations chart checks passed.'
