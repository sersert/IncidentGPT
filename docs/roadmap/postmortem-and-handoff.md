# Reports, handoff and lifecycle metrics

Reports are deterministic Markdown drafts from stored, bounded snapshots. No extra
LLM call is made for generation. Every draft retains captured UTC time, incident
composition, selected analysis ID, evidence IDs/excerpts, relevant change IDs,
resolution revision and lifecycle revision. Human facts identify the resolution
actor/revision; model hypotheses remain unresolved unless a separate human
resolution confirms them. A confirmation from an older/unknown composition is
explicitly flagged for applicability review. Customer counts, money, contributing
factors and missing timestamps are unknown. Report content is data, never tool input.

Set `incidentgpt-backend.env.REPORTS_ENABLED=true` in umbrella values after enabling
durable analysis and service authentication. Generation is disabled by default.
The backend needs the existing sanitizer URL/shared HMAC secret and Redis. It reuses
the four durable job consumers, 90-second ownership lease and four-attempt budget.
Only one report job per incident is accepted at once. A provider outage does not
block a report with already stored sources; missing/inaccessible source records
produce an explicit failure rather than an uncited replacement.

## API and workflow

All paths below are prefixed `/api/v1`, verify backend Basic credentials independently
and reject X-Auth-User as identity. Match the configured reader credentials to the
UI nginx admin. The umbrella development defaults generate a reader Secret from the existing UI account. For external htpasswd, supply a matching `incidentgpt-backend.readerAuth.existingSecret` with `username`/`password`, or the existing changes Secret reader password and configured username. Missing credentials disable authorized mutations, while the backend can still start. These are shared-admin permissions, not team tenancy.

- GET/POST/PUT `/incidents/{id}/postmortem` and `/incidents/{id}/handoff`.
- GET `/incidents/{id}/postmortem/export?report_id=...` and the handoff equivalent.
- GET/PUT `/incidents/{id}/lifecycle`.

POST returns 202 with `status: generating`, job ID and new report ID. GET returns
`items`, `jobs`, generation `enabled`, and the selected/latest report's compatible
`markdown`, `status: ready`, `generated_at`, edit metadata, revision, source and
review status. Empty installations return `status: empty` and no fabricated report.
The existing UI API is migrated: PUT now requires `report_id`, `revision`,
`markdown`, `review_status` (`draft` or `reviewed`) and optional `action_items`.
Conflicting edits return 409 and leave the human draft intact. Regeneration creates
a different report ID; it cannot overwrite prior edits. Each revision is audited.

An action item has `text`, `owner`, `status` (`open`, `in_progress`, `done`) and
optional ISO `due_date`. Up to 30 items are supported. They are appended to the
exported Markdown; no external issue or message is sent. Handoff selects concise
impact/owner, current interpretation, human actions and investigation state sections,
with an 8 KiB ceiling. Postmortems have a 32 KiB ceiling, at most 50 incident alerts,
80 cited evidence records and 25 associated changes/check jobs. Over-budget inputs
fail visibly. It is a bounded reporting workflow, not unlimited history export.

The UI provides version selection, editing, review state, action fields, source
references, queued/running/failure status, preview and download. Preview escapes
text, skips raw HTML/images and permits safe HTTPS/internal links. Downloads are
Markdown attachments with no-store/nosniff headers. Snapshot, generated record and
human edits are sanitized before persistence; sanitizer failure never exports new
original content. Existing sanitized drafts remain readable during an outage.

## Lifecycle semantics

First known symptom comes from a valid source StartsAt; alert receipt comes from
backend receipt, not symptom time. Ingest records these separately in a lifecycle
record. Legacy records can derive those two facts from stored alerts, with revision
zero until an actual event is persisted. Unknown timestamps are not backfilled
from incident creation or the current time. Administrative PATCH actions now also
verify Basic credentials, record acknowledgement/closure events and trusted actor;
source alert resolution does not create a recovery or human closure event.

PUT lifecycle accepts `revision`, `verified_recovery_at` and available incident-owned
`evidence_ids`. This is an explicit human verification with actor and recorded time,
not automatic interpretation of alert resolution. Future recovery times, times
before the known symptom, foreign/failed evidence and stale edits are rejected.
Lifecycle events/history have no automatic expiry. First acknowledgement remains
the first observed acknowledgement; closure stores the latest administrative close.
Reopening does not erase the historical closure. Recovery is independently correctable
with optimistic concurrency and retained prior revisions.

AI processing time is completed structured analysis end minus start. Acknowledgement
is receipt to acknowledgement; recovery is symptom to human-verified recovery;
closure is receipt to administrative close. Negative/missing intervals are excluded
and exposed as missing counts/null values. Analyses invalidated by composition are
excluded from current-sample processing metrics until refreshed. Legacy Markdown
has no measured processing start, so no latency is invented for it.

Dashboard and lifecycle analytics read at most 1,000 indexed incidents in the chosen
7/30/90-day period, exposing `bounded`, samples and missing counts. Summary includes
mean, median and p90 seconds separately for the four lifecycle metrics. Trend cohorts
use incident creation day UTC, chronological order and per-series sample counts.
Old before/after reductions, confidence averages, guessed response time and estimated
saved hours are null; no causal AI benefit is claimed. The all-time alert counter is
labeled separately from period incident counts; no incompatible reduction ratio is
computed. Legacy volume/top-source list APIs retain their prior scan behavior.

## Storage, migration and rollback

Report current/revision records and indices expire after 90 days; lists show the
latest 50 drafts per kind. Edits refresh retention. Immutable report source snapshots
also live in their durable job envelope, which follows PR 2's non-expiring job history;
plan archival/purge for that history and back up both. Incident, resolution, manual
audit and lifecycle records also require an operator retention policy. Redis remains
a single-primary multi-key transaction service, not Redis Cluster; AOF every-second
and disk/replication failures retain their documented loss windows.

Deploy compatible contracts/readers before enabling producers. Existing original
incident pages and Markdown remain readable. The integrated fixture also verifies the PR 5 fix for a
baseline bug: newly created incident alerts now persist their incident binding,
allowing later audited moves and consistent alert detail links. Existing unbound
legacy alert records are not silently assigned; correction rejects a conflicting
binding rather than rewriting history.

Disable report generation, drain report/diagnostic/analysis work, then disable durable
correlation/collectors before reverting consumers. Retain additive records; old binaries
do not render reports or lifecycle fields. Disable optional costly workflows without
deleting prior artifacts. No deployment, branch publication or third-party messages
are part of local validation.

## Offline regression

`backend/testdata/incident-workflow.json` contains sanitized input data and relative
change/symptom offsets. `TestOfflineIncidentWorkflow` drives real handlers, Redis
jobs and fencing with local fake structured worker, read-only collector and sanitizer.
It covers change → incident → cited analysis → confirmed memory → diagnostic evidence
→ manual split → reports, revision conflicts, regeneration, handoff and safe export.
It also checks source injection cannot alter the diagnostic scope, unknown cause,
sanitizer failure, trusted actors and stale ownership. Source-independent duration
checks ensure missing recovery never becomes AI latency or incident duration.

Run only against disposable Redis:

```
docker compose -p incidentgpt-regression -f deploy/test-compose.yaml run --rm backend-tests
docker compose -p incidentgpt-regression -f deploy/test-compose.yaml run --rm enricher-tests
docker compose -p incidentgpt-regression -f deploy/test-compose.yaml run --rm worker-tests
```

CI supplies Redis and runs module race tests, including this fixture; no live provider
is enabled. A real-provider or Kubernetes/browser evaluation must be separately
configured and is not a claimed outcome of the deterministic harness.
