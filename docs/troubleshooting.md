# IncidentGPT Troubleshooting

## Alertmanager Does Not Send Webhook

Check:

```bash
kubectl get svc -n incidentgpt
kubectl get endpoints -n incidentgpt
kubectl logs -n monitoring alertmanager-<pod>
kubectl logs -n incidentgpt deploy/incidentgpt-enricher
```

Verify receiver name, service DNS, namespace, port `9099`, path `/alert`, NetworkPolicy and endpoint availability.

## PrometheusRule Does Not Fire

```bash
kubectl get prometheusrules --all-namespaces
kubectl get prometheuses --all-namespaces
kubectl describe prometheusrule <name> -n <namespace>
```

Typical causes: wrong `release` label, rule selector mismatch, PromQL returning no series, long `for`, or the rule being created in the wrong namespace.

## Enricher Receives No Prometheus Metrics

If the container has no `wget`, use a debug pod:

```bash
kubectl run network-debug \
  --rm -it \
  --restart=Never \
  --image=curlimages/curl \
  -n incidentgpt \
  -- curl -v http://<prometheus-service>:9090/-/ready
```

Check URL, namespace, port, NetworkPolicy, service endpoints, PromQL and any authentication proxy.

## No Kubernetes Context

```bash
kubectl auth can-i list pods \
  --as=system:serviceaccount:incidentgpt:<service-account> \
  --all-namespaces

kubectl auth can-i list events \
  --as=system:serviceaccount:incidentgpt:<service-account> \
  --all-namespaces
```

The Enricher chart grants cluster-wide reads for pods, namespaces, nodes, events, deployments and replicasets.

## Redis Connection Refused

```bash
kubectl get svc,endpoints,pod -n incidentgpt | grep redis

kubectl run redis-debug \
  --rm -it \
  --restart=Never \
  --image=redis:7-alpine \
  -n incidentgpt \
  -- redis-cli -h redis-master.incidentgpt.svc.cluster.local ping
```

Expected response:

```text
PONG
```

## Alerts Are Not Grouped

Check:

- the same `namespace`;
- fingerprint or stable labels;
- Redis availability;
- whether alerts arrive inside `CORR_SETTLE`;
- Enricher logs for `ALERT_BUFFERED`;
- Enricher logs for `GROUP_SENT`.

`CORR_SETTLE` is the debounce wait before sending a group. `CORR_WINDOW` is the Redis TTL for the group.

## Telegram Returns An Error

Check bot token, channel ID, discussion group ID, administrator rights, `-100` prefix, linked discussion group and Telegram API errors in ai-worker logs.

Do not print the token in commands or screenshots.

## OpenRouter Returns 401

Possible causes:

- invalid API key;
- Secret not mounted;
- whitespace or newline in value;
- wrong endpoint;
- key from a different provider;
- environment variable missing from the pod.

Safe check:

```bash
kubectl exec -n incidentgpt deploy/ai-worker -- \
  sh -c 'test -n "$OPENROUTER_API_KEY" && echo configured || echo missing'
```

Never run:

```bash
echo "$OPENROUTER_API_KEY"
```
