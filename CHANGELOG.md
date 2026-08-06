# Changelog

Все значимые изменения проекта документируются в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/),
проект следует [семантическому версионированию](https://semver.org/lang/ru/).

## [Unreleased]

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

[Unreleased]: https://github.com/sersert/IncidentGPT/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sersert/IncidentGPT/releases/tag/v0.1.0
