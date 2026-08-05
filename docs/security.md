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
