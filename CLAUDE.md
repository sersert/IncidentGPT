# IncidentGPT — контекст для агентов

Open-source инструмент поверх Prometheus/Alertmanager: обогащает алерты
контекстом, склеивает каскад в инцидент, просит LLM написать черновик разбора
и отдаёт его инженеру в Telegram и в веб-интерфейс. Подробно: `README.md`,
`docs/architecture.md`, `docs/security.md`, `docs/web-ui.md`.

## Путь данных

```
Alertmanager ──webhook──► enricher ──► sanitizer ──► backend ──► ai-worker ──► OpenRouter (LLM)
                            │  Prometheus, K8s API, логи        │               │
                            └── Redis (окно корреляции) ◄───────┘      Telegram + разбор в backend
```

- `enricher/` (Go) — приём `POST /alert`, обогащение метриками/K8s/логами, буфер групп в Redis (`grp:{namespace}`), отправка группы в `GROUP_BACKEND_URL`.
- `sanitizer/` (Go) — маскирует секреты и персональные данные **до** LLM, Telegram и логов. HMAC-аутентификация запросов, `SANITIZER_FAIL_CLOSED=true` по умолчанию.
- `backend/` (Go) — хранит инциденты и алерты в Redis, отдаёт API для UI, пересылает группу в ai-worker как есть, принимает обратный канал с разбором.
- `ai-worker/` (Go) — единственный, кто знает про Telegram и OpenRouter. Сырой алерт постится **до** обращения к модели: LLM не точка отказа.
- `ui/` (React + TS, nginx) — интерфейс и прокси `/api/` на backend, basic-auth на nginx.
- `deploy/incidentgpt/` — umbrella-чарт; сабчарты лежат в `*/chart`, Redis в `deploy/redis`.

## Принципы

- **Черновик, а не решение.** Никакой автоматической remediation. Разбор проверяет человек.
- **Алерт ≠ инцидент.** Алерт — единица потока Alertmanager; инцидент — группа алертов с одним разбором. В UI это разные разделы.
- **UI для инженеров.** Показываем метки мониторинга, состав группы, контекст, разбор, логи. Никаких ROI-витрин, бизнес-метрик и абстракций поверх меток.
- **Компоненты отключаемы и самодостаточны.** Без backend/ui остаётся связка enricher → ai-worker → Telegram. Без Redis сервисы работают в памяти с предупреждением. Без Prometheus/K8s обогащение пропускается, а не падает.
- **Новое — за флагом, по умолчанию выключено** (`CHANGES_ENABLED`, `DURABLE_ANALYSIS_ENABLED`, `RUNBOOK_DIRECTORY`…). Поведение по умолчанию после апгрейда не меняется.
- **Sanitizer — граница безопасности.** Любые новые данные, уходящие в LLM, Telegram, webhook или лог, проходят через него. Fail-closed: отказ sanitizer ≠ отправка сырых данных.

## Правила для кода и чартов

- Go: `gofmt`, `go vet`, тесты рядом с кодом; CI гоняет их в пиннутом `golang:X-bookworm` из Dockerfile сервиса. Версия Go в `go.mod` и Dockerfile должна совпадать.
- Таймауты и контексты на все внешние вызовы (Prometheus, K8s, Redis, LLM, Telegram). Клиенты Redis должны переживать старт раньше Redis и его рестарт.
- Секреты только через `secrets.existingSecret` / `*.existingSecret` с настраиваемыми именами ключей. Ничего секретного в values, логах, аннотациях и промптах.
- Каждый новый сетевой путь между сервисами — правка NetworkPolicy в чарте получателя. Политики аддитивны и не ломают существующие.
- Чарт должен ставиться на любой Kubernetes (kubeadm, k3s, managed): никаких зашитых ClusterIP, DNS-адресов, ingressClass или CIDR нод в образах и шаблонах — всё через values.
- Имена сервисов зафиксированы `fullnameOverride` (`incidentgpt-backend`, `incidentgpt-redis`, `ai-worker`…), на них ссылаются другие сабчарты. Переименование = несовместимость.
- Схема ключей Redis и формат API backend — контракт. Изменение требует миграции или обратной совместимости и описания в `CHANGELOG.md`.
- Комментарии в values и docs — на русском, в стиле существующих файлов.
- Образы: `ghcr.io/sersert/incidentgpt-*`, тег = версия релиза, не `latest`.

## Что уже ловили (проверять при ревью)

- Адрес резолвера DNS зашит в `ui/nginx.conf` (`10.96.0.10`) — на k3s `/api/` висит. Нужен параметр.
- enricher и backend не переподключаются к Redis, если стартовали раньше него: корреляция выключается, инциденты живут в памяти.
- UI-чарт создаёт Secret `<release>-auth` с `admin/admin` даже при заданном `auth.existingSecret`.
- Новые карточки UI отвечали 401, потому что backend не получал `TIMELINE_READ_PASSWORD` без `changes.existingSecret`.
- Большие стековые PR, где каждый содержит предыдущие: ревьюить только дельту относительно base, лишнее просить вынести.
