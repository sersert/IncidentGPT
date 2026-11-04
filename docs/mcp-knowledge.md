# Автоматический поиск в базе знаний через MCP

Оператор задаёт разрешённые Confluence spaces либо GitHub/GitLab-репозитории и каталоги. После включения AI worker сам решает во время разбора, нужно ли уточнение по документации, вызывает MCP-инструмент `search_knowledge` и продолжает анализ с найденными материалами. Копировать страницы в промпт или вручную запускать поиск для каждого инцидента не требуется. Это работает в обычном Markdown-разборе и в structured/durable режиме.

Отдельный компонент `knowledge-mcp` предоставляет Streamable HTTP endpoint `/mcp` на официальном [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk). Разрешён один инструмент поиска; инструментов записи, shell, изменения инфраструктуры или запуска команд нет. Модель получает инструмент только при настроенном `KNOWLEDGE_MCP_URL`. Без этой настройки обработка не обращается к внешней базе знаний.

## Что оператор настраивает

Пример всех трёх источников: [values-mcp-knowledge.yaml](examples/values-mcp-knowledge.yaml). Скопируйте его в локальный файл, замените адреса/репозитории/spaces и удалите ненужные источники. В конфигурации остаются только имена ключей Secret; сами credentials в values не передаются.

| Поле источника | Назначение |
| --- | --- |
| `id` | Стабильное уникальное имя из строчных букв, цифр и дефисов. Оно сохраняется вместе со ссылками. |
| `kind` | `confluence`, `github` или `gitlab`. |
| `api_url` | Фиксированный HTTPS API endpoint, без токена, query string или fragment. |
| `web_url` | HTTPS корень веб-интерфейса для ссылок на документы. |
| `spaces` | Обязательный список разрешённых Confluence space keys. |
| `repository` | Один `owner/repo` для GitHub либо project path (`group/project`) для GitLab. |
| `paths` | Обязательные каталоги/файлы внутри репозитория, например `[docs, runbooks]`. `.` явно разрешает документацию во всём репозитории. |
| `ref` | Ветка/тег GitLab; без него используется ref результата поиска. У GitHub поле запрещено: Code Search индексирует default branch. |
| `token_key` | Имя файла/ключа существующего Secret с токеном конкретного источника. |
| `username_key` | Для Confluence Cloud — ключ с email пользователя, тогда применяется Basic email/API-token. Без него используется Bearer PAT для Data Center. |

Confluence Cloud: `api_url: https://company.atlassian.net/wiki/rest/api`, `web_url: https://company.atlassian.net/wiki`. Data Center: соответствующий корень установки с `/rest/api`. Используются CQL и `content/search` с `space`, `version`, `body.storage`; attachments и вложенные URL не скачиваются.

GitHub: `https://api.github.com` и `https://github.com`; для Enterprise задайте свои API/web endpoints. Используется scoped Code Search, затем чтение найденного blob по SHA. Нужен токен с доступом к выбранному репозиторию; предпочтителен fine-grained token с минимальными правами чтения.

GitLab: API root заканчивается на `/api/v4`, web root — адрес экземпляра. Используется поиск `scope=blobs`, затем Repository Files API; сохраняется commit ID прочитанного файла. Подойдёт ограниченный project access token с нужными правами чтения API/репозитория. Если экземпляр не поддерживает поиск blobs, источник будет `unavailable`; это не отсутствие документации. Возможности и ограничения поиска зависят от конфигурации/тарифа GitLab.

Поддерживаются `.md`, `.markdown`, `.txt`, `.rst`, `.adoc`. Фильтр каталогов проверяется перед скачиванием содержимого; результат из чужого repo/space или путь с `..` отбрасывается. Запрос модели преобразуется в литеральные поисковые термины, а ограничения источника формирует сервер. Модель не может поменять hostname, repository, space, токен или URL скачивания.

## Установка в отдельном проверочном окружении

Новый MCP-образ ещё не входит в опубликованный `0.2.0`. Соберите `knowledge-mcp`, `ai-worker`, `backend` и `ui` из одного кандидата; для чистой установки также соберите совместимые `enricher` и `sanitizer`. Например, из корня checkout:

```bash
export INCIDENTGPT_REGISTRY=registry.example.com/your-project
export INCIDENTGPT_IMAGE_TAG="$(git rev-parse --short=12 HEAD)"
for component in knowledge-mcp ai-worker backend ui enricher sanitizer; do
  docker buildx build --platform linux/amd64 \
    -t "${INCIDENTGPT_REGISTRY}/incidentgpt-${component}:${INCIDENTGPT_IMAGE_TAG}" \
    --push "./${component}"
done
```

Платформа должна соответствовать нодам. Замените image repository/tag для всех компонентов в локальных values. Для production нужны проверенные неизменяемые image digests, а не только имя тега.

Создайте namespace стенда (`kubectl create namespace incidentgpt`) и два разных Secret: серверу нужны credentials источников и HMAC Sanitizer, а worker — только MCP client token. Следующий пример подразумевает, что оператор создал файлы credentials в отдельном временном каталоге; не выводите их содержимое в терминал/логи.

```bash
export INCIDENTGPT_MCP_CREDENTIALS=/path/to/private/credential-files
# Создайте случайный MCP token; auth-shared-secret должен совпадать с Sanitizer.
openssl rand -hex 32 > "$INCIDENTGPT_MCP_CREDENTIALS/mcp-server-token"
chmod 600 "$INCIDENTGPT_MCP_CREDENTIALS/"*
kubectl -n incidentgpt create secret generic incidentgpt-knowledge-sources \
  --from-file=mcp-server-token="$INCIDENTGPT_MCP_CREDENTIALS/mcp-server-token" \
  --from-file=auth-shared-secret="$INCIDENTGPT_MCP_CREDENTIALS/auth-shared-secret" \
  --from-file=confluence-token="$INCIDENTGPT_MCP_CREDENTIALS/confluence-token" \
  --from-file=confluence-email="$INCIDENTGPT_MCP_CREDENTIALS/confluence-email" \
  --from-file=github-token="$INCIDENTGPT_MCP_CREDENTIALS/github-token" \
  --from-file=gitlab-token="$INCIDENTGPT_MCP_CREDENTIALS/gitlab-token"
kubectl -n incidentgpt create secret generic incidentgpt-knowledge-client \
  --from-file=mcp-server-token="$INCIDENTGPT_MCP_CREDENTIALS/mcp-server-token"
```

Удалите флаги `--from-file` для неиспользуемых источников. Не переиспользуйте production credentials/Redis на проверочном стенде. Worker по-прежнему отдельно требует обычные LLM, Telegram и Sanitizer credentials: MCP client Secret их не заменяет. Для structured UI нужны действующие backend reader credentials; для durable режима согласуйте service tokens и producers, как описано в [evidence-and-delivery.md](roadmap/evidence-and-delivery.md).

```bash
cp docs/examples/values-mcp-knowledge.yaml values-mcp.local.yaml
# Заполните источники, image repository/tag и существующую конфигурацию приложения.
helm dependency build --skip-refresh deploy/incidentgpt
helm lint deploy/incidentgpt -f values-mcp.local.yaml
helm upgrade --install incidentgpt deploy/incidentgpt \
  -n incidentgpt --create-namespace -f values-mcp.local.yaml --wait --timeout 5m
kubectl -n incidentgpt rollout status deployment/incidentgpt-knowledge-mcp
kubectl -n incidentgpt rollout status deployment/ai-worker
```

Namespace должен существовать до команд создания Secret. Можно поставить сервер отдельно через `knowledge-mcp/chart`: передайте значения без umbrella-префикса `incidentgpt-knowledge-mcp`. Пример структуры — [README компонента](../knowledge-mcp/README.md). Затем в worker задайте `knowledgeMCP.url`, `knowledgeMCP.existingSecret` и `knowledgeMCP.enabled: true`. URL может вести на отдельный сервер с совместимым инструментом/структурой результата, но это не универсальное подключение произвольных MCP-инструментов.

По умолчанию сервер доступен только как ClusterIP: NetworkPolicy допускает AI worker из того же namespace. Доступ к Sanitizer разрешён отдельным правилом на стороне Sanitizer. При межnamespace установке измените namespace/label selectors явно. В кластере с default-deny egress разрешите DNS, Sanitizer и HTTPS API заданных источников по своей политике; универсальных CIDR в чарте нет. TLS проверяется; внутренний CA нужно установить в trust store контейнера, не отключая проверку сертификатов. Внешний доступ требует отдельного TLS ingress/gateway и политики доступа; готового публичного ingress в этом компоненте нет.

## Поведение и проверка

Модель должна поддерживать OpenAI-compatible function calling (`tools`, `tool_calls`, `tool_choice`). В каждой попытке разбора разрешено максимум два read-only поиска и три запроса к модельному провайдеру; повторная доставка durable job может запустить новую попытку. Это увеличивает возможные расходы/задержку относительно одного запроса; `OPENROUTER_MAX_TOKENS` действует на каждый вызов, а общий timeout разбора остаётся ограниченным. Модель может не обращаться к MCP, если данных уже достаточно.

На сервере разрешено до восьми источников и восьми одновременных поисков. Один поиск возвращает максимум пять документов, не более двух от источника. Для Git поиск рассматривает первую страницу из 20 совпадений, для Confluence — до пяти страниц-документов; оставшиеся результаты обозначаются `truncated`. Это bounded keyword retrieval через API провайдеров, без полного клонирования, embedding-индекса или обещания полноты поиска. Документ ограничен 64 KiB, excerpt — 4096 байт. Сервер оставляет отдельное время для финальной санитизации после таймаутов источников.

Каждый запрос проходит Sanitizer до отправки в MCP/API. Документы проходят Sanitizer перед возвратом клиенту и ещё раз перед LLM. При отказе санитизации оригинальный запрос/текст не отправляется дальше. Источник отвечает только на GET; redirects не следуются и credentials не переносятся на другой адрес. MCP требует Bearer token длиной не менее 32 байт; browser Origin отвергается. Это внутренний machine-to-machine доступ, без OAuth discovery или пользовательского делегирования прав Confluence/Git. Все обладатели client token видят общую настроенную область — отдельной изоляции по командам нет.

Structured-анализ сохраняет additive поле `knowledge`: статус, число обращений, excerpts и ссылки с source ID, provider version и временем получения. Ссылки гипотез на документацию идут в `knowledge_reference_ids`; они не принимаются как `supporting_evidence_ids` и не могут подтвердить причину или восстановление. Backend сохраняет metadata в существующей версии анализа; новые Redis key families/миграции не вводятся. В UI «Анализ с доказательствами» доступны источники и выдержки. Markdown/Telegram-разбор получает отдельный список полученных материалов.

Статусы: `not_requested` — модель не искала; `available` — источники ответили; `partial` — часть недоступна/результат неполон; `unavailable` — поиск не удался. Полученная документация остаётся непроверенным справочным материалом; инструкции из неё не исполняются. Ошибка MCP не превращается в утверждение, что подходящих runbooks нет. Модель может закончить разбор с явно недостающей информацией.

Проверяйте запросом про тестовый runbook, которого нет в исходном контексте. Нужен provider, реально запросивший инструмент: обычный успешный анализ без `knowledge.calls` не проверяет интеграцию. Сверьте source URL/version/excerpt, затем отказ credentials/источника/Sanitizer и отсутствие оригинального секрета в LLM/Telegram. `/healthz` и `/readyz` проверяют процесс/конфигурацию, не доступность Confluence/Git, LLM или полноту поиска.

Проверки разработчика: `go test -race ./...` в `knowledge-mcp`, `ai-worker`, `backend`; `bash deploy/test-knowledge-mcp-charts.sh`; UI typecheck/build. Тесты используют реальные MCP SDK client/server и перехваченные API/LLM fixtures. Они не доказывают корректность credentials, TLS, RBAC или доступность ваших реальных систем.

Перед merge проверьте изолированный candidate deployment с целевой конфигурацией, NetworkPolicy, ingress/health и сохранением metadata в Redis; запишите commit/image digests и CI/deployment evidence в PR. Не переносите отладку на production. Для rollback выключите `ai-worker.knowledgeMCP.enabled` и `incidentgpt-knowledge-mcp.enabled`; старые версии анализа остаются читаемыми, additive metadata можно игнорировать. Перезапустите worker/server после ротации MCP/HMAC env Secrets; файловые токены источников читаются при запросе и обновляются через projected Secret volume.

API references: [MCP Streamable HTTP](https://github.com/modelcontextprotocol/go-sdk/blob/main/docs/protocol.md), [OpenRouter tool calling](https://openrouter.ai/docs/guides/features/tool-calling), [Confluence CQL content search](https://developer.atlassian.com/cloud/confluence/rest/v1/api-group-content/#api-wiki-rest-api-content-search-get), [GitHub Code Search](https://docs.github.com/en/rest/search/search#search-code), [GitLab project search](https://docs.gitlab.com/api/search/#search-a-project).
