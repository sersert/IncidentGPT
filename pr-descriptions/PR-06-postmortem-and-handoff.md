# feat: generate reviewed postmortems and incident handoffs

**Status:** Prepared locally; not published. Implemented as the sixth stacked change.

## Problem and implemented behavior

Production report paths lacked persisted reports, and analytics substituted incident
duration or guessed values for processing and recovery measurements. This change
generates bounded, cited Markdown postmortems and concise handoffs from immutable
stored source snapshots through the existing durable job consumers. Generation uses
deterministic templates and does not require another LLM call. Unconfirmed hypotheses,
unknown impact, missing timestamps and partial diagnostic sources remain explicit.

Authorized engineers can select versions, edit drafts, mark review status, maintain
owned action items and download sanitized Markdown. Optimistic revisions reject
conflicts; regeneration creates a new report without replacing human edits. The UI
shows pending/failed work, permissions, empty states and source composition warnings.

Separate lifecycle records capture symptom, receipt, acknowledgement, actual analysis
start/end, human-verified recovery and administrative closure. Analytics compute each
interval from its own events, with samples, missing counts, median and p90. Unsupported
MTTR reduction, confidence and saved-hours claims become null rather than estimates.

## Scope and dependencies

Base: `feat/dependency-aware-correlation` (`3b17568`). Head:
`feat/postmortem-and-handoff`. Depends on PRs 1–5; implementation commit is recorded
in the stack index. No external ticket creation, notification, production deployment,
autonomous action, or causal claim of AI benefit is included.

## API, data and configuration

`backend/reports.go`, `backend/lifecycle.go`, `backend/lifecycle_metrics.go` and
`docs/roadmap/postmortem-and-handoff.md` define the API and storage behavior.
Basic-authenticated GET/POST/PUT postmortem and handoff endpoints, authenticated
exports and GET/PUT lifecycle endpoints are additive. Report PUT now requires report
ID and revision; the UI client is migrated. Administrative status/assignee mutations
also verify backend credentials instead of trusting a supplied actor header.

`REPORTS_ENABLED` defaults false and generation requires durable analysis. Reports
reuse leases, retries and owner fencing. Sanitization precedes snapshot persistence,
generation publication and edits; failure never substitutes original content. Source
identity is protected across sanitization. Report sources retain incident composition,
analysis/evidence/change IDs and resolution/lifecycle revisions. Revision CAS and
immutable edit history preserve actor and UTC time. Exports use attachment,
no-store and nosniff headers; preview skips raw HTML/images and restricts links.

The umbrella chart generates matching development UI/backend reader credentials;
external htpasswd deployments must supply the matching backend reader Secret.
Browser mutations enforce same-origin when Origin is supplied. Nginx and the Vite
proxy preserve the request host, including its port. Permissions remain shared-admin,
with no tenant isolation claim.

## Compatibility, migration and operations

Existing incident pages and Markdown analyses remain readable. Legacy lifecycle
records derive only factual symptom/receipt timestamps from stored alerts; missing
analysis or recovery events remain unknown. Alert resolution never verifies recovery.
Knowledge text search now includes the stored confirmed failure mode. Dashboard
storage failures are surfaced instead of rendering an empty healthy result.

New analytics use an indexed maximum of 1,000 period incidents with truncation and
missing-data disclosure. Legacy volume/source handlers retain their prior scan
behavior. Reports retain at most 50 drafts per kind and expire after 90 days;
immutable source snapshots also remain in non-expiring durable job envelopes.
Lifecycle and audit history require operator retention/archival. Back up both records
and job history under PR 2's single-primary Redis/AOF durability assumptions.

Deploy compatible readers and matching credentials before enabling reports in a
limited environment. Verify citations, review/edit conflicts and recovery semantics.
For rollback, disable report generation, drain report/diagnostic/analysis work, then
disable durable correlation/collectors before reverting consumers. Preserve additive
history; older binaries do not render the new report/lifecycle fields.

## Validation

Passed:

- `docker compose -p incidentgpt-regression -f deploy/test-compose.yaml run --rm
  backend-tests sh -c 'go vet ./... && go build -o /tmp/service . && go test -race ./...'`.
- The same command using `enricher-tests` and `worker-tests`; backend and enricher
  exercised disposable Redis. The final enricher run includes PR 5's node-only case.
- In `sanitizer`: `go vet ./...` and `go test -race ./...`.
- In `enricher`: escalated host `go test -race ./...`; Redis-dependent tests use Docker.
- In `ui`: `npx tsc --noEmit -p tsconfig.app.json` and `npm run build`.
- `helm dependency build --skip-refresh deploy/incidentgpt`; backend and umbrella
  `helm lint`; `helm template` for defaults, enabled features, UI-disabled and external
  reader credentials. Rendered Secrets, environment variables and mounts inspected.
- `docker build -t incidentgpt-<component>:local-roadmap <component>` for backend,
  enricher, ai-worker, ui and sanitizer, using their production Dockerfiles.
- `git diff --check` and Go formatting checks.

`backend/testdata/incident-workflow.json` and `backend/reports_test.go` drive the
production router with deterministic local fake providers. Coverage includes change
ingestion, durable cited analysis, confirmed-memory retrieval, all six diagnostic
checks with partial availability, verified recovery, audited regrouping, snapshot
provenance, lease recovery and stale-owner fencing, concurrent generation, edit CAS,
regeneration preserving edits, handoff states and sanitized authorized export.
Separate tests reject cross-origin mutations and prevent missing recovery from
becoming AI latency; nearest-rank p90 handles small samples.

Failed checks corrected before successful reruns: an unused import after analytics
replacement; fixture search missing failure-mode indexing; incomplete diagnostic
fixture check records; Redis serialization of report json.RawMessage (converted to
bytes). Integrated graph serialization and baseline alert-binding issues were fixed
in PR 5. Initial Helm overrides used an incorrect umbrella prefix and were corrected.
A host enricher test could not bind its test server inside the sandbox; the required
escalated rerun passed. The UI build retains its existing large-bundle warning.

Not run: live Kubernetes/Argo CD collection, live provider or Telegram delivery,
interactive browser evaluation, production canary, load benchmark or cross-platform
image matrix. Image builds validate native Docker production builds, not deployment.

## Limitations and reviewer focus

Report input ceilings are 50 alerts, 80 evidence records and 25 changes/check jobs;
postmortem/handoff ceilings are 32/8 KiB. Excess inputs fail visibly. Metrics measure
the current completed analysis composition and disclose missing legacy events;
chronological trends are observational. Cancellation fences publication but is not
instant cross-replica abortion. Shared-admin authorization, bounded templates and
operator-managed history retention are deliberate constraints.

Review immutable provenance, report/job atomic publication, edit conflicts, sanitizer
identity checks, safe export, reader credential deployment and independent lifecycle
definitions. The offline fixture validates behavior without claiming live-provider
quality or measured reliability improvement.
