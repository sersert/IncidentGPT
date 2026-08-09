# Обогащение логами

Кроме метрик и контекста Kubernetes enricher умеет прикладывать к алерту выжимку
логов. Это заметно меняет разбор: без логов модель пишет «проверь логи пода», с
логами — называет конкретную ошибку.

Сбор логов **не входит в чарт**: в кластере обычно уже есть свой стек, и ставить
второй незачем. Ниже — рабочий пример на OpenSearch и fluent-bit, но подойдёт
любое хранилище с совместимым поисковым API.

---

## Что нужно от хранилища

Enricher шлёт обычный `_search` и ждёт документы с полями:

| Поле | Что содержит |
|---|---|
| `@timestamp` | время строки |
| `log` или `message` | сам текст |
| `kubernetes.namespace_name` | namespace пода |
| `kubernetes.pod_name` | имя пода |

Такую структуру даёт fluent-bit с фильтром `kubernetes` — она же у filebeat и
fluentd с аналогичным обогащением.

> **Подвох с маппингом.** Строковые поля индексируются как `text` с подполем
> `keyword`, а точное совпадение `term` работает только по `.keyword`. Запрос
> без суффикса молча возвращает ноль документов — ошибки не будет, просто
> пустой результат. Enricher уже ходит по `.keyword`.

---

## Пример: OpenSearch

Один узел, без кластера — для сбора логов этого достаточно:

```yaml
# values.yaml
image:
  repository: opensearchproject/opensearch
  tag: "2.17.1"
persistence:
  size: 20Gi
```

Ключевые переменные пода:

```yaml
env:
  - name: discovery.type
    value: single-node
  # TLS терминируется на ingress, внутри кластера ходим по HTTP — иначе
  # пришлось бы возить самоподписанный сертификат по всем клиентам.
  - name: DISABLE_SECURITY_PLUGIN
    value: "true"
  - name: OPENSEARCH_JAVA_OPTS
    value: "-Xms1g -Xmx1g"
```

Проба готовности — TCP, а не HTTP: с включённой авторизацией движок отвечает
401 на любой путь, и HTTP-проба потребовала бы пароль прямо в манифесте.

Интерфейс — **OpenSearch Dashboards** отдельным деплойментом. Наружу публикуйте
именно его, а не сам движок: голый API с полным доступом к данным в интернете
не нужен. Вход закрывайте basic-авторизацией на ingress.

---

## Пример: fluent-bit

DaemonSet читает логи с нод. Фильтровать namespace лучше **по пути к файлу** —
тогда лишние логи даже не читаются с диска:

```ini
[INPUT]
    Name tail
    Tag kube.myapp.*
    Path /var/log/containers/*_myapp_*.log
    multiline.parser docker, cri
    Mem_Buf_Limit 32MB
    Skip_Long_Lines On
    # На части нод бывает исчерпан лимит inotify-инстансов, и сборщик падает
    # с «Too many open files». Опрос дороже по CPU, но не зависит от соседей.
    Inotify_Watcher Off

[FILTER]
    Name kubernetes
    Match kube.*
    Kube_Tag_Prefix kube.myapp.var.log.containers.
    Merge_Log On
    Keep_Log Off

# Собственные логи сборщика в индекс не пускаем: иначе он пишет о том, что пишет.
[FILTER]
    Name grep
    Match kube.*
    Exclude $kubernetes['labels']['app.kubernetes.io/name'] fluent-bit

[OUTPUT]
    Name opensearch
    Match kube.*
    Host opensearch.logging.svc
    Port 9200
    Logstash_Format On
    Logstash_Prefix logs-myapp
    Time_Key @timestamp
    Replace_Dots On
    Suppress_Type_Name On
```

Для Elasticsearch вместо `Name opensearch` укажите `Name es`.

Учтите: сборщику логов нужен `hostPath` к `/var/log`. Политики вроде Kyverno
`disallow-host-path` будут на это ругаться — для DaemonSet'а сбора логов это
неустранимо, ему нужен доступ к файлам на ноде.

---

## Подключение к enricher

```yaml
incidentgpt-enricher:
  env:
    logsStoreUrl: "http://opensearch.logging.svc:9200"
    logsStoreUser: ""              # если авторизация включена
    logsStoreIndex: "logs-*"
  logsStore:
    existingSecret: ""             # секрет с ключом password
    passwordKey: password
```

Без `logsStoreUrl` шаг обогащения молча пропускается — компонент остаётся
рабочим и без логов.

> Не путайте с `logsBaseUrl`: тот задаёт базу для **ссылки** на веб-интерфейс
> логов, которую инженер открывает руками. `logsStoreUrl` — адрес API, куда
> ходит сам enricher.

Тот же адрес нужен backend'у для вкладки «Логи» в инциденте:

```yaml
incidentgpt-backend:
  env:
    LOGS_STORE_URL: "http://opensearch.logging.svc:9200"
    LOGS_STORE_INDEX: "logs-*"
```

Настройки объёма:

| Переменная | По умолчанию | Смысл |
|---|---|---|
| `LOGS_RANGE_BEFORE` | `10m` | сколько смотреть до алерта |
| `LOGS_RANGE_AFTER` | `2m` | сколько после |
| `LOGS_MAX_LINES` | `40` | сколько групп класть в промпт |

---

## Что попадает в модель

Класть в промпт сотни строк нельзя — он раздувается, дорожает и топит важное
в шуме. Поэтому enricher:

- отбрасывает `INFO`: причину ищут в ошибках;
- **схлопывает повторы** — из строки убираются времена, числа и хеши, всё
  остальное считается одной ошибкой;
- сортирует `FATAL` → `ERROR` → `WARN`, внутри уровня по частоте;
- берёт логи всего namespace, но под из алерта ставит первым.

В результате пятьсот строк превращаются в пару десятков:

```
[ERROR ×156] catalogue-674f789b65: circuit breaker 'List' is open
[ERROR ×16]  catalogue-674f789b65: database connection error
[WARN ×3]    session-db-7979d74bb: Read-only file system
```

Счётчик `×N` — сам по себе сигнал: единичный сбой или шторм.
