# Changelog

Все значимые изменения проекта документируются в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
проект следует [семантическому версионированию](https://semver.org/lang/ru/).

## [Unreleased]

- Вторая учётная запись «только просмотр»: `incidentgpt-backend.viewerAuth.existingSecret`
  (`VIEWER_USER`/`VIEWER_PASSWORD`). Viewer читает всё, мутации `/api/` получают 403
  в backend по cookie и по Basic; ответ `auth/session` получает additive поле `role`,
  Redis-запись сессии — `role`. Без Secret поведение прежнее.
- Opt-in model evaluation lab (`/evaluation`): immutable incident snapshots and
  engineer-authored cases, explicit comparison of 1–4 allowlisted model/prompt
  profiles, provider-reported usage and revisioned human assessments. Expected
  causes stay outside inference. Separate bounded Redis queue, no Telegram or
  live-analysis updates, no automatic paid retries, AI pause support and 90-day
  retention. Configuration and verified preview: `docs/model-evaluation.md`.
- Operations в umbrella включаются одним `global.operations.enabled`; частичная
  конфигурация отклоняется. MCP receiver policy требует отдельного opt-in и не
  создаётся только из-за mcpHealthURL. Контроль provider URL нормализован; безопасный
  сброс в paused позволяет восстановиться после потери Redis-ключа без рестарта.
  Worker 423 не расходует attempts при любой конфигурации backend. Снимок мониторинга
  собирается независимо от браузерного контекста, параллельно и с общим бюджетом 4s.

- Opt-in `/operations`: реальные health checks важных компонентов, свежесть
  последнего inference, явный ограниченный платный probe; автообновление без LLM.
- Сохранённая в Redis пауза AI: общий guard всех provider POST, отмена текущих
  контекстов, fail-closed при отказе control. Raw ingestion/Telegram и report jobs
  независимы; durable-анализы откладываются без расхода retry budget. Новые ключи
  `igpt:operations:ai-control` (JSON v1 без TTL), `:audit` (stream MAXLEN ~1000),
  `igpt:operations:model-probe:limit` (TTL 60s). Совместимость/rollback и Secret
  opt-in описаны в `docs/operations.md`; первоначальное состояние — пауза.
- Добавлен отдельный MCP-сервер для read-only поиска по разрешённым Confluence spaces и каталогам GitHub/GitLab. При явной настройке AI worker сам вызывает поиск по необходимости (не более двух обращений), сохраняя default-поведение без MCP. Запросы и документы проходят fail-closed санитизацию. Sanitizer сохраняет непрозрачные 40-символьные Git SHA, продолжая маскировать credentials и телефоны. Версии structured-анализа получают additive metadata `knowledge` и `knowledge_reference_ids`; старые записи совместимы, новых Redis keys и миграций нет. Подробнее: [mcp-knowledge.md](docs/mcp-knowledge.md).

- Сохранён Basic/htpasswd по умолчанию. Страница входа — opt-in `authMode=session` +
  `browserSessions.enabled=true`; umbrella проверяет reader Secret и совместимую
  конфигурацию. Обновляйте UI/backend вместе; session UI со старым backend даёт
  ошибку auth-check. Для rollback верните basic и сохранённый htpasswd Secret.
- Новые маршруты: публичный `/auth-config.json` (только режим), backend
  `GET/POST/DELETE /api/v1/auth/session`, `GET /api/v1/auth/check` (nginx subrequest),
  включены только при `BROWSER_SESSIONS_ENABLED=true`. В basic контракт прежний;
  в session Basic API-клиенты используют `TIMELINE_READ_*`, X-Auth-User удаляется,
  а auth/audit получают principal без переноса пароля в заголовки.
- Новый Redis-контракт `igpt:session:<SHA256(cookie)>`, JSON `credential` (HMAC от
  случайного cookie с ключом user\0password), `expires`, TTL 8h. Старые preview
  `browser-session:*` не читаются и истекают сами; инциденты не мигрируют. Клиент
  сессий повторяет соединение независимо от durable и не заменяет настроенный
  Redis памятью. `trustedProxyCIDRs`/`realIpHeader` задают доверие к ingress IP,
  `loginBurst` — burst POST-входов; по умолчанию forwarded IP не доверяем.

- Исправлена публикация postmortem: additive ключ `igpt:incident:postmortem:{id}`
  (без TTL, удалять вместе с incident) исключает порчу JSON через Lua cjson;
  reader поддерживает старый флаг и известные пустые массивы, записанные как `{}`.
- Lifecycle пишет только изменения при opt-in durable/reports/`LIFECYCLE_ENABLED`.
  `LIFECYCLE_HISTORY_RETENTION_DAYS` задаёт TTL новых audit-ревизий (0 — политика
  оператора), текущие записи и прежняя история автоматически не удаляются.
- Старые PATCH и dashboard/analytics сохраняют доступ через доверенный UI-прокси;
  `LEGACY_API_AUTH_REQUIRED=true` явно включает fail-closed проверку backend Basic.
  Новые API всегда требуют credentials. Убрано автоматическое копирование admin/admin;
  reader Secret обязателен при его выборе. Прокси с заменой Host поддержан через
  явный `ALLOWED_ORIGINS`, без доверия к произвольным forwarded-заголовкам.
- Dashboard/analytics восстановлены в memory fallback (`persistent:false`), без
  выдуманных времён восстановления/анализа; устранено повторное чтение списка.
- Исправлен resolve общей пачки: обновляются все затронутые инциденты, закрытие
  возможно только без оставшихся firing-участников. Повторный эпизод после закрытия
  и восстановление orphan не создают пустые инциденты. Диагностика после regroup
  использует один согласованный namespace/workload/pod/node; labels копируются.
- Корреляция читает bounded declared-документ, а не общий outgoing-индекс: поток
  observed ownership/placement не вытесняет зависимости. Файловый граф блокирует
  PUT (409), логирует ошибки загрузки; ConfigMap автоматически задаёт путь и ключ.
- `SERVICE_GRAPH_ENABLED` — opt-in графа; без новых флагов GET graph возвращает
  `enabled:false` без Basic challenge, detail — 404, карточка/контролы UI скрыты.
  При opt-in чтение графа требует `TIMELINE_READ_*`: это общая administrator-роль,
  не read-only. Мутации graph/regroup требуют JSON и доверенный Origin; прокси с
  другим Host поддержан явным `ALLOWED_ORIGINS`. Actor — проверенный Basic user.
- `igpt:alertfp:{digest(fp)}` (HASH) и `igpt:alertfp:ready:v1` — additive,
  восстанавливаемый индекс активного алерта; при opt-in достраивается перед трафиком
  вне composition lock, включая restart после rollback старых writers. Legacy без
  opt-in новые ключи не пишет. TTL как у alert, удалять при архивировании alerts.
- Ручные `igpt:regroup:binding:{fp}` имеют TTL 7 дней и снимаются при закрытом target;
  прежние ключи получают TTL при чтении, при выключенном regroup игнорируются.
  Audit сохраняется отдельно. Пачка ставит один анализ на каждого фактического
  владельца алертов без чужих evidence и дублей при схождении partitions.
- `igpt:corr:fpgroup:{digest(cluster,fp,starts_at)}` живёт в течение окна корреляции:
  resolve очищает правильный pending даже при изменившемся/недоступном графе.
  Наблюдения topology отправляются асинхронно, ограничены четырьмя слотами,
  ошибки/переполнение видны в status и логах; provenance не растёт на каждый повтор.
- startupProbe enricher включён только для durable. `redisStartup.timeoutSeconds`
  задаёт согласованные runtime timeout и probe budget. Для durable/observations
  ключ `analysis-service-token` в existingSecret обязателен: подготовить до upgrade.

- Dependency graph API и additive Redis-ключи `igpt:graph:*`: идентичности
  cluster/namespace/kind/name, provenance и TTL 30 дней. Declared edges заменяются
  сразу; metadata не создаёт application dependencies. Новые флаги enricher
  `topology.observationsEnabled` / `topology.correlationEnabled` выключены по умолчанию.
- Ручной `POST /incidents/{id}/regroup` требует opt-in `MANUAL_REGROUP_ENABLED=true`
  и durable Redis. История/audit/bindings `igpt:regroup:*` сохраняются без TTL;
  analysis history получает composition/hash и source links. Без флага ingest
  сохраняет прежний lock по группе; с флагом использует общий fenced composition lock.
  Перед переключением флага дренировать ingest, перед rollback сохранить Redis backup.


### Added
- Управляемые проверки чтения по инциденту: `POST/GET
  /api/v1/incidents/{id}/investigations` и `POST
  /api/v1/incidents/{id}/investigations/{job}/cancel`. Действия `refresh`,
  `hypothesis`, `compare` ограничены ресурсами инцидента; вопрос — до 500
  Unicode-символов. Доступ использует существующую единую admin-учётку
  `TIMELINE_READ_*`, включая запуск и отмену; отдельной роли reader пока нет.
- Внутренний `POST /v1/diagnostics` у enricher: фиксированные проверки
  Kubernetes, метрик и логов; обязательные сервисный токен и список namespace.
  По умолчанию выключен вместе с `GUIDED_DIAGNOSTICS_ENABLED` у backend.
- GET истории анализа дополнен `operations` и `diagnostics_enabled`.
  Вход анализа дополнен необязательными `job_kind` и `operation`; очередь
  использует `job.kind=diagnostics` и ключ `igpt:diagnostics:active:{id}`
  для одной активной проверки на инцидент. Собранные evidence сохраняются
  перед анализом и повторно используются при отказе провайдера.

### Fixed
- Повторная пачка алертов не дублирует fingerprint из другого активного
  инцидента и не перезаписывает его binding. Новый эпизод создаётся только
  после закрытия прежнего инцидента; received_at обновляется при повторе.
- Enricher в durable correlation ожидает Redis перед запуском HTTP/очереди,
  закрывает неудачные клиенты и использует startupProbe; legacy fallback сохранён.
- После ручного regroup текущие namespace/labels/context пересчитываются по
  оставшимся алертам; удалённый ресурс не используется при следующем refresh.
  Старые enrichment/chain pointers очищаются, immutable history сохраняется.
- Телефонный детектор sanitizer не маскирует цифровые фрагменты внутри
  смешанных hex-ID длиной 32/64 символа: они связывают observations и citations.
  Телефоны рядом с ID, числовые строки и секреты в чувствительных полях
  продолжают маскироваться; custom/credential правила не обходятся.
- В durable-режиме backend ожидает Redis до приёма трафика и запуска очереди,
  поэтому свежая установка не зависит от порядка старта подов. `/readyz`
  проверяет доступность Redis; Helm использует его для readiness. Legacy-режим
  сохраняет прежний fallback в память. Без `REDIS_ADDR` durable-запуск отклоняется.
- Пустые типизированные массивы входа очереди сохраняются как `null`, чтобы
  Lua cjson не превращал их в `{}` при claim, retry, cancel и finish.
  Существующие корректные записи сохраняют формат; backfill не требуется.
- Отказ обработки по таймауту записывает повторную попытку отдельным коротким
  контекстом, сохраняя проверки владельца, lease и отмены.
- UI блокирует запуск выключенных проверок, сохраняя историю и отмену.
  Helm отклоняет включение без обязательных секретов/namespace; адрес
  collector по умолчанию разрешается в namespace релиза.

Порядок обновления: сначала совместимые ai-worker и enricher, затем backend
и UI; флаги включать только после настройки общих analysis/sanitizer секретов,
admin-доступа и явного списка namespace. Старый ai-worker игнорирует новые
JSON-поля, но не предоставляет всю семантику диагностических операций.
Перед откатом выключить новые запросы, отменить или дождаться завершения
очереди и сохранить evidence/историю и резервную копию Redis.

## [0.2.0] - 2026-08-09

Веб-интерфейс, логи в обогащении и разбор, который не отправляет инженера за данными.

### Added
- **Backend и веб-интерфейс** — два новых компонента в общем чарте (`enabled: false`,
  если не нужны). Инциденты видны в браузере, а не только в Telegram:
  состав группы, разбор, логи, контекст, журнал действий. См. [docs/web-ui.md](docs/web-ui.md)
- **Алерт и инцидент разделены**: алерт — единица сырого потока, инцидент — группа
  связанных алертов с одним разбором. Раньше группа из 8 алертов превращалась
  в 8 инцидентов
- **Обратный канал разбора** — `ANALYSIS_CALLBACK_URL` у ai-worker. Без переменной
  поведение прежнее
- **Логи как слой обогащения** — enricher прикладывает к алерту выжимку логов
  из OpenSearch/Elasticsearch: повторы схлопнуты, счётчик `×N` показывает масштаб,
  INFO отброшен. Сбор логов в чарт не входит, пример — [docs/logs-enrichment.md](docs/logs-enrichment.md)
- **`ns_deployments_scaled_to_zero`** в контексте Kubernetes: деплойменты без единой
  реплики. Метрики такое не ловят — при `replicas: 0` всё выглядит здоровым,
  хотя сервис выключен
- Плейсхолдеры `{{ .Pod }}` и `{{ .Container }}` в шаблонах метрик — теперь метрики
  можно привязать к проблемному поду, а не только к namespace
- Модель сообщает уверенность в разборе числом; раньше интерфейс показывал заглушку
- **Redis в составе чарта** — сабчарт `incidentgpt-redis`: один узел, диск 2 Gi,
  вытеснение выключено. Раньше его требовалось ставить отдельно. Свой Redis
  подключается через `incidentgpt-redis.enabled: false`
- **Публикация образов в ghcr.io** — `.github/workflows/release.yml` собирает все
  пять образов по тегу `v*`. Метки `org.opencontainers.image.*` добавлены
  backend и ui, у остальных были

### Changed
- **Настройки хранилища логов названы по назначению, а не по продукту**:
  `ELASTICSEARCH_URL/USER/PASSWORD/INDEX` → `LOGS_STORE_*`, в values
  `elasticsearchUrl` → `logsStoreUrl`, блок `elasticsearch:` → `logsStore:`.
  Продукт может быть любым с поисковым API OpenSearch/Elasticsearch, а рядом
  уже жили `LOGS_RANGE_BEFORE` и `LOGS_MAX_LINES`. Слово `STORE` отделяет их от
  `LOGS_BASE_URL` — тот задаёт ссылку на веб-интерфейс логов, а не адрес API
- `LOGS_RANGE_BEFORE`, `LOGS_RANGE_AFTER` и `LOGS_MAX_LINES` выведены в values
  enricher: раньше их можно было менять только правкой окружения пода
- В «Исправлении» запрещены команды сбора данных (`kubectl logs`, `describe`, `tail`):
  собирать данные — работа системы. Если данных не хватает, модель пишет
  **Не хватает данных:** — это указание расширить сбор
- Логи для обогащения берутся по всему namespace, под из алерта идёт первым:
  причина часто оказывается у соседа

### Fixed
- `NextID` инкрементировал счётчик без блокировки — гонка при конкурентных запросах
- Backend-сервис в зонтичном чарте рендерился как `incidentgpt`, а nginx в ui
  проксировал `/api/` на `incidentgpt-backend` — свежая установка отвечала 502
  на каждый запрос интерфейса
- Дефолты чарта указывали на приватный registry: снаружи три компонента из пяти
  не скачивались. Теперь `ghcr.io/sersert/incidentgpt-*`
- В зонтичных values групповой путь enricher и обратный канал ai-worker не были
  сведены на backend — инциденты не доезжали до веба без ручной правки

[0.2.0]: https://github.com/sersert/IncidentGPT/compare/v0.1.0...v0.2.0
## [0.1.0] - 2026-08-06

Первый публичный релиз.

### Added
- Три сервиса: `enricher` (обогащение алертов метриками Prometheus + контекстом Kubernetes),
  `ai-worker` (LLM-разбор через OpenRouter + доставка в Telegram), `sanitizer` (маскирование секретов).
- Единый umbrella-чарт `deploy/incidentgpt` — установка всех трёх компонентов одним `helm install`.
- Корреляция каскада алертов в один инцидент по namespace + временному окну (Redis).
- AI-разбор одиночного алерта приходит комментарием (reply) под самим алертом в Telegram.
- Эндпоинты наблюдаемости во всех сервисах: `/metrics` (Prometheus), `/readyz`, `/healthz`, `/status`.

### Security
- **Data Sanitizer** — шлюз, маскирующий секреты (bearer-токены, connection strings, приватные ключи)
  до отправки данных в LLM и Telegram. HMAC-аутентификация между сервисами, поведение fail-closed.
- Секреты подключаются через `existingSecret` (совместимо с External Secrets / Vault),
  без плейнтекста в `values.yaml`.

### Ops
- Образы: distroless nonroot, multi-stage сборка, HEALTHCHECK, OCI-labels, BuildKit cache mounts,
  `.dockerignore` на каждый сервис.
- `sanitizer` chart: `runAsUser`/`runAsGroup` 65532 — `runAsNonRoot` проходит проверку PodSecurity.

### Thanks
- @opsmon — Data Sanitizer и эндпоинты мониторинга.

[Unreleased]: https://github.com/sersert/IncidentGPT/compare/v0.2.0...HEAD
[0.1.0]: https://github.com/sersert/IncidentGPT/releases/tag/v0.1.0
