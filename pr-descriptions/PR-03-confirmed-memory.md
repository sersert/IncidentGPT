# feat: retrieve confirmed incident resolutions and runbooks

**Status:** Draft for upstream review. PRs #16 and #17 are merged; isolated candidate deployment verification remains blocked by the unavailable local Docker daemon.

## Problem and behavior

Feedback ratings and AI hypotheses do not prove a successful historical fix. This change adds explicit human-confirmed resolutions with restoring actions, failed attempts, verification evidence and environment/version caveats. Corrections/withdrawals retain history and immediately affect future retrieval. Unknown causes stay unknown. Existing helpful/unhelpful feedback is untouched.

`backend/knowledge.go` replaces empty knowledge search/pattern handlers with bounded indexed Redis queries and deterministic ranking explanations. `ui/src/components/IncidentMemory.tsx` provides a real resolution editor and confirmed-solution cards with differences, citations and source hashes. A configured read-only Markdown directory supplies bounded sanitized runbook snippets; it cannot fetch arbitrary URLs or execute instructions.

## Scope and dependencies

Target: `sersert/IncidentGPT:main` at `2983daa48ff85b23a4560c6c5898d3ab8bd863b8`, after squash merges of PRs #16 and #17. Head: `opsmon:feat/confirmed-incident-memory-upstream`. Only the original confirmed-memory change (`ab158bfaba216751c64f896f38c31cb8add62223`) was replayed onto current upstream main, preserving its author. Previously merged changes and later diagnostic/correlation/report features are excluded. Depends on the merged incident-owned evidence and reader/sanitizer boundaries. Excludes semantic databases, fine-tuning and runbook execution. Later stack branches still need replay and validation after this PR merges.

## API, data and configuration

New incident resolution GET/PUT routes use optimistic revision checks and backend-verified Basic identity; actor headers cannot confirm a cause. Knowledge search preserves results/total and adds structured resolutions, matching features, differences and bounded candidate status. Pattern counts remain a list and require cluster scope. Confirmation checks available evidence ownership; sanitization and Redis failures return errors.

Current/history resolution keys and cluster/service sorted-set indexes update atomically. Runbooks retain identity/version/modification/collection metadata and immutable sanitized Redis excerpts with 30-day TTL. Resolution retention follows incidents (no automatic deletion). Local ConfigMap mount and RUNBOOK_DIRECTORY configuration are optional/disabled by default. Operating details and backup/rollback assumptions: `docs/roadmap/confirmed-memory.md`.

## Compatibility and migration

No feedback/AI record is backfilled as confirmed. Existing incidents and feedback stay readable. New knowledge cards use the structured resolution payload; the old mock-only scalar result shape is not claimed as a working backend contract. Current deployment retains shared-admin scope and cluster-aware matching; no per-team tenant isolation is claimed. Clean installs return empty/unknown states rather than examples. Runbook source symlinks must stay inside the configured root, including projected ConfigMap links.

## Historical implementation validation

Passed:

- Backend module `go test ./...` and Docker `docker compose -p incidentgpt-regression -f deploy/test-compose.yaml run --rm backend-tests sh -c 'go vet ./... && go build -o /tmp/backend . && go test -race ./...'`.
- Focused tests: relevance ranking and version/environment caveats, authenticated actor overriding spoofed headers, stale revision conflict, withdrawal removing authoritative index entries while keeping audit history, foreign evidence rejection, unknown-cause preservation, runbook version/source identity and directory escape rejection.
- UI `npx tsc --noEmit -p tsconfig.app.json`, `npm run build` (existing bundle-size warning).
- `helm lint backend/chart`, umbrella dependency build/lint/render and `git diff --check`.

An accidental repository-root go test invocation failed because Go modules are separate; the correct backend/container commands above passed. Not run: production ConfigMap mount, browser interaction tests, live sanitizer policy/LLM/Telegram evaluation or production rollout. Runbooks use deterministic fake sanitizer tests.

## Current candidate verification — 2026-10-19

UI TypeScript checking and production build passed on the replayed candidate. The existing bundle-size warning remains. Backend and umbrella Helm lint passed; default and runbook-enabled umbrella renders passed. The enabled render includes the configured read-only ConfigMap mount and RUNBOOK_DIRECTORY. `git diff --check` passed.

Backend `go vet ./...`, `go build -o /tmp/incidentgpt-memory-backend .` and `go test -race -v ./...` passed using local Go 1.26.3 (`GOTOOLCHAIN=local`), rather than the CI/production Go 1.24 container. Four Redis integration tests were skipped because no dedicated Redis is available, including resolution correction/withdrawal. The initial sandboxed test attempt failed to bind the local httptest port; its authorized rerun outside the sandbox passed. CI must still exercise the intended toolchain and disposable Redis.

The local Docker daemon is unavailable, including outside the sandbox. No candidate production images or image digests, isolated deployment, ingress/health/migration smoke evidence or production verification are available from this run. Historical Docker checks above do not verify this candidate. Keep this PR open and in draft until CI and isolated deployment verification with the intended runtime configuration pass; record candidate commit/image digests and results in the PR before a squash merge. Preserve production data and take backups before any production migration.

## Rollout and rollback

Deploy compatible backend/UI with reader credentials, persistent Redis and sanitizer HMAC. Optionally mount a reviewed Markdown ConfigMap/directory and set RUNBOOK_DIRECTORY. Verify one confirmed resolution/correction before expanding use. Back up resolution histories and indexes with all referenced evidence. Rollback unsets runbook configuration or removes compatible UI/routes; preserve additive records and feedback. No external notifications, crawling or deployment was performed.

## Limitations and reviewer focus

Search ranks only the latest 200 scoped candidates, exposes that bound and paginates within it. Pattern counts are bounded-window counts. Runbook retrieval caps files/output and reports partial/unavailable sources. It does not prove applicability or execute mitigations. Review confirmation semantics, evidence/actor authorization, atomic correction/withdrawal, honest unknown/version caveats, source provenance, safe links and storage failure behavior.
