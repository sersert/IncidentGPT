#!/usr/bin/env bash
# Совместимый Basic по умолчанию и явная настройка сессий.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
helm template login ui/chart >"$scratch/basic"
grep -A1 'name: AUTH_MODE' "$scratch/basic" | grep -F 'value: "basic"' >/dev/null
if helm template login ui/chart --set authMode=invalid >"$scratch/out" 2>"$scratch/error"; then exit 1; fi
grep -F 'authMode must be' "$scratch/error" >/dev/null
helm template login ui/chart --set auth.existingSecret=external-auth >"$scratch/external"
if grep -F 'kind: Secret' "$scratch/external"; then echo 'Generated unused htpasswd Secret' >&2; exit 1; fi
helm dependency build --skip-refresh deploy/incidentgpt >"$scratch/dependencies"
if helm template login deploy/incidentgpt --set incidentgpt-ui.authMode=session >"$scratch/out" 2>"$scratch/error"; then exit 1; fi
grep -F browserSessions "$scratch/error" >/dev/null
if helm template login deploy/incidentgpt --set incidentgpt-ui.authMode=session --set incidentgpt-backend.browserSessions.enabled=true >"$scratch/out" 2>"$scratch/error"; then exit 1; fi
grep -F existingSecret "$scratch/error" >/dev/null
helm template login deploy/incidentgpt --set incidentgpt-ui.authMode=session --set incidentgpt-backend.browserSessions.enabled=true --set incidentgpt-backend.readerAuth.existingSecret=reader >"$scratch/session"
grep -A1 'name: AUTH_MODE' "$scratch/session" | grep -F 'value: "session"' >/dev/null
grep -A1 'name: BROWSER_SESSIONS_ENABLED' "$scratch/session" | grep -F 'value: "true"' >/dev/null
if grep -F 'mountPath: /etc/nginx/secrets' "$scratch/session"; then echo 'Session mode unexpectedly requires htpasswd' >&2; exit 1; fi
echo 'Browser auth chart checks passed.'
