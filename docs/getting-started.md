# Запуск и первое дежурство

Это руководство для отдельного проверочного стенда из текущего исходного кода.
Оно использует namespace `incidentgpt`, локальный доступ через port-forward и
базовый путь обработки без дополнительных функций. Для production сначала
проверьте кандидат в изолированном кластере с целевой конфигурацией.

## 1. Подготовьте окружение

Нужны Docker с работающим daemon и доступом к вашему registry, Kubernetes,
`kubectl`, Helm, доступный Prometheus и Redis с постоянным диском. Встроенный Redis
входит в чарт; используйте отдельный том и данные проверочного стенда. Команды ниже
выполняются из корня репозитория. Проверьте выбранный кластер:

```bash
kubectl config current-context
kubectl cluster-info
docker info
helm version
```

Подготовьте отдельного тестового Telegram-бота/канал и ключ модельного провайдера:
[Telegram](telegram-setup.ru.md). Реальная доставка тестового алерта вызывает
провайдера и отправляет сообщения в настроенный тестовый канал.

## 2. Соберите образы проверяемого коммита

Опубликованные образы `0.2.0` не содержат новые возможности этой ветки. Соберите
все пять компонентов из одного кандидата. Registry должен быть доступен кластеру;
для приватного registry настройте image pull Secret в соответствующих subcharts.

```bash
export INCIDENTGPT_REGISTRY=registry.example.com/your-project
export INCIDENTGPT_IMAGE_TAG="$(git rev-parse --short=12 HEAD)"
for component in ai-worker enricher sanitizer backend ui; do
  docker buildx build --platform linux/amd64 \
    --tag "${INCIDENTGPT_REGISTRY}/incidentgpt-${component}:${INCIDENTGPT_IMAGE_TAG}" \
    --push "./${component}"
done
```

Платформа примера — `linux/amd64`; согласуйте её с нодами стенда. Для выпуска
запишите полные commit/image digests: тег с хешем коммита сам по себе не делает
образ неизменяемым. Не продвигайте непроверенные образы в production.

## 3. Заполните настройки и отдельный файл секретов

```bash
cp docs/examples/values-preview.yaml values-local.yaml
```

В `values-local.yaml` замените все `repository`/`tag` на образы из шага 2, адрес
Prometheus и пароль UI. Не оставляйте значения `REPLACE_*`. В этом примере ingress
выключен; DNS/TLS для локального доступа не нужны. Имена сервисов и несколько
адресов привязаны к namespace `incidentgpt`: при другом namespace обновите также
адреса worker/sanitizer/collector и nginx upstream, а не только флаг `-n`.

Создайте `values-secret.yaml` (этот файл исключён из Git):

```yaml
incidentgpt-enricher:
  secretValues:
    sanitizerAuthSharedSecret: "<один-общий-HMAC-ключ>"
ai-worker:
  secretValues:
    sanitizerAuthSharedSecret: "<тот-же-HMAC-ключ>"
    openrouterApiKey: "<ключ-провайдера>"
    telegramBotToken: "<токен-тестового-бота>"
  env:
    TELEGRAM_CHANNEL_ID: "<chat-id-тестового-канала>"
    TELEGRAM_THREAD_CHAT_ID: "<chat-id-тестовой-группы>"
incidentgpt-sanitizer:
  secretValues:
    authSharedSecret: "<тот-же-HMAC-ключ>"
    hashKey: "<отдельный-ключ-псевдонимизации>"
```

Ключи можно сгенерировать `openssl rand -hex 32`. Ограничьте доступ к файлам:
`chmod 600 values-local.yaml values-secret.yaml`. Не коммитьте ключи или вывод
отрендеренных Secret. Sanitizer должен работать в fail-closed режиме; не отключайте
его ради прохождения теста. Подробнее: [граница санитизации](security/data-sanitizer-ru.md).

Для production используйте управляемые Secrets. При внешнем htpasswd Secret UI
нужен отдельный backend reader Secret с тем же логином и паролем. Генерируемая
umbrella-пара по умолчанию подходит для этого проверочного стенда; подробности —
[доступ к веб-интерфейсу](web-ui.md#доступ).

## 4. Разверните и проверьте готовность

```bash
helm dependency build --skip-refresh deploy/incidentgpt
helm lint deploy/incidentgpt -f values-local.yaml -f values-secret.yaml
helm upgrade --install incidentgpt deploy/incidentgpt \
  --namespace incidentgpt --create-namespace \
  -f values-local.yaml -f values-secret.yaml --wait --timeout 5m
kubectl -n incidentgpt get pods,pvc,svc
kubectl -n incidentgpt rollout status deployment/incidentgpt-backend
kubectl -n incidentgpt rollout status deployment/incidentgpt-ui
```

Ready означает готовность процесса, но не доказывает, что данные проходят весь
путь. Если deployment не готов, сначала изучите `kubectl describe pod` и логи.
Не переключайтесь на production для отладки.

В отдельном терминале откройте доступ:

```bash
kubectl -n incidentgpt port-forward service/incidentgpt-ui 8088:80
```

Откройте `http://127.0.0.1:8088`, войдите с логином/паролем из локального values.
Проверьте `/healthz` (`curl -fsS http://127.0.0.1:8088/healthz`) и загрузку API в
браузере. На новом стенде списки пусты. Руководство дежурного доступно в меню и
по пути `/help`. Health endpoint UI не проверяет Redis, worker или модель.

## 5. Проверьте один тестовый алерт

В отдельном терминале:

```bash
kubectl -n incidentgpt port-forward service/incidentgpt-enricher 9099:9099
```

Во втором терминале создайте сигнал с текущим временем:

```bash
python3 - <<'PY' > /tmp/incidentgpt-test-alert.json
import datetime, json
now = datetime.datetime.now(datetime.timezone.utc).isoformat()
print(json.dumps({"receiver": "preview", "status": "firing", "alerts": [{
    "status": "firing", "labels": {"alertname": "IncidentGPTPreview", "severity": "warning",
    "namespace": "incidentgpt", "service": "preview", "cluster": "incidentgpt-preview"},
    "annotations": {"summary": "Проверка тестового стенда"}, "startsAt": now,
    "endsAt": "0001-01-01T00:00:00Z"
}]}))
PY
curl --fail-with-body -sS -X POST http://127.0.0.1:9099/alert \
  -H 'Content-Type: application/json' --data-binary @/tmp/incidentgpt-test-alert.json
```

Дождитесь окна группировки (`corrSettle`, по умолчанию 40 секунд) и обработки
провайдером. В UI проверьте алерт, инцидент и разбор; в тестовом Telegram —
доставку. Проверьте назначение и принятие в работу. Отдельно проверьте перезапуск
backend с сохранением истории в Redis. Синтетический алерт без реального workload
может иметь неизвестный Kubernetes-контекст — это ожидаемо.

После успешного прямого теста настройте [webhook Alertmanager](../README.md#шаг-6-подключить-alertmanager)
и проверьте реальный маршрут. Прямой POST не проверяет Alertmanager, production
ingress, TLS, NetworkPolicy источника или доступ внешних интеграций.

## 6. Работайте с инцидентом

1. «Инциденты»: выберите статус/критичность, найдите запись, примите в работу.
2. «Обзор и анализ»: проверьте состав, логи и доказательства; зафиксируйте ответственного.
3. «Расследование»: сопоставьте изменения и выполните разрешённые проверки чтения.
4. «Решения и runbooks»: сравните прошлые случаи и сохраните проверенное решение.
5. «Отчёты и передача»: проверьте и отредактируйте черновик, назначьте последующие действия.

Шаги 3–5 требуют отдельно включённых функций. Разделы сохраняются в URL через
`?view=investigate`, `?view=knowledge`, `?view=reports`. Черновики остаются при
переключении вкладок внутри открытой карточки; перед уходом или перезагрузкой
их нужно сохранить. Подробнее: [веб-интерфейс](web-ui.md).

## Дополнительные функции и production

Включайте функции по порядку после развёртывания совместимых consumers:
[изменения](roadmap/change-timeline.md), [durable анализ](roadmap/evidence-and-delivery.md),
[подтверждённые решения](roadmap/confirmed-memory.md), [диагностика](roadmap/guided-diagnostics.md),
[зависимости](roadmap/dependency-correlation.md), [отчёты](roadmap/postmortem-and-handoff.md).
Согласуйте service tokens, reader credentials, стабильный cluster ID и
sanitizer Secrets. Redis здесь одновузловой: настройте AOF/backup и учитывайте
окна потери данных. Перед миграциями production сохраняйте резервную копию.

Для production ingress задайте реальный host, TLS и разрешения NetworkPolicy:
в UI-чарте есть CIDR исходного окружения, их нельзя считать подходящими вашему
кластеру. В preview с port-forward ingress намеренно не проверяется. Перед merge
проверьте изолированный кандидат с целевыми ingress/TLS, доступом к API, состоянием
Redis, очередями и smoke-сценарием, запишите digests/CI/deployment evidence в PR.

## Если результат отличается от ожидаемого

| Симптом | Что проверить |
| --- | --- |
| UI открывается, API отвечает 401/403 | Совпадают ли Basic credentials UI/backend; передаётся ли Authorization. X-Auth-User не заменяет проверку пароля. |
| Алерты есть, инцидентов нет | `incidentgpt-enricher.env.groupBackendUrl` должен вести на `/api/v1/ingest` backend; проверьте Redis и логи enricher. |
| Инцидент есть, разбора нет | В legacy режиме нужны callback worker → backend и доступ к модели/sanitizer. В durable режиме проверьте задания и service token; старый callback там не используется. |
| После перезапуска пропала история | Проверьте `REDIS_ADDR` backend, `redisAddr` enricher, PVC и persistence. In-memory режим не хранит историю между рестартами. |
| `ImagePullBackOff` | Доступность registry, правильные repository/tag, image pull Secrets и архитектуру нод. |
| Sanitizer отклоняет запрос | Совпадение HMAC ключей, policy, время и доступность сервиса. Не отключайте fail-closed. |
| Рабочая вкладка недоступна | Соответствующие feature flags, credentials, разрешённые namespaces, источники и совместимость образов. |
| Ingress недоступен, port-forward работает | Ingress class, DNS/TLS, адрес upstream, NetworkPolicy и CIDR вашего контроллера. |

Логи: `kubectl -n incidentgpt logs deployment/incidentgpt-backend`,
`deployment/incidentgpt-enricher`, `deployment/ai-worker` и
`deployment/incidentgpt-sanitizer`. Перед передачей логов удалите секреты.

## Локальная разработка интерфейса

[ui/README.md](../ui/README.md) описывает Vite, адрес backend, демонстрационные
данные и проверки перед PR. Демонстрационный режим не подтверждает работу
источников, API авторизации или развёртывания.
