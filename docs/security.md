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

New feature ingestion uses explicit service Bearer tokens; UI mutations/reads verify
backend Basic credentials, independently of nginx and untrusted actor headers.
Status/assignee PATCH also verifies Basic identity. Browser mutations verify that Origin matches the UI host; nginx and the development proxy preserve Host, including its port. The umbrella development install shares its existing UI account through a generated reader Secret; installations with external htpasswd must supply matching backend reader credentials. The current deployment has a
shared administrator scope; cluster-aware IDs and query scope do not constitute
team tenancy. Keep backend/Redis/internal diagnostics private under NetworkPolicy.

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
