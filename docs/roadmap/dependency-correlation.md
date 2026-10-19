# Dependency correlation and manual composition

The graph stores cluster/namespace/kind/name identities and directed relationships.
Kubernetes metadata produces ownership (Deployment → ReplicaSet → Pod) and
placement (Pod → Node), never application dependencies. Enable the optional
bounded metadata submission with `TOPOLOGY_OBSERVATIONS_ENABLED=true` and
`TOPOLOGY_BACKEND_URL=http://backend:8080/api/v1/services/observations`. It reuses
actual GET results, four concurrent submissions and a three-second deadline.
Service authentication and backend sanitization are required. Failed submissions
leave topology unknown; they do not block basic alert ingestion beyond the deadline.

Declared application dependencies use `docs/fixtures/service-dependencies.json`.
PUT `/api/v1/services/graph` with backend Basic credentials replaces the cluster's
previous declared edges; POST `/services/observations` requires the analysis service
token. GET `/services/graph?incident_id=...` and `/services/{node-id}` require Basic
authentication. Nodes explicitly have unknown health. Payloads are at most 32 KiB,
200 nodes and 400 edges. Indexed lookups expose truncation and stale omissions.
Metadata edges expire for correlation after 15 minutes; declarations after seven
days. Stored records/provenance expire after 30 days. A configured
`SERVICE_GRAPH_FILE=/etc/incidentgpt/topology/dependencies.json` is refreshed every
ten minutes. Set `incidentgpt-backend.serviceGraph.existingConfigMap` in umbrella values to mount that file;
set the environment path separately. Invalid/unavailable configuration produces no
fresh graph, so investigate file/sanitizer availability when the view is empty.
No tracing or graph database is required.

`DEPENDENCY_CORRELATION_ENABLED=true` requires durable correlation. With known
cluster/workload, one fresh declared dependency produces the group key and wording
“shared dependency suspected.” Two workloads can therefore share a group across
namespaces. Multiple distinct dependencies fall back to workload grouping. Only
one outgoing hop, at most 16 candidates, and a 500 ms lookup budget are used.
Cycles are not traversed; no causal or healthy state is inferred. Missing workload
uses pod/fingerprint; missing cluster isolates source observations. Node-only alerts can group by the same reported cluster/node; workload or pod alerts are never merged with them solely by placement. Placement alone does not merge applications. The existing settle deadline defines the time window;
Redis buffers cap batches at 50 alerts. Backend durable ingestion also refuses incident growth beyond 50 alerts; it returns a visible 503 and leaves the batch recoverable for bounded retries/dead-letter inspection. Separate or close a full group before retrying. Closed firing fingerprints can recur as a new incident episode. Standalone alert bindings are persisted before accepting that episode. Limit/lookup failures explicitly degrade.
The deployment shares administrator visibility; this is not tenant isolation.

POST `/incidents/{id}/regroup` with `{ "alert_id": "...",
"target_incident_id": "..." }` moves one alert to an active same-cluster incident.
Omit the target to split into a new stable incident. There is no bulk merge command;
repeat bounded moves when consolidation is appropriate. IDs and old incident URLs
remain valid. `history_sources` references both records; original immutable evidence
and analyses retain their original incident identity and are read on that page.
Audit records retain trusted Basic actor, UTC time and compositions. Durable ingestion partitions moved fingerprints before scheduling, with idempotent child batches, so a later automatic batch cannot undo human separation. Current summaries
and latest/requested pointers are cleared, fencing late results; history returns the
current composition hash for comparison. Request refresh to interpret the new group.

Durable ingestion and regrouping share a Redis composition lock (30-second lease),
with 15/10-second request budgets; atomic compare-and-set rejects concurrent changes.
This serializes durable ingestion globally, reducing peak throughput deliberately.
Legacy concurrent writers are protected against lost updates during the move by CAS,
but legacy grouping must be drained before enabling manual correction. Single-primary
Redis/AOF/backup assumptions from evidence-and-delivery.md apply. Audit and incident
records have no automatic expiry; include them in backup/retention policies.

Rollout: load and inspect topology first, drain legacy batches, enable durable mode,
then opt into correlation for a limited environment. No topology backfill is invented.
Disable new correlation/observations before rollback; drain durable batches before
reverting their consumer. Human moves and historical source links remain additive.
