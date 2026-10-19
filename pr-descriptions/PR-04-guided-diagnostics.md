# feat: add guided read-only incident diagnostics

**Status:** Implemented locally; not published.

## Problem and behavior

Initial analysis often lacks observations needed to distinguish competing hypotheses. This change adds refresh, hypothesis checks and before/after change comparisons backed by typed read-only collectors. It retains operation/evidence history and creates a new structured analysis version with previous-analysis context and cited confirmed-resolution snapshots.

Backend resources/permissions and collector cluster/namespace/tool/time bounds are enforced independently of the model. Questions, logs and runbooks cannot expand tools or execute actions. Failed sources remain explicitly partial; successful empty queries differ from disabled, failed, timed-out and unauthorized sources. UI includes queued/running/canceled/partial/failed/complete states, collection timestamps and safe cancellation.

## Scope and dependencies

Base: `feat/confirmed-incident-memory` at `ab158bf`; head: `feat/guided-incident-diagnostics`, commit in index. Depends on PRs 2–3 job/evidence/confirmed-memory contracts and PR 1 change identities. Rebase/retarget after predecessor merge and replay/revalidate successors. No terminal, arbitrary plugin framework, remediation or autonomous investigation loop. Prepared locally; not published.

## API, data and configuration

New incident-scoped investigation GET/POST/cancel routes use backend-verified Basic credentials. Internal enricher POST /v1/diagnostics uses service authentication and explicit DIAGNOSTIC_NAMESPACES. Fixed operations read pod termination/events, rollout/node conditions, parameterized Prometheus restart metrics and bounded configured OpenSearch/Elasticsearch logs. Compare validates a stored change association/source time and queries bounded historical before/after windows.

DiagnosticOperation/Request/Result contracts precede consumers. Atomic scheduling enforces one active investigation per incident. Collected observations persist before reasoning; retries retain collection time. Lease/cancellation fences prevent stale output publication. Optional disabled-by-default flags, collector URL/namespace settings and minimal internal NetworkPolicy are in charts; no Kubernetes write/exec RBAC is added. Details, budgets, retention and rollback: `docs/roadmap/guided-diagnostics.md`.

## Compatibility and migration

No legacy observation is relabeled fresh, and live snapshots do not claim historical coverage. Provider retries reuse collected evidence; new refresh creates a new analysis. Confirmed history remains cited data with version/environment caveats and never runs automatically. Source availability is explicit. Existing incident/Telegram/legacy Markdown paths remain. Deploy compatible consumers before enqueueing new diagnostic kinds; no fabricated backfill.

## Validation

Passed:

- Docker backend/enricher vet, build and race suites with disposable Redis using `docker compose -p incidentgpt-regression -f deploy/test-compose.yaml run --rm <service> sh -c 'go vet ./... && go build -o /tmp/service . && go test -race ./...'`.
- Tests cover incident-derived resource scope, rejected user scope overrides/unsupported exec/foreign cluster/unlisted namespace, per-incident concurrency, persisted partial observations, retry without recollection, cancellation fencing and preservation of earlier evidence.
- Collector fixtures exercise successful-empty metrics versus source outage/disabled logs and real OOMKilled termination metadata through GET-only Kubernetes requests. Existing structured-output tests enforce untrusted data/citation boundaries.
- UI `npx tsc --noEmit -p tsconfig.app.json`, `npm run build`; existing bundle warning remains.
- Component/umbrella Helm lint/render and `git diff --check`.

Initial failure corrected: a local chart-edit script overwrote enricher defaults with backend values. Restored the predecessor's enricher defaults and added only diagnostics settings; Helm lint passed. Not run: production cluster/integrations, browser interaction tests or live-provider evaluation. Deterministic fixture providers cover the implemented boundaries; no autonomous actions were taken.

## Rollout and rollback

Enable backend/collector flags only after compatible code and service/reader/sanitizer credentials. Select explicit namespaces and a small reviewed scope; observe partial-source behavior and queue state. To rollback, disable new requests, cancel/drain pending operations and retain completed evidence/history before downgrading. Redis persistence/backup assumptions remain those of PR 2.

## Limitations and reviewer focus

One investigation per incident; bounded fixed operations, provider deadlines and four attempts. Questions do not dynamically invent tools. Historical comparison requires a known source change timestamp and configured ranged metrics/logs; it cannot reconstruct old live Kubernetes state. Cancellation fences publication but in-flight reads may finish within deadline. Source settings use actual enricher configuration, not assumed UI form wiring. Review scope/auth separation, generated queries, observation-time semantics, cancellation/recovery, partial evidence, cited confirmed context and absence of write capability.
