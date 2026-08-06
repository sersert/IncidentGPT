## IncidentGPT v0.1.0 — первый публичный релиз 🎉

AIOps-помощник: обогащает алерты Prometheus/Alertmanager контекстом (метрики + K8s),
связывает каскад в один инцидент и шлёт AI-разбор в Telegram.

### ✨ Что внутри
- **3 сервиса**: `enricher` (обогащение), `ai-worker` (LLM-разбор + Telegram), `sanitizer` (маскирование секретов).
- **Единый umbrella-чарт** `deploy/incidentgpt` — все три компонента ставятся одним `helm install`.
- **Корреляция** каскада алертов по namespace + временному окну (Redis).
- **AI-разбор** одиночного алерта приходит комментарием под самим алертом.

### 🔒 Безопасность
- **Data Sanitizer** — шлюз, который маскирует секреты (bearer-токены, connection strings, приватные ключи) **до** отправки в LLM и Telegram. HMAC-аутентификация между сервисами, fail-closed.
- Секреты — через `existingSecret` (совместимо с External Secrets / Vault), без плейнтекста в values.

### 📊 Мониторинг
- Эндпоинты `/metrics` (Prometheus), `/readyz`, `/healthz`, `/status` во всех сервисах.

### 🐳 Ops
- Образы: distroless nonroot, multi-stage, HEALTHCHECK, OCI-labels, BuildKit cache mounts.

### 🙌 Спасибо
- @opsmon — Data Sanitizer, эндпоинты мониторинга.

### ⚠️ Важно при установке
- Сгенерируй ключи: `openssl rand -hex 32` для `auth-shared-secret` и `hash-key`.
- Не коммить реальные ключи — используй `existingSecret`.
