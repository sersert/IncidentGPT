#!/usr/bin/env bash
# MCP отключён по умолчанию; включение требует явных источников и Secrets.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
if helm template knowledge knowledge-mcp/chart >"$scratch/out" 2>"$scratch/error"; then
  echo 'MCP chart accepted missing credentials' >&2
  exit 1
fi
grep -F secrets.existingSecret "$scratch/error" >/dev/null
if helm template worker ai-worker/chart --set knowledgeMCP.enabled=true >"$scratch/out" 2>"$scratch/error"; then
  echo 'Worker accepted MCP without credentials' >&2
  exit 1
fi
grep -F 'knowledgeMCP requires existingSecret' "$scratch/error" >/dev/null
helm dependency build --skip-refresh deploy/incidentgpt >"$scratch/dependencies"
helm template candidate deploy/incidentgpt >"$scratch/default"
if grep -F 'name: KNOWLEDGE_MCP_URL' "$scratch/default" >/dev/null; then
  echo 'MCP unexpectedly enabled by default' >&2
  exit 1
fi
helm template candidate deploy/incidentgpt -f docs/examples/values-mcp-knowledge.yaml >"$scratch/enabled"
grep -F 'name: KNOWLEDGE_MCP_URL' "$scratch/enabled" >/dev/null
grep -F 'name: KNOWLEDGE_MCP_TOKEN' "$scratch/enabled" >/dev/null
grep -F 'secretName: incidentgpt-knowledge-sources' "$scratch/enabled" >/dev/null
grep -F 'app.kubernetes.io/component: knowledge-mcp' "$scratch/enabled" >/dev/null
grep -F 'kind: NetworkPolicy' "$scratch/enabled" >/dev/null
echo 'Knowledge MCP chart configuration checks passed.'
