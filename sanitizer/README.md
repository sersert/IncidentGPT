# IncidentGPT Sanitizer

`incidentgpt-sanitizer` is a standalone Go service that acts as the data protection boundary between IncidentGPT's trusted Kubernetes-side components and less trusted external destinations such as LLM APIs, Telegram, logs, and webhooks.

## API

- `POST /v1/sanitize` sanitizes structured JSON payloads.
- `POST /v1/sanitize/text` sanitizes plain text.
- `POST /v1/inspect` detects sensitive data without returning matched values.
- `GET /healthz`, `GET /readyz`, `GET /metrics`, `GET /version` expose operational status.

All `/v1/*` requests require HMAC headers:

```text
X-IncidentGPT-Timestamp
X-IncidentGPT-Request-ID
X-IncidentGPT-Signature
```

Signature base:

```text
timestamp + "\n" + requestID + "\n" + method + "\n" + path + "\n" + sha256(body)
```

## Configuration

Important environment variables:

- `SANITIZER_AUTH_SHARED_SECRET` HMAC key, required.
- `SANITIZER_HASH_KEY` HMAC key for stable resource pseudonyms.
- `SANITIZER_FAIL_CLOSED=true` blocks unsafe processing on errors.
- `SANITIZER_MAX_INPUT_BYTES=1048576` limits request size.
- `SANITIZER_MAX_DEPTH=20` limits recursive JSON traversal.
- `SANITIZER_CUSTOM_PATTERNS=[]` adds bounded custom regexp rules.
- `SANITIZER_IP_MODE=none|partial|full` controls IP masking.

## Local Test

```bash
SANITIZER_AUTH_SHARED_SECRET=dev-secret go test ./...
SANITIZER_AUTH_SHARED_SECRET=dev-secret go run ./cmd/sanitizer
```

## Kubernetes

The Helm chart creates a `ClusterIP` service, Deployment, ServiceAccount, NetworkPolicy, PodDisruptionBudget, and optional HPA/ServiceMonitor. Do not expose this service through Ingress, NodePort, or LoadBalancer.
