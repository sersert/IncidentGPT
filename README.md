# IncidentGPT — обогащение и корреляция алертов с AI-разбором

![License](https://img.shields.io/badge/license-MIT-green)
![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)
![Kubernetes](https://img.shields.io/badge/Kubernetes-Helm-326CE5?logo=kubernetes&logoColor=white)
![Prometheus](https://img.shields.io/badge/Prometheus-Alertmanager-E6522C?logo=prometheus&logoColor=white)
![LLM](https://img.shields.io/badge/LLM-OpenRouter-6E56CF)
![correlation](https://img.shields.io/badge/correlation-namespace%20%2B%20window-lightgrey)

Два маленьких Go-сервиса поверх твоего Prometheus/Alertmanager, которые:

1. **Обогащают** каждый алерт контекстом — метрики из Prometheus (с трендами) + статус
   подов/нод/событий из Kubernetes API.
2. **Связывают** каскад алертов в один инцидент (по namespace + временному окну).
3. Просят LLM написать **черновик разбора** — вероятная первопричина и что проверить.
4. Кладут всё в **Telegram**: сырые алерты по одному + разбор.

> Это не «ИИ вместо инженера». На выходе — черновик; решение принимает человек.

---

## Как это работает

```
Cluster / ноды
   │  метрики
   ▼
Prometheus ──► Alertmanager
                   │  webhook (сырой алерт)
                   ▼
              Data Enricher (Go)
                   │  ├─ идёт в Prometheus за метриками [startsAt−15m … +5m]
                   │  ├─ идёт в K8s API за статусом подов/нод/событий
                   │  └─ буферизует в Redis (окно корреляции)
                   ▼
              AI Worker (Go)  ──► OpenRouter (LLM)
                   │
                   ▼
              Telegram: сырые алерты по одному → сводка «N связанных» → AI-разбор
```

- **Alertmanager** ничего не знает про Telegram/LLM — просто шлёт webhook на enricher.
- **Enricher** сам ходит за контекстом (pull), буферит в Redis для склейки.
- **AI Worker** — единственный, кто знает про Telegram и OpenRouter.
- Сырой алерт постится **до** обращения к модели → **LLM не точка отказа**.

---

## Требования

- Кластер Kubernetes с **kube-prometheus-stack** (Prometheus + Alertmanager + kube-state-metrics)
- **Redis** в кластере (для корреляции; без него enricher шлёт алерты по одному)
- Аккаунт **OpenRouter** (https://openrouter.ai) — или любой OpenAI-совместимый эндпоинт
- **Telegram-бот** + канал + группа обсуждения
- Container registry для образов (или собери и запушь свои)

---

## Шаг 1. Telegram: бот + канал + группа

Порядок важен — комментарии от бота работают только в канале со **связанной группой**.

1. **Создай бота.** Напиши [@BotFather](https://t.me/BotFather): `/newbot` → имя, username.
   Получишь **токен** вида `123456:AA...` → это `TELEGRAM_BOT_TOKEN`.
   Полезно: `/setprivacy` → **Disable** (чтобы бот видел сообщения в группе).
2. **Создай канал** (например `IncidentGPT`) и **группу** (`IncidentGPT Group`).
3. В настройках канала → **Discussion** → привяжи группу.
   *(если не привязывается — сделай группу временно публичной, привяжи, потом верни приватной).*
4. **Добавь бота админом** и в канал, и в группу (право Post/Send Messages).
5. **Узнай chat_id.** Напиши что-нибудь в канал, потом открой в браузере:
   ```
   https://api.telegram.org/bot<ТОКЕН>/getUpdates
   ```
   Найди:
   - `"chat":{"id":-100...,"type":"channel"}` → `TELEGRAM_CHANNEL_ID`
   - `"chat":{"id":-100...,"type":"supergroup"}` → `TELEGRAM_THREAD_CHAT_ID`

## Шаг 2. OpenRouter

Заведи ключ на https://openrouter.ai/keys → это `OPENROUTER_API_KEY`.
Модель по умолчанию — `google/gemini-2.5-flash` (можно любую с OpenRouter).

## Шаг 3. Redis

Любой Redis в кластере. Например Bitnami, одиночный инстанс без персистентности:
```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm upgrade --install redis bitnami/redis -n incidentgpt --create-namespace \
  --set architecture=standalone --set auth.enabled=false --set master.persistence.enabled=false
```
Сервис будет `redis-master.incidentgpt.svc.cluster.local:6379` → это `redisAddr`.

## Шаг 4. Собрать образы

```bash
# ai-worker
cd ai-worker
docker buildx build --platform linux/amd64 -t <твой-registry>/ai-worker:latest . --push
# enricher
cd ../enricher
docker buildx build --platform linux/amd64 -t <твой-registry>/incident-enricher:latest . --push
```

## Шаг 5. Заполнить values и задеплоить

### ai-worker — `ai-worker/chart/values.yaml`
```yaml
image:
  repository: <твой-registry>/ai-worker
  tag: latest
imagePullSecret: ""              # имя secret'а, если registry приватный

env:
  OPENROUTER_API_KEY: "<ключ OpenRouter>"      # 🔑
  OPENROUTER_MODEL: "google/gemini-2.5-flash"
  OPENROUTER_MAX_TOKENS: "2000"

  TELEGRAM_BOT_TOKEN: "<токен бота>"           # 🔑
  TELEGRAM_CHANNEL_ID: "-100..."               # id канала
  TELEGRAM_THREAD_CHAT_ID: "-100..."           # id группы обсуждения
```
```bash
helm upgrade -i ai-worker ./ai-worker/chart -n incidentgpt
```

### enricher — `enricher/chart/incidentgpt-enricher/values.yaml`
```yaml
image:
  repository: <твой-registry>/incident-enricher
  tag: latest

env:
  prometheusUrl: "http://<release>-kube-prometheus-stack-prometheus.monitoring:9090"
  clusterName: "my-cluster"
  redisAddr: "redis-master.incidentgpt.svc.cluster.local:6379"
  # groupBackendUrl / rawBackendUrl уже указывают на ai-worker внутри кластера
  corrWindow: "10m"     # сколько держим группу в Redis
  corrSettle: "40s"     # сколько ждём следствий каскада перед склейкой
  runbookBaseUrl: ""    # опц. база ссылок на runbook: <url>/<alertname>
```
Секция `metrics:` в этом же файле — какие PromQL enricher гоняет при обогащении.
**Это примеры, адаптируй под свои метрики** (см. комментарии в файле).
```bash
helm upgrade -i incidentgpt-enricher ./enricher/chart/incidentgpt-enricher -n incidentgpt
```

## Шаг 6. Подключить Alertmanager

Alertmanager должен слать webhook на enricher (порт **9099**, путь **/alert**).
В values kube-prometheus-stack:
```yaml
alertmanager:
  config:
    route:
      receiver: incident-enricher
      routes:
        - receiver: incident-enricher
          continue: true         # чтобы алерты шли и сюда, и в другие ресиверы
    receivers:
      - name: incident-enricher
        webhook_configs:
          - url: "http://incidentgpt-enricher.incidentgpt.svc:9099/alert"
            send_resolved: true
```
> ⚠️ Имя в `route.receiver` должно **точно совпадать** с `receivers[].name`.
> Для быстрого отклика на демо можно снизить `group_wait` роута до `2s`.

---

## Проверка

### Быстрый тест — послать алерт прямо в enricher (без Alertmanager)
```bash
kubectl port-forward -n incidentgpt deploy/incidentgpt-enricher 9099:9099

curl -s -XPOST -H "Content-Type: application/json" localhost:9099/alert -d '{
  "receiver":"test","status":"firing",
  "alerts":[{
    "status":"firing",
    "labels":{"alertname":"SyntheticHighCPU","severity":"critical","namespace":"monitoring","pod":"synthetic-pod-1","node":"worker-1","instance":"10.0.0.1:9100"},
    "annotations":{"summary":"Synthetic test alert","expr":"sum(rate(container_cpu_usage_seconds_total{namespace=\"monitoring\"}[5m]))"},
    "startsAt":"2025-11-25T10:00:00Z","endsAt":"0001-01-01T00:00:00Z"
  }]
}'
```
(больше примеров — в `enricher/check_pod`)

### Тест через настоящий PrometheusRule
Заведи правило, которое гарантированно сработает (пример — `ai-worker/cpu-alert.yaml`),
и Alertmanager сам прогонит его через enricher.
```bash
kubectl apply -f ai-worker/cpu-alert.yaml   # PrometheusRule с меткой release: <твой-release>
```
> Метка `release:` в PrometheusRule должна совпадать с тем, что селектит твой Prometheus,
> иначе правило не подхватится.

### Тест ai-worker напрямую (уже обогащённый алерт)
Пример payload — в `ai-worker/readme` (POST на `/incident`).

### Логи обогащения
```bash
kubectl logs -f -n incidentgpt deploy/incidentgpt-enricher   # ALERT_RAW / ALERT_ENRICHED / ALERT_BUFFERED / GROUP_SENT
kubectl logs -f -n incidentgpt deploy/ai-worker              # got incident / raw alert posted / group analysis posted
```

---

## Корреляция: как связываются алерты

Связывание — **грубое и детерминированное**: алерты одного `namespace`, пришедшие в
окно `CORR_SETTLE`, считаются одним инцидентом.

1. Каждый алерт enricher кладёт в Redis-хеш `grp:{namespace}` (поле = fingerprint).
2. Первый алерт группы заводит debounce-таймер на `CORR_SETTLE` секунд.
3. Пока таймер идёт, любой алерт того же namespace добавляется в тот же хеш.
4. Таймер сработал → вся группа уходит пачкой в ai-worker → **один вызов LLM**.
5. **Корень** внутри группы определяет уже модель (не связывание).

**Чего тут НЕТ:** графа зависимостей. «Связаны» = один namespace + окно. Может ложно
склеить несвязанное; не ловит кросс-namespace каскады. Зато дёшево и работает для
каскадов внутри namespace (упала БД → сервисы того же ns посыпались).

Весь модуль корреляции — `enricher/correlation.go` (~200 строк, 6 функций).

---

## Структура

```
ai-worker/          # Go: промпт, вызов LLM, постинг в Telegram
  main.go
  chart/            # Helm-чарт ai-worker
enricher/           # Go: обогащение + корреляция
  main.go           #   приём алерта, обогащение метриками/K8s
  correlation.go    #   Redis-буфер, окно, склейка группы
  k8s.go            #   походы в Kubernetes API
  chart/            # Helm-чарт enricher (+ секция metrics в values)
```

## Ключевые настройки (enricher env)

| Переменная | Дефолт | Смысл |
|---|---|---|
| `PROM_RANGE_BEFORE` / `PROM_RANGE_AFTER` | `15m` / `5m` | окно метрик вокруг `startsAt` алерта |
| `CORR_WINDOW` | `10m` | TTL группы в Redis |
| `CORR_SETTLE` | `40s` | сколько ждать следствий каскада перед склейкой |
| `REDIS_ADDR` | — | адрес Redis (⚠️ у Bitnami сервис = `...-master`) |
| `RUNBOOK_BASE_URL` | `""` | база ссылок на runbook; пусто → не добавляется |
| `ENRICH_*_CONTEXT` | `true` | какие слои контекста собирать |

## Лицензия

MIT (см. `LICENSE`).
