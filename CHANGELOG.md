# Changelog

Все значимые изменения проекта документируются в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
проект следует [семантическому версионированию](https://semver.org/lang/ru/).

## [Unreleased]

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
