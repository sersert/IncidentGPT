# feat: correlate incidents using explicit dependency evidence

**Status:** Prepared locally; not published. Implemented as the fifth stacked change.

## Problem and behavior

Namespace-only grouping combines independent workloads and misses cross-namespace
symptoms sharing a configured dependency. This change persists cluster-aware typed
graph nodes/edges, metadata provenance and freshness; adds optional actual Kubernetes
ownership/placement submission and a versioned declared dependency configuration;
and uses bounded, deterministic one-hop correlation with explicit explanations.
Missing/conflicting/stale topology retains conservative workload grouping. Node-only alerts can group by a reported cluster/node; applications are never joined solely by placement. Legacy buffering also records its namespace/window explanation. The UI
shows relationships, unknown health, alert explanations, and authorized move/split
controls. These explanations are separate from root-cause hypotheses.

Manual composition changes atomically preserve stable alert/incident identities,
original evidence/analysis history and audit actor/time. Both pages link to their
historical counterpart. A move clears current interpretation pointers and fences
late completion; the history UI compares the stored analysis composition against
the current group. Refresh requests a new analysis.

## Scope, dependencies and branches

Base: `feat/guided-incident-diagnostics` (`a3a621b`). Head:
`feat/dependency-aware-correlation`; local commit recorded in README.md after commit.
Depends on PRs 1–4 contracts, Redis jobs and diagnostics. No graph database, tracing,
causal detection, bulk merge command, or autonomous remediation. Consolidation uses
bounded individual moves.

## API, data and configuration

See `docs/roadmap/dependency-correlation.md`, `backend/graph.go`,
`backend/regroup.go`, `enricher/dependencies.go` and the graph contract shared with
the enricher. Basic-authenticated graph reads and declaration PUT; Bearer-authenticated
metadata POST; Basic-authenticated incident regroup POST. New Redis indices are
bounded; graph storage is retained 30 days with shorter freshness policies.
Audit/composition history has no automatic expiry. Sanitization precedes graph persistence.

`DEPENDENCY_CORRELATION_ENABLED` and `TOPOLOGY_OBSERVATIONS_ENABLED` default false.
Correlation requires durable buffering. `SERVICE_GRAPH_FILE` optionally reads a
bounded local configuration refreshed every ten minutes. Backend ConfigMap mounting
and enricher Helm switches are included. The fixture contains two services sharing
one database. Declaration replacement removes previous declared edges immediately.

## Compatibility, migration and operations

Old alerts show an unknown grouping reason. No historical graph is fabricated.
Drain legacy batches before activating new grouping/manual correction. Durable ingest
and regroup share a single composition lock; atomic CAS also rejects concurrent
record edits. Global serialization deliberately lowers maximum ingest throughput.
Shared-admin authorization does not claim tenant isolation. Persist and back up Redis
with the PR 2 single-primary/AOF assumptions; include non-expiring audit history.

Roll out topology observation/configuration first, inspect coverage, then enable
durable dependency correlation for a limited environment. Disable correlation and
observation before rollback; drain durable batches before reverting their consumer.
Retain additive graph and manual history records. Old binaries do not render source
links or composition warnings.

## Validation

Passed:
- Docker `enricher-tests`: `go vet ./... && go build -o /tmp/service . && go test -race ./...`.
- Docker `backend-tests`: same commands against the real Redis test service.
- `ui`: `npx tsc --noEmit -p tsconfig.app.json`; `npm run build` (existing bundle-size warning).
- `helm dependency build --skip-refresh deploy/incidentgpt`; `helm lint deploy/incidentgpt`;
  `helm template regression deploy/incidentgpt` (default rendering), and enabled
  rendering with `--set incidentgpt-backend.serviceGraph.existingConfigMap=topology
  --set incidentgpt-backend.runbooks.existingConfigMap=runbooks`. The original
  attempted overrides used the wrong umbrella prefix; corrected and explicitly
  inspected both rendered mounts.

Initial failed checks: backend vet found an incorrect BasicAuth return count;
backend tests then found normalization mutating a rejected document. Both corrected
before the successful rerun. The split-ingest fixture initially omitted firing status; corrected and isolated fixture identifiers before its successful rerun. Integrated API testing later exposed direct Redis serialization of json.RawMessage; converted to bytes and added full graph API persistence tests before finalizing this unpublished commit. Tests exercise cluster separation, metadata/dependency
type boundaries, unrelated workloads, shared dependencies, stale/placement fallback,
stable split/history, stale-result fencing, duplicate move rejection, later automatic ingestion preserving human separation, full graph API persistence/withdrawal, sanitizer outage, finite incident composition and closed-alert recurrence.

Not run: production Kubernetes observation, tracing, browser interaction or load
benchmark. Production image builds are recorded with the integrated final validation.

## Limitations and reviewer focus

One outgoing dependency is considered; conflicting dependency candidates degrade.
Cycles are never traversed beyond one hop. Durable ingestion refuses growth beyond 50 incident alerts with a visible 503, leaving the recoverable batch unacknowledged. A 50-alert group is also the correction limit; oversized legacy records are rejected. Newly created standalone alert records now retain incident bindings, and a closed incident’s firing alert can recur as a new episode. Metadata
submission failures preserve unknown coverage; invalid configuration remains absent
rather than becoming healthy. Inspect lock/CAS behavior, declared edge replacement,
source links, sanitizer identity protection and composition invalidation.
