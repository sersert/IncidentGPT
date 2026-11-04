#!/usr/bin/env bash
# Проверяем совместимость старого доступа и явную настройку новых API.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
reject() {
  local expected=$1
  shift
  if helm template reports backend/chart "$@" >"$scratch/out" 2>"$scratch/error"; then
    echo "Expected Helm rejection: $*" >&2
    exit 1
  fi
  grep -F "$expected" "$scratch/error" >/dev/null
}
reject DURABLE_ANALYSIS_ENABLED --set-string env.REPORTS_ENABLED=true
reject changes.existingSecret --set-string env.REPORTS_ENABLED=true --set-string env.DURABLE_ANALYSIS_ENABLED=true
reject readerAuth.existingSecret --set-string env.LEGACY_API_AUTH_REQUIRED=true
reject 'not both' --set readerAuth.existingSecret=reader --set changes.existingSecret=runtime
helm template reports backend/chart --set-string env.REPORTS_ENABLED=true --set-string env.DURABLE_ANALYSIS_ENABLED=true --set changes.existingSecret=runtime >"$scratch/reports"
helm template reports backend/chart --set readerAuth.existingSecret=reader --set-string env.LEGACY_API_AUTH_REQUIRED=true >"$scratch/reader"
test "$(grep -c 'optional: false' "$scratch/reader")" = 2
helm dependency build --skip-refresh deploy/incidentgpt >"$scratch/dependencies"
helm template reports deploy/incidentgpt >"$scratch/default"
helm template reports deploy/incidentgpt --set incidentgpt-ui.auth.existingSecret=external-htpasswd >"$scratch/external"
for file in default external; do
  if grep -F 'name: incidentgpt-backend-reader' "$scratch/$file" >/dev/null; then
    echo 'Unexpected automatic reader credentials' >&2
    exit 1
  fi
done
helm template reports deploy/incidentgpt --set incidentgpt-ui.auth.existingSecret=external-htpasswd --set incidentgpt-backend.readerAuth.existingSecret=matching-reader >"$scratch/external-reader"
grep -F 'name: matching-reader' "$scratch/external-reader" >/dev/null
echo 'Report chart configuration checks passed.'
