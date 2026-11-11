# Мониторинг компонентов и пауза AI

Страница `/operations` показывает доступность компонентов и позволяет приостановить
AI-анализ. Возможность включается явно и требует совместимых backend, ai-worker,
enricher и UI из одного проверенного кандидата. Без opt-in поведение прежнее.

## Включение

1. Подготовьте в namespace релиза Secret `incidentgpt-operations` с ключом
   `operations-service-token`: случайная строка длиной не менее 32 байт
   (`openssl rand -hex 32`). Один и тот же Secret используется тремя Go-сервисами.
   Значение не кладите в values, Git или PR.
2. Подготовьте Secret `incidentgpt-reader` с ключами `username` и `password`:
   это исходные credentials общей administrator-учётной записи UI Basic-auth,
   а не строка htpasswd. Он должен соответствовать существующему UI auth.
   Если backend уже использует `changes.existingSecret`, оставьте его и уберите
   `readerAuth.existingSecret` из примера: задавать оба нельзя.
3. Наложите [values-operations.yaml](examples/values-operations.yaml) на рабочие
   values и задайте image tag проверенного кандидата для всех сервисов.
   Поднимите отдельный preview namespace с отдельными Redis/PVC/Secrets/ingress.
   Параметры адресов в `operations` нужны при других именах сервисов или портах.
   При имени Helm release, отличном от `incidentgpt`, согласуйте
   `incidentgpt-backend.networkPolicy.aiWorker.instance` с именем релиза.
4. Убедитесь, что backend подключён к постоянному Redis (`REDIS_ADDR`), а UI
   проксирует Basic Authorization и browser Origin. При замене Host разрешите
   реальный полный Origin через backend `ALLOWED_ORIGINS`. Не используйте `*`.
5. Откройте `/operations`. **Первое включение начинается с паузы**: при отсутствии
   записи Redis `enabled=false`. Нажмите «Включить AI» после проверки компонентов.

В чартах `operations.enabled=false` по умолчанию. `existingSecret` и `tokenKey`
для umbrella согласуются одним `global.operations.enabled=true`. Частичное
включение отклоняется при рендере. В отдельных сабчартах остаётся локальный opt-in. Backend задаёт
`OPERATIONS_ENABLED`, `OPERATIONS_WORKER_URL`, `OPERATIONS_ENRICHER_URL`,
`OPERATIONS_MCP_HEALTH_URL`; worker — `AI_CONTROL_URL`; три сервиса получают
`OPERATIONS_SERVICE_TOKEN` только через Secret. Enricher также получает
`OPERATIONS_ENABLED=true`. Не переопределяйте эти управляемые env вручную.

## Что означает лампочка

Автообновление раз в 10 секунд не вызывает генерацию моделью. Backend кеширует
общие проверки на 10 секунд; состояние переключателя читает из Redis отдельно.
Один независимый сбор ограничен четырьмя секундами; HTTP, Redis и логи проверяются
параллельно. Закрытие вкладки не отменяет снимок для остальных пользователей.
Ошибки обновления опрашиваются с интервалом 60 секунд, 401/403/404 останавливают опрос.
Сетевые проверки имеют таймауты. Серый статус означает отсутствие свежего
подтверждения, отключённый или ненастроенный источник. При ошибке обновления UI
снимает прежние зелёные статусы и блокирует управление до успешного чтения.

| Компонент | Что реально проверяется |
|---|---|
| Модель LLM | Последний корректный ответ на реальный inference в опрошенном worker, не старше 5 минут. Настроенный URL или `/readyz` worker не доказывают доступ к модели. |
| Backend | Успешный ответ API мониторинга. |
| AI worker, enricher, sanitizer | HTTP `/readyz`; отдельные runtime-запросы worker/enricher требуют service token. |
| Redis | PING из backend; отдельно PING из enricher. |
| Telegram | Реальный `getMe` из worker, кеш 60 секунд. Доставка в конкретный чат/канал этим не проверяется. |
| Метрики | Реальный Prometheus `/-/ready` из enricher. Успех отдельных PromQL-запросов не гарантируется. |
| Kubernetes | Реальный `/version` из enricher. Workload RBAC отдельно не проверяется. |
| Логи | Loki `/ready` или корневой health API OpenSearch с настроенным Basic-auth. Поисковый запрос не выполняется. |
| MCP | Опциональный health URL из конфигурации backend; без установленного MCP — «Не настроен». Доступ к каждой базе знаний отдельно не проверяется. |

Кнопка ручной проверки модели делает один запрос с фиксированным текстом через
sanitizer, `max_tokens=8`, без инцидентов и найденных документов. Это **платное
действие**: 8 — лимит ответа, входной текст и особенности billing провайдера
считаются отдельно. Глобальный Redis limiter и дополнительный локальный worker
limiter допускают не более одного запуска в минуту. Автоматических повторов нет.
Во время паузы ручная проверка недоступна. Текст ответа и provider credentials
не возвращаются в мониторинг и не сохраняются в наблюдениях.

## Что делает выключатель

Backend атомарно сохраняет желаемое состояние и номер ревизии. PUT проверяет
Basic-auth общей administrator-роли, JSON Content-Type и доверенный Origin.
Actor берётся только из проверенного Basic user; `X-Auth-User` не используется.
Конкурирующее изменение со старой ревизией получает 409. Без Redis изменение
отклоняется. Включение требует доступного совместимого managed worker. Запрос
на паузу можно сохранить и при недоступном worker; UI отдельно показывает,
подтвердил ли worker применение. Это позволяет поставить паузу до его восстановления.

Worker читает control API перед **каждым** POST к провайдеру: legacy-анализ,
structured-анализ и ручной probe проходят через один transport guard. Фоновый
опрос раз в 2 секунды отменяет контексты текущих запросов; сетевой таймаут
control API — 2 секунды. Это не обещание мгновенной отмены или возврата оплаты:
уже принятый провайдером запрос может потратить токены. Redirect провайдера в
managed-режиме не выполняется. При недоступном control API, некорректной записи
или откате ревизии worker блокирует новые вызовы и отменяет текущие. Счётчик
активных вызовов и подтверждение в UI относятся к **опрошенному pod**, а не ко
всем репликам; все реплики должны иметь одинаковую конфигурацию control.

При паузе сырые алерты, хранение инцидентов, обычные уведомления Telegram и
детерминированные report jobs продолжают работать. Durable analysis/diagnostics
остаются в очереди без расхода попыток; после включения продолжаются автоматически.
Если пауза застала running analysis, ответ worker 423 возвращает его в очередь
с компенсацией попытки. Пауза не очищает очередь и не удаляет данные. Legacy
анализы, пропущенные во время паузы, автоматически не переигрываются.

## Хранение, сеть и совместимость

Новые ключи additive, миграция существующих инцидентов не нужна:

- `igpt:operations:ai-control` — JSON schema_version=1, enabled, revision,
  updated_at/by, без TTL. Сохраняйте его в backup Redis вместе с очередью.
- `igpt:operations:ai-control:audit` — stream, приблизительный MAXLEN 1000,
  каждая успешная смена состояния; без TTL. Это запись общей admin-учётной записи,
  а не индивидуальный корпоративный аудит.
- `igpt:operations:model-probe:limit` — limiter, TTL 60 секунд.

Перезапуск backend/worker сохраняет паузу. Потеря ключа начинает с паузы; worker
при откате к enabled=true остаётся закрытым. Безопасный сброс к enabled=false
принимается как новая база ревизий: дождитесь подтверждения паузы, прочитайте
актуальное состояние и включите AI с его CAS-ревизией. Рестарт worker не нужен. Старые версии игнорируют новые
ключи. **Удаление `AI_CONTROL_URL`, выключение флага в chart или rollback worker
на старую версию возвращает прежнее поведение с разрешённым AI**. Если нужно
сохранить паузу, оставьте совместимый worker и control-конфигурацию. Недоступный
backend у такого worker блокирует AI. Не используйте снятие opt-in как способ
оставить AI выключенным.

Новые HTTP-маршруты backend: `GET /api/v1/operations/status`,
`GET/PUT /api/v1/operations/ai-control`, `POST /api/v1/operations/model-probe`
(Basic-auth). `GET /internal/operations/ai-control` закрыт Bearer service token;
browser Origin запрещён. Worker: `GET /internal/operations`,
`POST /internal/operations/model-probe`; enricher: `GET /internal/operations` —
тот же service token, без browser Origin. На внешнем ingress `/internal/*`
не публикуется. Отказ LLM/control не меняет readiness raw ingestion.

Существующие receiver NetworkPolicy разрешают backend→worker, worker→backend,
backend→enricher и backend→sanitizer. Опциональный MCP grant выбирает
`component=knowledge-mcp` и `instance=<имя релиза>` в том же namespace, порт
`operations.mcpPort` (8080 по умолчанию). Для внешнего MCP настройте receiver
policy отдельно; URL не создаёт доступ сквозь внешние firewall/egress policy.

## Проверка кандидата перед merge

CI гоняет Go race-тесты с Redis, типы/сборку UI, браузерные сценарии управления,
ошибок и мобильной ширины, Helm guards и сборку образов. Браузерные сценарии
используют фикстуры и не доказывают работу реального ingress/Secret/провайдера.
Перед merge нужен изолированный deployment с intended runtime configuration:
сохраните SHA и image digests в PR, проверьте readiness/ingress/Basic/Origin,
реальные source checks, raw-alert→Telegram при паузе, очередь и возобновление,
перезапуск с сохранением паузы, отказ Redis/control. Платный probe запускайте
явно, отдельно от smoke без токенов. При blocker deployment оставьте PR открытым.

Optional MCP health URL сам по себе не создаёт NetworkPolicy на чужом получателе.
Предпочтительно разрешить backend в чарте MCP. Старый receiver grant доступен только
через отдельный `operations.mcpNetworkPolicy.enabled=true`; первая Ingress-политика
изолирует выбранные MCP-поды, поэтому оператор должен сохранить разрешения других
клиентов в дополнительных политиках. Не включайте grant для внешнего MCP.

Worker 423 откладывает durable job независимо от флага operations backend, без
расхода retry budget. Лимит ручной проверки модели расходуется после проверок
control/sanitizer в worker; явные 401/423/503 preflight-отказы освобождают резерв
backend. При неизвестном сетевом исходе резерв сохраняется, чтобы не повторять
потенциально платный запрос. Ошибки internal credentials возвращаются как 502 с
error_code, без ложного браузерного 401. Provider URL сравнивается с нормализацией
схемы, хоста и стандартного порта; query не освобождает endpoint от guard.

При operations backend ожидает Redis при старте так же, как durable-очередь; поздний
старт Redis не закрепляет memory fallback. Пустой REDIS_ADDR — ошибка конфигурации.
