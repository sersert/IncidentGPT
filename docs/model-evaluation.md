# Model evaluation lab

`/evaluation` is an opt-in workspace for comparing language models and prompt
versions against immutable test cases. It does not replace an incident’s current
analysis, confirm a resolution, collect new telemetry, execute tools or send Telegram
messages. All new APIs independently require the existing shared administrator
credentials; browser mutations require JSON and the configured trusted Origin.
This is shared-admin access, not per-user RBAC or tenant isolation.

## Configure

Deploy compatible backend, ai-worker and UI images from one verified candidate.
Apply [values-evaluation.yaml](examples/values-evaluation.yaml) over your environment’s
values. Both backend and worker require `EVALUATION_ENABLED=true`; default is false.
Backend needs persistent Redis, reader credentials, sanitizer URL/HMAC and
`ANALYSIS_SERVICE_TOKEN` (at least 32 bytes). Worker uses its existing provider URL,
API key, sanitizer and controlled provider transport. The endpoint is derived from
`STRUCTURED_WORKER_URL`; browsers cannot choose URLs, credentials or arbitrary models.
Never put real provider credentials into a test case, profile JSON or Git.

Worker `EVALUATION_PROFILES_JSON` declares 1–16 profiles, each with a unique `id`,
`label`, `model`, optional `max_tokens` (default 2000, range 64–4000) and
`prompt_version`. Supported prompts are `evidence-analysis/v2` and
`evidence-analysis/alternatives-v1`; the latter explicitly asks for competing
explanations and distinguishing observations. Profiles share the configured
OpenAI-compatible provider. No custom system prompt is accepted from the browser.
A revision hash binds all profile settings; configuration drift between scheduling
and execution fails before inference rather than silently testing another model.
Changing a lab selection never changes the production model. Lab calls remain subject to AI pause/concurrency control but do not overwrite the production model’s health observation.

## Cases and references

Create a case from a selected retained structured analysis, or write a synthetic
scenario. Incident cases copy the exact retained observation records cited by that
version, preserving their collection/observation times, availability and composition.
Missing or foreign records fail visibly. Legacy Markdown without retained evidence
cannot be replayed. Existing observations are not refreshed at capture or execution.

Only observation sources are retained. Previous model answers, confirmed-resolution
retrieval, `confirmed_context`, diagnostic operations and report sources are excluded.
MCP and other live knowledge searches are disabled during evaluation. Consequently
this is a frozen-observation benchmark, not a byte-for-byte replay of an original
request that included prior knowledge or analyses. A SHA-256 snapshot identity is
shown in the UI. Aliased models can change and sampling remains stochastic; identical
inputs do not promise deterministic output or an immutable provider model revision.

An engineer can supply an expected cause separately. If omitted for an incident,
the current confirmed resolution is captured only when its composition matches the
source analysis. It retains resolution revision and actor. Unknown or mismatched
references remain unknown. The case name, expected cause, resolution and review are
never sent to the model. A reference can itself be incorrect: assessments are human
judgment, not calibrated accuracy scores. Corrections create a new case.

Synthetic scenarios are bounded to 8192 UTF-8 bytes, names to 160 bytes, references
to 4000 bytes, and snapshots to 128 KiB/200 observations. Sanitization is required
before persistence and again at the provider boundary. Raw prompt/log dumps are not
written to application logs. The UI renders observations and outputs as text.

## Comparisons and cost

Select 1–4 allowlisted configurations and explicitly confirm provider calls.
Creating/browsing a case or reloading results never calls a model. The server accepts
one comparison batch per minute globally and at most 32 pending jobs; up to 1000
retained runs and 200 cases are accepted within the retention window. A request ID
makes repeated scheduling of the same comparison idempotent; conflicting reuse is
rejected. Enqueue persists the run and every model job atomically before returning 202.

The lab has its own Redis queue and one global 90-second execution lease. Each
configuration gets one provider attempt with bounded input/output and token cap.
There is no automatic paid retry. Worker/provider errors, invalid citations and
profile drift fail independently, retaining other results. Lease loss after a crash
marks an in-flight attempt interrupted; it is never resubmitted automatically.
A provider may bill a failed or interrupted request. Explicitly start a new comparison
if another attempt is desired. Provider redirects and tool calls are rejected.

Managed AI pause blocks queued jobs without starting provider requests. Existing
worker control guards also apply at the actual provider boundary. Resuming allows
queued jobs to proceed; a request that already failed/canceled remains failed and
needs an explicit new run. Lab calls consume the worker’s normal concurrency budget.

Compare hypotheses, citations, missing information, suggested checks, measured
analysis duration and provider-reported usage. Missing token/cost values stay
unknown, not zero. Only bounded nonnegative numeric usage fields are retained;
sanitizer-redacted strings cannot become usage metadata. `usage.cost` has the unit
and billing semantics of the configured provider; do not assume a currency or treat
it as a final invoice. Reviews use `unrated`, `accurate`, `partial`, `incorrect` plus
an optional note. CAS revisions prevent silent overwrite and retain actor/time and
review history. Reviews do not modify the live incident or confirmed resolution.

## APIs, storage and rollout

All browser paths start at `/api/v1/evaluation`:

- `GET /config`: enablement and allowlisted profiles; retained data remains readable
  when profile discovery is unavailable.
- `GET/POST /cases`, `GET /cases/{case}`: bounded library and frozen case creation.
- `GET/POST /cases/{case}/runs`: latest 25 comparisons and atomic explicit scheduling.
- `GET /runs/{run}`: the captured case/reference and independent model results.
- `PUT /runs/{run}/results/{profile}/review`: revisioned human assessment.

Internal worker `GET /v1/evaluation/profiles` and
`POST /v1/evaluation/analyze` require a service token and reject browser Origin.
Do not expose worker routes on public ingress. Existing private backend/worker
NetworkPolicy and sanitizer boundaries apply; no new Kubernetes RBAC is needed.

Keys use `igpt:evaluation:*`. Cases, runs, jobs and human review history have 90-day
retention. Run records contain their own case/reference snapshot, so later changes
to an incident or case expiry cannot rewrite the comparison. Indices are bounded by
retention; global lease/rate-limit keys are short-lived. Back up these keys with the
existing Redis data. This requires a single primary (multi-key Lua), not Redis Cluster.
There is no destructive migration or backfill. Before rollback, stop new launches and drain the queue, or disable backend evaluation to retain queued work for a compatible consumer. Disable/revert the worker only after that: leaving the backend consumer enabled with worker evaluation disabled records failures rather than silently retrying. Old versions ignore the additive keys. Retained lab data is independent of live incident history.

## Verification

`go test -race ./...` in backend with a dedicated
`INCIDENTGPT_TEST_REDIS_ADDR` exercises snapshot isolation, answer exclusion,
idempotency, atomic multi-model scheduling, global leases, fencing, interrupted
attempts, independent results, review CAS, auth/Origin and sanitizer failure. Worker
tests cover profile overrides, excluded answer context, disabled MCP/Telegram,
unknown citations, provider redirects and sanitizer failure. UI browser tests cover
case creation, explicit paid-call confirmation, multiple results, missing usage,
review, errors and mobile layout.

For an isolated deployment with real nginx, backend, worker, sanitizer and Redis:

```sh
docker compose -p incidentgpt-evaluation-preview -f deploy/evaluation-preview.compose.yaml up -d --build
python3 deploy/check-evaluation-preview.py
docker compose -p incidentgpt-evaluation-preview -f deploy/evaluation-preview.compose.yaml restart backend worker
python3 deploy/check-evaluation-preview.py --verify-proof /tmp/incidentgpt-evaluation-proof.json
```

All credentials, models and data in this Compose file are disposable fixtures. The
provider is deterministic and never forwards requests externally. Backend/worker
are on an internal network; a separate bridge exposes only loopback UI/test endpoints.
The smoke test verifies pause/resume, immutable data, 2 successful models and one
invalid-citation failure, exactly 3 explicit fixture calls, zero Telegram sends,
review conflicts and persistence after restart. It does not prove compatibility with
a real paid provider. Remove only this Compose project’s resources after verification.
