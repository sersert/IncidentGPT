#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
helm template evaluation backend/chart >/dev/null
helm template evaluation ai-worker/chart >/dev/null
if helm template evaluation backend/chart --set-string env.EVALUATION_ENABLED=true >/dev/null 2>&1; then
  echo 'backend evaluation accepted without credentials' >&2; exit 1
fi
helm template evaluation backend/chart --set-string env.EVALUATION_ENABLED=true --set changes.existingSecret=fixture >/dev/null
if helm template evaluation ai-worker/chart --set-string env.EVALUATION_ENABLED=true >/dev/null 2>&1; then
  echo 'worker evaluation accepted without profiles and credential Secret' >&2; exit 1
fi
helm template evaluation ai-worker/chart --set-string env.EVALUATION_ENABLED=true --set secrets.existingSecret=fixture --set-json 'env.EVALUATION_PROFILES_JSON="[{\"id\":\"alpha\",\"label\":\"Alpha\",\"model\":\"fixture/model\"}]"' >/dev/null
echo 'Evaluation chart configuration guards passed'
