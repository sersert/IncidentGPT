# IncidentGPT Knowledge MCP

Отдельный read-only MCP-сервер для Confluence Cloud/Data Center, GitHub/Enterprise и GitLab. AI worker сам решает, когда вызвать `search_knowledge`. Все разрешённые источники задаёт оператор; модель не получает credentials или инструменты записи.

Полная установка, ограничения, автоматический AI-разбор и rollback: [docs/mcp-knowledge.md](../docs/mcp-knowledge.md).

Для standalone Helm chart подготовьте файл `knowledge-server-values.local.yaml` такой структуры:

```yaml
secrets:
  existingSecret: incidentgpt-knowledge-sources
sanitizerURL: http://incidentgpt-sanitizer:8080
image:
  repository: registry.example.com/incidentgpt-knowledge-mcp
  tag: YOUR_CANDIDATE_COMMIT
sources:
  - id: github-runbooks
    kind: github
    api_url: https://api.github.com
    web_url: https://github.com
    repository: your-org/runbooks
    paths: [docs, runbooks]
    token_key: github-token
```

```bash
helm upgrade --install incidentgpt-knowledge-mcp ./knowledge-mcp/chart \
  -n incidentgpt -f knowledge-server-values.local.yaml --wait
```

Для локального запуска без Kubernetes задайте `KNOWLEDGE_SOURCES_FILE` (JSON-массив sources), `KNOWLEDGE_CREDENTIALS_DIR` (каталог файлов token_key/username_key), `MCP_SERVER_TOKEN`, `SANITIZER_URL`, `SANITIZER_AUTH_SHARED_SECRET`; `LISTEN_ADDR` по умолчанию `:8080`. JSON config не содержит токены. При локальном доступе задайте `LISTEN_ADDR=127.0.0.1:8080`; публичный bind требует отдельной сетевой защиты. Endpoint `/mcp` использует Bearer; `/healthz` и `/readyz` не требуют credentials.

```bash
cd knowledge-mcp
go test -race ./...
go build -o /tmp/incidentgpt-knowledge-mcp .
```
