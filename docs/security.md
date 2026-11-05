# IncidentGPT Security

IncidentGPT handles operational context. Treat it as part of your incident-management surface.

## Secrets Management

- Store `OPENROUTER_API_KEY` and `TELEGRAM_BOT_TOKEN` in Kubernetes Secrets.
- Do not commit real tokens to Git, Helm values or documentation.
- Prefer the `secrets.existingSecret` values API for `ai-worker`.
- Recommended improvement: add secretKeyRef support for Enricher `REDIS_PASSWORD`.

## LLM Data Exposure

The LLM prompt may include labels, annotations, Prometheus samples, Kubernetes context, events, runbook URLs and diagnostic hints.

Do not send:

- passwords or API keys;
- Kubernetes Secret values;
- personal data;
- full configuration dumps;
- internal tokens or credentials;
- sensitive customer identifiers unless your policy allows it.

## Telegram Permissions

- Use a dedicated bot.
- Add the bot only to the required channel and discussion group.
- Grant the minimum rights needed to send messages.
- Rotate the token if it appears in logs or shell history.

## RBAC

The Enricher chart creates cluster-wide read permissions for pods, namespaces, nodes, events, deployments and replicasets. Review this scope before production rollout.

Recommended improvements:

- namespace-scoped mode for smaller clusters;
- optional RBAC toggles in chart values;
- audit policy review for Kubernetes API access.

## NetworkPolicy Recommendations

Restrict egress and ingress:

- Alertmanager to Enricher on port `9099`;
- Enricher to Prometheus, Kubernetes API, Redis and AI Worker;
- AI Worker to Telegram API and OpenRouter or your compatible provider.

## Container Security

- Use non-latest image tags.
- Add resource requests and limits.
- Run vulnerability scans in CI.
- Consider read-only root filesystem and non-root users after validating the Dockerfiles.

## Logging Policy

Avoid enabling debug payload logging in production:

- `DEBUG_PROMPT=1` logs LLM prompts.
- `DEBUG_OR_PAYLOAD=1` logs OpenRouter request payloads.
- `DEBUG_ENRICHED=1` logs enriched alert payloads.

Never print token values with `echo`.

## Dependency Updates

Track Go modules, npm dependencies in `website/`, Docker base images and GitHub Actions versions. Review changes before production rollout.

## Responsible Disclosure

If you discover a security issue, avoid posting exploit details publicly. Contact the repository owner privately and include a minimal reproduction, affected versions and suggested mitigation.

## Investigation, knowledge and report boundaries

Новые API используют сервисные Bearer-токены или собственную проверку Basic-auth
backend (`TIMELINE_READ_*`), независимо от nginx и `X-Auth-User`. Старые PATCH
status/assignee и dashboard/analytics сохраняют защиту nginx auth_request и NetworkPolicy.
Для дополнительной проверки этих старых API включите `LEGACY_API_AUTH_REQUIRED=true`
и задайте `changes.existingSecret` либо `readerAuth.existingSecret`. Отсутствие
credentials в этом режиме даёт 401, автоматического ослабления защиты нет.
Новые reports/lifecycle/analysis API всегда требуют backend credentials.

Страница входа использует учётную запись backend и серверную сессию. Нужен
Secret с исходными username/password; старый хеш htpasswd не подходит. Чарт не копирует
admin/admin в backend и не создаёт reader Secret автоматически. Выбранный Secret
обязателен: ошибка имени/ключа блокирует запуск, вместо скрытого 401 после установки.
Браузерные изменения проверяют Origin. Для прокси, меняющего Host, задайте полные
доверенные адреса в `ALLOWED_ORIGINS` через запятую, например
`http://127.0.0.1:8088,https://incident.example`. Wildcard, path и произвольные
forwarded-заголовки не дают доступа. Для новых API backend проверяет Basic principal
либо восстанавливает его из действующей серверной сессии.
Общая administrator-роль не обеспечивает разделение команд. Backend/Redis/diagnostics
должны оставаться приватными под NetworkPolicy.

Change collectors read allowlisted deployment/ReplicaSet metadata, not Secret or
ConfigMap values. Graph observations use actual ownership/placement metadata and
cannot declare application dependencies. Routing/resource identities must survive
sanitization unchanged; incompatible pseudonymization policies fail closed rather
than causing cross-resource associations. Diagnostic operations are typed reads
with backend-bound resources, explicit namespace/cluster authorization, timeout,
call/concurrency/output budgets and no Kubernetes writes, exec, arbitrary queries
or URL destinations. Untrusted logs/runbooks/questions cannot choose tools.

Reports snapshot only incident-owned evidence, stored analysis/changes and trusted
human facts. Snapshot, generation and edits pass sanitizer before persistence.
Exports share incident authentication, contain sanitized Markdown and use attachment,
no-store/nosniff headers. Preview skips raw HTML/images and rejects unsafe links.
Unknown impact/cause/recovery is not invented. Human edits retain actor/revision and
regeneration cannot overwrite prior edits. Handoff never sends Slack/email/Telegram
or opens external tickets. Download/copy is the explicit user workflow.

Redis incidents, jobs, resolution/composition/lifecycle audits require backup and an
operator retention policy. Graph/change data defaults to 30 days; report records to
90 days, while captured report job input follows non-expiring durable job history.
Deletion/archival must consider both. Read the [workflow-specific limits and rollback](roadmap/postmortem-and-handoff.md).

## Граф и ручное изменение состава

`TIMELINE_READ_*` — историческое имя общей administrator-пары, а не отдельная
read-only роль. Она авторизует новые graph/regroup/diagnostics API; разделения
операторов и tenancy пока нет. Нельзя выдавать эти credentials как read-only.
Имя в audit берётся из проверенного Basic principal, включая восстановленный из сессии. `X-Auth-User` от прямого
клиента не является доказательством другой личности и не может подменять actor.

Graph/regroup требуют JSON и проверяют browser Origin; произвольный сторонний
`text/plain` POST отклоняется до мутации. При замене Host доверенные адреса задаются
явно в `ALLOWED_ORIGINS`. Для authenticated browser-запросов нет wildcard CORS.
Чтение выключенного графа возвращает только `enabled:false` и пустые списки,
без новых credentials/metadata; включённый граф требует backend Basic-auth.
Подробные флаги, сроки хранения и rollout: [руководство](roadmap/dependency-correlation.md).

## MCP и внешняя база знаний

Источники включает оператор. Он ограничивает Confluence spaces и repository/paths и выдаёт серверу минимальные права чтения; модели доступны только короткие поисковые запросы. Worker хранит отдельный MCP client token и не получает credentials источников. MCP принимает внутренний Bearer доступ, отвергает browser Origin, а NetworkPolicy ограничивает вход AI worker того же namespace. Публичного ingress/OAuth discovery по умолчанию нет; token открывает общую настроенную область без изоляции по пользователям.

Query и документы проходят fail-closed Sanitizer, ссылки/версии проверяются, произвольные адреса модели не запрашиваются, provider redirects не следуются. Документы не исполняются и не считаются подтверждением причины/восстановления; ссылки на них не проходят как incident evidence IDs. Бюджеты запросов, TLS/CA, дополнительные права egress и ротация Secrets описаны в [руководстве MCP](mcp-knowledge.md).

## Browser sessions

The public sign-in shell contains no operational data. Nginx authenticates all API
and WebSocket paths through a backend subrequest, including API URLs ending in
static-file extensions. Passwords are never stored in browser storage or cookies.
Session cookies are opaque, HttpOnly, Secure by default, SameSite=Strict and expire
after eight hours. Login/logout require a trusted browser Origin. Logout deletes
the server session; credential rotation invalidates existing sessions. Redis stores
only token hashes and expires sessions automatically. The bounded in-memory fallback
is intended for one backend process. Keep Redis private and persistent across replicas.
Session endpoints return no-store responses without a browser Basic challenge.
See [deployment and upgrade requirements](web-ui.md#доступ).
