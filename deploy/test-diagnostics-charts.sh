#!/usr/bin/env bash
# Проверяем ошибочные настройки отдельно: дефолтный render их не обнаруживает.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

reject() {
  local expected=$1
  shift
  if helm template diagnostics "$@" >"$scratch/out" 2>"$scratch/error"; then
    echo "Expected Helm to reject: $*" >&2
    exit 1
  fi
  grep -F "$expected" "$scratch/error" >/dev/null
}

enricher=enricher/chart/incidentgpt-enricher
backend=backend/chart
reject changes.existingSecret "$enricher" --set diagnostics.enabled=true
reject diagnostics.namespaces "$enricher" --set diagnostics.enabled=true --set changes.existingSecret=fixture
reject diagnostics.namespaces "$enricher" --set diagnostics.enabled=true --set changes.existingSecret=fixture --set-string 'diagnostics.namespaces=payments\,'
reject DURABLE_ANALYSIS_ENABLED "$backend" --set-string env.GUIDED_DIAGNOSTICS_ENABLED=true
reject changes.existingSecret "$backend" --set-string env.GUIDED_DIAGNOSTICS_ENABLED=true --set-string env.DURABLE_ANALYSIS_ENABLED=true
reject DIAGNOSTIC_COLLECTOR_URL "$backend" --set-string env.GUIDED_DIAGNOSTICS_ENABLED=true --set-string env.DURABLE_ANALYSIS_ENABLED=true --set changes.existingSecret=fixture --set-string env.DIAGNOSTIC_COLLECTOR_URL=

# Другой namespace и разрешённая конфигурация должны рендериться успешно.
helm template diagnostics "$enricher" -n diagnostics-preview --set diagnostics.enabled=true --set diagnostics.namespaces=payments --set changes.existingSecret=fixture >"$scratch/enricher"
helm template diagnostics "$backend" -n diagnostics-preview --set-string env.GUIDED_DIAGNOSTICS_ENABLED=true --set-string env.DURABLE_ANALYSIS_ENABLED=true --set changes.existingSecret=fixture >"$scratch/backend"
grep -F 'optional: false' "$scratch/enricher" >/dev/null
grep -F 'optional: false' "$scratch/backend" >/dev/null
grep -F 'http://incidentgpt-enricher:9099/v1/diagnostics' "$scratch/backend" >/dev/null
echo 'Diagnostics chart configuration checks passed.'
