#!/usr/bin/env bash
# Проверяем дефолты и согласованность startup-бюджета с runtime-конфигурацией.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
chart=enricher/chart/incidentgpt-enricher
helm template review "$chart" >"$scratch/default"
if grep -F 'startupProbe:' "$scratch/default" >/dev/null; then
 echo 'Legacy install unexpectedly has a startup probe' >&2
 exit 1
fi
helm template review "$chart" --set changes.durableCorrelation=true --set changes.existingSecret=runtime --set redisStartup.timeoutSeconds=45 >"$scratch/durable"
grep -F 'startupProbe:' "$scratch/durable" >/dev/null
grep -F 'value: "45s"' "$scratch/durable" >/dev/null
grep -F 'failureThreshold: 38' "$scratch/durable" >/dev/null
if helm template review "$chart" --set changes.durableCorrelation=true --set changes.existingSecret=runtime --set redisStartup.timeoutSeconds=0 >"$scratch/out" 2>"$scratch/error"; then
 echo 'Invalid Redis startup budget accepted' >&2
 exit 1
fi
grep -F redisStartup.timeoutSeconds "$scratch/error" >/dev/null
helm template review backend/chart --set serviceGraph.existingConfigMap=topology --set serviceGraph.key=custom.json --set env.SERVICE_GRAPH_FILE=/ignored >"$scratch/graph"
test "$(grep -c 'name: SERVICE_GRAPH_FILE' "$scratch/graph")" = 1
grep -F 'value: /etc/incidentgpt/topology/dependencies.json' "$scratch/graph" >/dev/null
grep -F 'key: "custom.json"' "$scratch/graph" >/dev/null
grep -F 'path: dependencies.json' "$scratch/graph" >/dev/null
echo 'Dependency graph and startup configuration checks passed.'
