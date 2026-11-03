# IncidentGPT — карта фактов для ревью (снято с `main` 2026-10-30)
Дополняет `CLAUDE.md`, не повторяет его. При расхождении верить коду. Обновлять в PR, меняющих маршруты, ключи Redis, флаги, секреты, NetworkPolicy.

## 1. HTTP-маршруты и кто их закрывает

**enricher** `:9099`, `enricher/main.go:480-487`: `GET /healthz /readyz /status /metrics` — без auth. `POST /alert` (`alertHandler` main.go:516) — без auth, защита только NetworkPolicy (ns `monitoring`).

**sanitizer** `:8080`, `sanitizer/internal/api/server.go:34-40`: `/healthz /readyz /version /metrics` — без auth. `POST /v1/sanitize`, `/v1/sanitize/text`, `/v1/inspect` — HMAC через `readSignedRequest` (server.go:162) → `auth.Verifier.Verify` (`internal/auth/hmac.go:25`): заголовки `X-IncidentGPT-Timestamp/-Request-ID/-Signature`, секрет `SANITIZER_AUTH_SHARED_SECRET`, skew `SANITIZER_AUTH_MAX_CLOCK_SKEW` (60s), replay-кэш по Request-ID. Пустой секрет → всегда 401 (fail-closed).

**ai-worker** `:8080`, `ai-worker/main.go:177-184`: `/healthz /readyz /status /metrics` — без auth. `POST /incident`, `/incident-raw`, `/incident-group` — без auth (только NP). `POST /v1/analyze` (`structured.go:94`) — `Bearer ANALYSIS_SERVICE_TOKEN` (`constantToken`), пустой токен → 401.

**backend** `:8080`, роутер `backend/main.go:1802-1866`, всё через `corsMiddleware` (main.go:754). Собственной сессии нет — расчёт на basic-auth nginx в UI.

| Маршрут | Guard | Файл |
|---|---|---|
| `POST /api/v1/ingest`, `POST /api/enriched` | без auth при `DURABLE_ANALYSIS_ENABLED=false`; при `true` → `handleDurableIngest`: `trustedBearer(ANALYSIS_SERVICE_TOKEN)` | main.go:831, jobs.go:345,422 |
| `POST /api/v1/incidents/{id}/analysis?group_key=` (callback ai-worker) | без auth; при durable → 409 `rejectDurableCallback` | main.go:1097, jobs.go:467 |
| `POST /api/v1/incidents/{id}/feedback`, `PATCH /api/v1/incidents/{id}` | без auth; actor = `X-Auth-User` или BasicAuth user (`requestUser` main.go:1079) | main.go:1294, 1011 |
| `GET /api/v1/incidents`, `/incidents/{id}`, `/incidents/{id}/logs`, `/alerts*`, `/dashboard/*`, `/analytics/*`, `/predictions*`, `/license`, `/services/*`, `GET/PUT /settings/llm`, `POST /settings/llm/test`, `GET/PUT/POST /settings/integrations/*` | без auth | main.go, logs.go:52 |
| `GET /incidents/{id}/changes`, `/incidents/{id}/analyses`, `/incidents/{id}/evidence/{e}`, `/jobs/metrics`, `GET/PUT /incidents/{id}/resolution`, `/knowledge/search`, `/knowledge/patterns` | `changeReadAuth`: HTTP Basic `TIMELINE_READ_USER`/`TIMELINE_READ_PASSWORD`, любая пустая → 401 | changes.go:104; jobs.go:248,315,426; knowledge.go:76,232,385 |
| `POST /api/v1/changes`, `/changes/coverage` | `changeServiceAuth`: `CHANGES_ENABLED=true` (иначе 503) + `Bearer CHANGE_INGEST_TOKEN` (пустой → 401) + Redis | changes.go:84 |
| `GET /healthz` | без auth | main.go:1784 |

**ui** nginx `:80`, `ui/nginx.conf`: `auth_basic` на всё кроме `/healthz`; `/api/` → `incidentgpt-backend.incidentgpt.svc.cluster.local:8080` с `X-Auth-User=$remote_user`; `/ws/` проксируется туда же, но в backend маршрута `/ws` нет; resolver `10.96.0.10` зашит.

## 2. Ключи Redis

| Ключ | Тип / TTL | Пишет → читает | Файл |
|---|---|---|---|
| `grp:{groupKey}` | HASH fp→alert, TTL `CORR_WINDOW` (10m) | enricher `bufferAlert` → `flushGroup` после `CORR_SETTLE` | enricher/correlation.go:74-105,159 |
| `raw:{fingerprint}` | SetNX, TTL `RAW_DEDUP_TTL` (2m) | enricher `claimRawDelivery` | correlation.go:136, internal/rawdedup |
| `igpt:corr:pending:{gk}` HASH (≤50), `igpt:corr:due` ZSET | без TTL | enricher `bufferDurable` (только `DURABLE_CORRELATION_ENABLED`) | durable_correlation.go:19-40 |
| `igpt:corr:batch:{id}` HASH, `igpt:corr:meta:{id}` HASH(group_key,attempts,owner,status), `igpt:corr:batches` ZSET, `igpt:corr:dead` ZSET | без TTL; lease 45s, attempts>8 → dead_letter, backoff 2^min(n,6)s | enricher `recoverCorrelation` (тик 1s) | durable_correlation.go:53-112 |
| `igpt:incident:{id}` JSON, `igpt:incidents` ZSET(created_at), `igpt:fp:{fp}`→incident id, `igpt:seq:incident` | без TTL | backend `Storage` | backend/storage.go:24-32 |
| `igpt:alert:{id}`, `igpt:alerts` ZSET, `igpt:seq:alert`, `igpt:alerts:total`, `igpt:alerts:today:{YYYY-MM-DD}` | today — TTL 48h, остальное без | backend `Storage` | storage.go:125-205 |
| `igpt:job:{jid}` JSON-envelope, `igpt:jobs:due` ZSET(score=due→lease_until), `igpt:jobs:pending:created` ZSET, `igpt:jobs:incident:{inc}` ZSET | без TTL | backend enqueue/claim/finish | jobs.go:68-186 |
| `igpt:analysis:{aid}`, `igpt:analysis:latest:{inc}`, `igpt:analysis:history:{inc}` ZSET, `igpt:analysis:requested:{inc}`, `igpt:evidence:{eid}` (SET NX) | без TTL | backend finishJob → `handleAnalysisVersions`, `handleEvidence`, knowledge | jobs.go:26-27,149-164; knowledge.go:64 |
| `igpt:ingest:batch:{digest}`→job id, `igpt:ingest:lock:{digest}` | lock SetNX 30s | backend `handleDurableIngest` | jobs.go:391-401 |
| `igpt:jobs:{retries,failures,completed,latency:sum,latency:samples}` | счётчики | claim/finish → `handleJobMetrics` | jobs.go:131-157,449 |
| `igpt:change:{id}`, `igpt:changes:index:{digest(cluster,ns,workload)}` ZSET, `igpt:change:coverage:{digest(cluster,ns)}` | retention `CHANGE_RETENTION_DAYS` (30) | backend `handleChangeIngest/Coverage` → `handleIncidentChanges` | changes.go:73,218,290,377 |
| `igpt:resolution:{inc}`, `igpt:resolution:history:{inc}:{rev}`, `igpt:knowledge:index:{digest(cluster,service)}` ZSET (+ с service="") | без TTL | backend `handleResolution` (PUT) → `knowledgeSearch/Patterns` | knowledge.go:47,91,171 |
| `igpt:runbook:version:{id}:{ver}` | SetNX 30d | backend `readRunbooks` | knowledge.go:373 |

ai-worker Redis не использует. `settings/llm` и `settings/integrations` живут только в памяти backend (integrations.go), в Redis не пишутся. `finishJobScript` дополнительно перезаписывает `igpt:incident:{id}.analysis_summary` (jobs.go:160-161).

## 3. Флаги и env (код ↔ values чарта)

| Env | Дефолт в коде | values сабчарта / umbrella | Файл |
|---|---|---|---|
| `DURABLE_CORRELATION_ENABLED` | false | `changes.durableCorrelation: false` | enricher/durable_correlation.go:17 |
| `CHANGE_COLLECTOR_ENABLED` | false | `changes.enabled: false` | enricher/changes.go:206 |
| `CHANGES_ENABLED` | "false" | `"false"` | backend/changes.go:83 |
| `DURABLE_ANALYSIS_ENABLED` | "false" | `"false"` | backend/jobs.go:25 |
| `RUNBOOK_DIRECTORY` | "" (выкл) | `""`; `runbooks.existingConfigMap` монтируется в `/etc/incidentgpt/runbooks`, env чарт сам не выставляет | knowledge.go:313; backend deployment.yaml:67-72 |
| `SANITIZER_FAIL_CLOSED` | true | `"true"` | sanitizer/internal/config/config.go:48 |
| `FORWARD_TO_AI_WORKER` / `AI_WORKER_URL` | "true" / `ai-worker…:8080/incident` | "true" / `…/incident-group` (**расходится**) | backend/main.go:23-24 |
| `STRUCTURED_WORKER_URL` | `ai-worker.incidentgpt.svc:8080/v1/analyze` | то же | jobs.go:192 |
| `GROUP_BACKEND_URL` | `ai-worker…/incident-group` | сабчарт: то же; umbrella: `incidentgpt-backend:8080/api/v1/ingest` | enricher/main.go:386; deploy/incidentgpt/values.yaml |
| `PYTHON_BACKEND_URL` (`env.backendUrl`) | `incidentgpt-backend…:8000/api/enriched` | `ai-worker…:8080/incident` (**расходится**) | enricher/main.go:370 |
| `RAW_BACKEND_URL` | `ai-worker…/incident-raw` | то же | enricher/main.go:387 |
| `SANITIZER_URL` | enricher/ai-worker: `incidentgpt-sanitizer.incidentgpt.svc:8080`; backend: `""` | везде `…svc:8080` | enricher/main.go:378; backend/changes.go:244 |
| `REDIS_ADDR` | enricher: `redis.incidentgpt.svc.cluster.local:6379`; backend: `""` = память | `incidentgpt-redis:6379` | enricher/main.go:384; backend/main.go:245 |
| `CORR_WINDOW` / `CORR_SETTLE` / `RAW_DEDUP_TTL` | 10m / 20s / 2m | 10m / **40s** / 2m | enricher/main.go:388-391 |
| `ANALYSIS_CALLBACK_URL` | "" (callback не шлётся) | umbrella: `incidentgpt-backend:8080/api/v1/incidents/by-group/analysis` | ai-worker/analysis_callback.go:23 |
| `OPENROUTER_MAX_TOKENS` / `STRUCTURED_OUTPUT_MODE` | 600 / "" | **2000** / `"compatibility"` | ai-worker/main.go:158, structured.go:33 |
| `CHANGE_RETENTION_DAYS` / `CHANGE_LOOKBACK` / `CHANGE_POLL_SECONDS` | 30 / 2h / 60 | 30 / 2h / 60 | backend/changes.go:77,393; enricher/changes.go:222 |
| `PROMETHEUS_URL` | enricher `kube-prometheus-stack-prometheus.monitoring:9090`; backend `kps-kube-prometheus-stack-…` | enricher: как в коде | enricher/main.go:369; backend/integrations.go:83 |

Сопряжение флагов: enricher ставит `Authorization: Bearer ANALYSIS_SERVICE_TOKEN` и `X-IncidentGPT-Batch-ID` только в durable-пути (`sendDurableGroup` с batchID≠"", correlation.go:270-273, вызов durable_correlation.go:106); обычный `flushGroup` шлёт без auth → при `DURABLE_ANALYSIS_ENABLED=true` на backend получает 401.

## 4. Секреты (existingSecret → ключ → env)

| Чарт | values | ключ Secret (дефолт) → env | Шаблон |
|---|---|---|---|
| enricher | `secrets.existingSecret` (иначе Secret `<fullname>-secrets` из `secretValues`) | `auth-shared-secret` → `SANITIZER_AUTH_SHARED_SECRET` | enricher/chart/…/templates/deployment.yaml:119-123, secret.yaml |
| enricher | `changes.existingSecret` (без него env не создаются) | `change-ingest-token` → `CHANGE_INGEST_TOKEN`; `analysis-service-token` → `ANALYSIS_SERVICE_TOKEN` | deployment.yaml:35-59 |
| enricher | `logsStore.existingSecret` (только при `env.logsStoreUrl`) | `password` → `LOGS_STORE_PASSWORD` | deployment.yaml:81-87 |
| enricher | — | `REDIS_PASSWORD` берётся plain из `env.redisPassword`, не из Secret | deployment.yaml:109 |
| sanitizer | `secrets.existingSecret` (иначе `<fullname>-secrets`) | `hash-key` → `SANITIZER_HASH_KEY`; `auth-shared-secret` → `SANITIZER_AUTH_SHARED_SECRET` | sanitizer/chart/templates/deployment.yaml:45-55 |
| ai-worker | `secrets.existingSecret` (иначе `ai-worker-secrets`; свой secret.yaml **не** пишет `analysis-service-token`) | `OPENROUTER_API_KEY`, `TELEGRAM_BOT_TOKEN`, `analysis-service-token` → `ANALYSIS_SERVICE_TOKEN`, `auth-shared-secret` → `SANITIZER_AUTH_SHARED_SECRET`; одноимённые ключи из `.env` отфильтрованы | ai-worker/chart/templates/deployment.yaml:38-64 |
| backend | `changes.existingSecret` — один Secret на всё; без него ни один из 4 env не задан | `analysis-service-token`, `change-ingest-token`, `timeline-read-password` → `TIMELINE_READ_PASSWORD`, `auth-shared-secret` → `SANITIZER_AUTH_SHARED_SECRET` | backend/chart/templates/deployment.yaml:44-66 |
| backend | `logsStore.existingSecret` | `password` → `LOGS_STORE_PASSWORD`; `REDIS_PASSWORD` чартом не прокидывается | deployment.yaml:35-43 |
| ui | `auth.existingSecret` (ключ `htpasswd`), иначе Secret `<fullname>-auth` из `auth.username/password` (admin/admin); создаётся при `auth.enabled` даже с existingSecret | ui/chart/templates/secret.yaml, deployment.yaml:57-62 |
| redis | `auth.existingSecret` / `auth.password`, ключ `auth.passwordKey=password` → `--requirepass` | deploy/redis/templates/statefulset.yaml:36-46 |

`ANALYSIS_SERVICE_TOKEN` и `SANITIZER_AUTH_SHARED_SECRET` — общие, должны совпадать в Secret'ах enricher, backend, ai-worker (и sanitizer).

## 5. Сеть и RBAC

| Поток | Разрешает (шаблон) |
|---|---|
| Alertmanager (ns `monitoring`) → enricher:9099 | enricher NP: ingress только из ns `monitoring` (`enricher/chart/…/templates/networkpolicy.yaml`, без `enabled`-условия) |
| enricher / ai-worker / backend → sanitizer:8080 | sanitizer NP: `component=enricher`, `app.kubernetes.io/name=ai-worker`, `app=<networkPolicy.backendAppLabel>` (incidentgpt-backend) + ns monitoring |
| enricher → ai-worker:8080 (`/incident-raw`, `/incident-group`, `/incident`) | ai-worker NP: `component=enricher` + ns monitoring (без `enabled`-условия) |
| backend → ai-worker:8080 (`/incident-group`, `/v1/analyze`) | backend NP `-to-ai-worker` (`backend/chart/templates/networkpolicy.yaml`); селектор ai-worker по `networkPolicy.aiWorker.instance=incidentgpt` — должен совпадать с именем релиза |
| ui, enricher (`/api/v1/ingest`, `/api/v1/changes*`), ai-worker (callback) → backend:8080 | backend NP `-from-ui`: `app=<uiAppLabel>`, `component=enricher`, `name=ai-worker` |
| enricher, backend → redis:6379 | redis NP: `app=<backendAppLabel>`, `component=enricher` (`deploy/redis/templates/networkpolicy.yaml`) |
| ingress → ui:80 | ui NP `-from-ingress`: источник не ограничен; `networkPolicy.ingressNodeCIDRs` в шаблоне не используется |
| Egress (Prometheus, K8s API, логи, OpenRouter, Telegram) | нигде не ограничен; все NP — только `Ingress` |

RBAC: только enricher — ClusterRole `<fullname>-read` (`enricher/chart/…/templates/rbac.yaml`): core `pods, namespaces, nodes, events` get/list/watch; apps `deployments, replicasets` get/list; ClusterRoleBinding на SA enricher. Код делает List/Get Pods, Events, Deployments, ReplicaSets (k8s.go, changes.go), Watch не используется. sanitizer — свой SA без ролей; backend, ai-worker, ui — SA default, RBAC нет.

## 6. Очередь job в backend (`backend/jobs.go`)

- Старт `startAnalysisJobs` (:222): только при `DURABLE_ANALYSIS_ENABLED=true` и Redis; 4 горутины, тик 1s, ctx 60s на итерацию.
- `enqueueAnalysisScript` (:68): дедуп по `IngestKey` (возврат -2 → отдать прежний job id), job уже есть → 0, инцидента нет → -1; записывает `analysis_request_id` в инцидент (cjson decode/encode).
- `claimJobScript` (:124) / `claimJob` (:135): `ZRANGEBYSCORE due ≤ now`, lease = now+90s, owner = `newStableID("owner")`, статус `running`, `attempts++` (>1 → `igpt:jobs:retries`); `attempts ≥ 4` до claim → `failed`, `error_code=retry_budget_exhausted`.
- `executeAnalysis` (:190): `POST STRUCTURED_WORKER_URL` с `Bearer ANALYSIS_SERVICE_TOKEN`, HTTP-таймаут 55s, лимит ответа 256 KiB, `validateAnalysis`.
- `finishJobScript` (:149) / `finishJob` (:166): fencing по owner + lease; статусы `queued` (повтор через 2^attempts s) / `complete` / `failed`; при complete — `igpt:analysis:*`, history, latest, `analysis_summary` в инциденте.
- Жизненный цикл: `queued → running → complete | failed | queued`; `canceled` учитывается в claim, но в коде не выставляется.
- `handleDurableIngest` (:345): лок `igpt:ingest:lock` 30s, batch id из `X-IncidentGPT-Batch-ID` или digest(body), 1–50 алертов, один кластер, sanitizer fail-closed (503), ответ 202 `{incident_id, job_id}`.

## 7. Тесты, CI, версии

- Go-тесты рядом с кодом: ai-worker (`main`, `sanitizer_flow`, `structured`), backend (`changes`, `integrations`, `jobs`, `knowledge`, `logs`, `main`), enricher (`changes`, `durable_correlation`, `tests/rawdedup`), sanitizer (`internal/api/server`, `internal/masking`). Redis-тесты скипаются без `INCIDENTGPT_TEST_REDIS_ADDR` (backend/jobs_test.go:31, knowledge_test.go:26, changes_test.go:64; enricher/durable_correlation_test.go:14).
- `.github/workflows/ci.yml`: Go-матрица в контейнере `golang:X-bookworm` + service `redis:7.2.7`, шаги `gofmt` / `go vet` / `go build` / `go test -race`; ui — `tsc --noEmit` + `vite build` (тестов нет); hadolint; docker build без push; `helm lint` всех чартов + `helm template` umbrella + kubeconform (k8s 1.28.0).
- `release.yml`: по тегу `v*` → `ghcr.io/sersert/incidentgpt-<svc>` (semver, `latest` на тегах, `edge` при dispatch). `pages.yml` — `website/` на GitHub Pages. `claude-review.yml` — ревью по комментарию `@claude`. `deploy/test-compose.yaml` — локальный прогон тестов с Redis.

| Сервис | go.mod | Dockerfile | CI image |
|---|---|---|---|
| enricher | 1.24 | `golang:1.24-bookworm` → distroless | 1.24-bookworm |
| sanitizer | 1.24 | `golang:1.24-bookworm` → distroless/static | 1.24-bookworm |
| ai-worker | 1.24 | `ARG GO_VERSION=1.25` bookworm → distroless | 1.25-bookworm |
| backend | 1.24 | `golang:1.24-alpine` → scratch | 1.24-bookworm |
| ui | — | `node:20-alpine` → `nginx:alpine` | node 20 |

## 8. Открытые PR (все base `main`, стек: каждый содержит предыдущие)

- **#19** `feat/guided-incident-diagnostics-upstream` — включает коммит #18 (в main уже как squash 2466ed0). Новое: `backend/diagnostics.go`, `enricher/diagnostics.go`, `investigation_contracts.go` в 3 сервисах, enricher chart (deployment, networkpolicy, values), backend chart, `ui/src/components/GuidedDiagnostics.tsx`, `docs/roadmap/guided-diagnostics.md`.
- **#20** `feat/dependency-aware-correlation-upstream` — включает #19. Новое: `backend/graph.go`, `graph_contracts.go`, `regroup.go`; `enricher/dependencies.go`, `graph_contracts.go`, правки `correlation.go`, `durable_correlation.go`, `k8s.go`; `docs/fixtures/service-dependencies.json`; `ui/src/components/DependencyContext.tsx`.
- **#21** `feat/postmortem-and-handoff-upstream` — включает #20. Новое: `backend/lifecycle.go`, `lifecycle_metrics.go`, `reports.go`, правки `storage.go`, `changes.go`, `testdata/`; `deploy/incidentgpt/templates/backend-reader-secret.yaml` + umbrella values; `ui/nginx.conf`, `vite.config.ts`, `IncidentLifecycle.tsx`, `IncidentReports.tsx`; docs architecture/security/web-ui.
- **#22** `feat/operator-ui-and-guide` — включает #21. Новое: редизайн ui (`App.tsx`, `Layout`, `Sidebar`, `ThemeProvider/ThemeToggle`, `pages/Help.tsx`, `index.css`, `ui/README.md`), `docs/getting-started.md`, `docs/examples/values-preview.yaml`, `docs/images/ui/*`, `docs/ui-design-review.md`.

## 9. Чего в репо НЕТ (не искать)

- Миграций схемы Redis, `values.schema.json`, `Makefile`, `docker-compose.yml` в корне, `.golangci.yml`, e2e/интеграционных тестов на весь путь, тестов UI (только typecheck+build).
- WebSocket-сервера в backend (nginx `/ws/` ведёт в никуда); персистентности для `settings/llm` и `settings/integrations`; egress-NetworkPolicy; RBAC у backend/ai-worker/sanitizer/ui; шаблонов в umbrella кроме `grafana-dashboard.yaml`; `.claude/` кроме этого файла.
- `CHANGELOG.md` и `RELEASE_NOTES.md` есть (0.2.0, `Unreleased` пуст). `website/` — отдельный сайт (vitest), в чарт не входит. `backend/incidentgpt-backend` — собранный бинарник, лежит в git.
