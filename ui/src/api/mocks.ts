import type {
  Incident,
  Alert,
  IncidentDetail,
  DashboardStats,
  AlertVolumeBucket,
  Prediction,
  PredictionSummary,
  LicenseInfo,
  AIAnalysis,
  ChainEvent,
  LogEntry,
  IncidentContext,
  MetricSeries,
  AnalyticsSummary,
  MTTRDataPoint,
  AnalyticsQuality,
  KnowledgeResult,
  ServiceNode,
  ServiceEdge,
  ServiceDetail,
  Integration,
  LLMConfig,
  PostMortem,
  PredictionTrend,
} from './types.ts'

const delay = (ms: number) => new Promise((r) => setTimeout(r, ms))
const rand = (min: number, max: number) => Math.floor(Math.random() * (max - min + 1)) + min

export async function mock<T>(data: T): Promise<T> {
  await delay(rand(200, 500))
  return data
}

export const mockIncidents: Incident[] = [
  {
    id: 'INC-2026-0142',
    title: 'TargetDown — 22 связанных алерта, vpn-stage',
    severity: 'critical',
    status: 'active',
    namespace: 'vpn-stage',
    source: 'prometheus',
    created_at: '2026-02-19T14:22:00Z',
    updated_at: '2026-02-19T14:35:00Z',
    analysis_summary: 'Сбой сетевой доступности к VPN-подам — Prometheus не скрапит метрики',
    alert_count: 22,
    discarded_count: 2,
    chain_length: 5,
    log_errors: 23,
    confidence: 87,
    duration_sec: null,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0141',
    title: 'node-3 /data disk usage >90%',
    severity: 'warning',
    status: 'active',
    namespace: 'storage',
    source: 'prometheus',
    created_at: '2026-02-19T12:10:00Z',
    updated_at: '2026-02-19T14:00:00Z',
    analysis_summary: 'Log rotation misconfigured, /data/tmp growing at 1.2GB/hour',
    alert_count: 3,
    discarded_count: 0,
    chain_length: 3,
    log_errors: 5,
    confidence: 92,
    duration_sec: null,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0140',
    title: 'redis-cluster latency spike >500ms',
    severity: 'warning',
    status: 'acknowledged',
    namespace: 'cache',
    source: 'prometheus',
    created_at: '2026-02-19T10:45:00Z',
    updated_at: '2026-02-19T11:30:00Z',
    analysis_summary: 'Slow queries from checkout-svc causing connection pool exhaustion',
    alert_count: 4,
    discarded_count: 0,
    chain_length: 4,
    log_errors: 12,
    confidence: 78,
    duration_sec: null,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0139',
    title: 'api-gateway 5xx rate >5% on /v2/orders',
    severity: 'critical',
    status: 'resolved',
    namespace: 'default',
    source: 'prometheus',
    created_at: '2026-02-18T22:15:00Z',
    updated_at: '2026-02-19T00:30:00Z',
    analysis_summary: 'Upstream payment-svc timeout causing cascading 502s',
    alert_count: 6,
    discarded_count: 0,
    chain_length: 6,
    log_errors: 142,
    confidence: 94,
    duration_sec: 8100,
    has_postmortem: true,
  },
  {
    id: 'INC-2026-0138',
    title: 'CrashLoopBackOff — checkout-svc v2.3.1',
    severity: 'critical',
    status: 'resolved',
    namespace: 'checkout',
    source: 'kubernetes',
    created_at: '2026-02-18T16:00:00Z',
    updated_at: '2026-02-18T17:20:00Z',
    analysis_summary: 'Missing env var DATABASE_URL after ArgoCD deploy v2.3.1',
    alert_count: 4,
    discarded_count: 0,
    chain_length: 4,
    log_errors: 67,
    confidence: 96,
    duration_sec: 4800,
    has_postmortem: true,
  },
  {
    id: 'INC-2026-0137',
    title: 'PostgreSQL replication lag >30s',
    severity: 'warning',
    status: 'resolved',
    namespace: 'databases',
    source: 'zabbix',
    created_at: '2026-02-18T09:30:00Z',
    updated_at: '2026-02-18T10:15:00Z',
    analysis_summary: 'Heavy write load from batch-import cronjob without connection pooling',
    alert_count: 3,
    discarded_count: 0,
    chain_length: 3,
    log_errors: 8,
    confidence: 85,
    duration_sec: 2700,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0136',
    title: 'Loki ingestion rate drop — logs missing',
    severity: 'info',
    status: 'resolved',
    namespace: 'monitoring',
    source: 'loki',
    created_at: '2026-02-17T20:00:00Z',
    updated_at: '2026-02-17T22:30:00Z',
    analysis_summary: 'Loki distributor OOM due to high cardinality labels from new service',
    alert_count: 2,
    discarded_count: 0,
    chain_length: 2,
    log_errors: 3,
    confidence: 72,
    duration_sec: 9000,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0135',
    title: 'cert-manager renewal failure — TLS expired',
    severity: 'warning',
    status: 'resolved',
    namespace: 'cert-manager',
    source: 'kubernetes',
    created_at: '2026-02-17T08:00:00Z',
    updated_at: '2026-02-17T09:45:00Z',
    analysis_summary: 'ACME challenge failed — DNS propagation timeout for *.example.io',
    alert_count: 3,
    discarded_count: 0,
    chain_length: 3,
    log_errors: 15,
    confidence: 91,
    duration_sec: 6300,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0134',
    title: 'ArgoCD sync failed — kustomization error',
    severity: 'info',
    status: 'resolved',
    namespace: 'argocd',
    source: 'argocd',
    created_at: '2026-02-16T14:20:00Z',
    updated_at: '2026-02-16T15:00:00Z',
    analysis_summary: 'Invalid resource reference in kustomization.yaml after merge',
    alert_count: 2,
    discarded_count: 0,
    chain_length: 2,
    log_errors: 4,
    confidence: 88,
    duration_sec: 2400,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0133',
    title: 'High CPU on worker-pool nodes (>95%)',
    severity: 'info',
    status: 'muted',
    namespace: 'batch',
    source: 'prometheus',
    created_at: '2026-02-16T10:00:00Z',
    updated_at: '2026-02-16T10:30:00Z',
    analysis_summary: 'Expected behavior during nightly ML training job',
    alert_count: 1,
    discarded_count: 0,
    chain_length: 1,
    log_errors: 0,
    confidence: 95,
    duration_sec: 1800,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0132',
    title: 'Kafka consumer lag >100k — notifications delayed',
    severity: 'warning',
    status: 'resolved',
    namespace: 'messaging',
    source: 'prometheus',
    created_at: '2026-02-15T18:30:00Z',
    updated_at: '2026-02-15T20:00:00Z',
    analysis_summary: 'Consumer group rebalance after scaling event, partition reassignment stalled',
    alert_count: 4,
    discarded_count: 0,
    chain_length: 4,
    log_errors: 22,
    confidence: 81,
    duration_sec: 5400,
    has_postmortem: false,
  },
  {
    id: 'INC-2026-0131',
    title: 'Ingress NGINX 499 errors — client timeouts',
    severity: 'info',
    status: 'resolved',
    namespace: 'ingress',
    source: 'prometheus',
    created_at: '2026-02-15T11:00:00Z',
    updated_at: '2026-02-15T12:30:00Z',
    analysis_summary: 'Upstream keepalive too low for traffic spike, connection churn',
    alert_count: 2,
    discarded_count: 0,
    chain_length: 2,
    log_errors: 34,
    confidence: 76,
    duration_sec: 5400,
    has_postmortem: false,
  },
]


// Группа из реального вывода: 8 алертов TargetDown, два из которых разбор
// отбросил — у них starts_at на две недели раньше, они к текущему сбою не относятся.
const vpnTargets = [
  'white-vpn-stage-mtg-sergey', 'white-vpn-marzban-exporter', 'white-vpn-stage-mtg-oksana',
  'white-vpn-1-node-exporter', 'white-vpn-stage-mtg-diana', 'white-vpn-stage-mtg-valera',
  'white-vpn-stage-mtg-anton', 'white-vpn-2-node-exporter', 'white-vpn-stage-mtg-irina',
  'white-vpn-3-node-exporter', 'white-vpn-stage-mtg-pavel', 'white-vpn-4-node-exporter',
  'white-vpn-stage-mtg-nikita', 'white-vpn-5-node-exporter', 'white-vpn-stage-mtg-olga',
  'white-vpn-6-node-exporter', 'white-vpn-stage-mtg-roman', 'white-vpn-7-node-exporter',
  'white-vpn-stage-mtg-timur', 'white-vpn-8-node-exporter',
]
const staleTargets = ['botserver-node-exporter', 'white-vpn-lte1-node-exporter']

export const mockAlerts: Alert[] = [
  ...vpnTargets.map((job, i) => ({
    id: `ALT-2026-0${100 + i}`,
    fingerprint: `fp-vpn-${i}`,
    alertname: 'TargetDown',
    severity: 'warning' as const,
    status: 'firing' as const,
    namespace: 'vpn-stage',
    source: 'prometheus',
    starts_at: '2026-07-29T11:43:34Z',
    ends_at: '',
    summary: 'One or more targets are unreachable',
    description: `job=${job}`,
    labels: { alertname: 'TargetDown', job, severity: 'warning' },
    received_at: '2026-07-29T11:44:02Z',
    incident_id: 'INC-2026-0142',
    discarded: false,
  })),
  ...staleTargets.map((job, i) => ({
    id: `ALT-2026-0${200 + i}`,
    fingerprint: `fp-stale-${i}`,
    alertname: 'TargetDown',
    severity: 'info' as const,
    status: 'firing' as const,
    namespace: 'vpn-stage',
    source: 'prometheus',
    starts_at: '2026-07-14T17:57:04Z',
    ends_at: '',
    summary: 'One or more targets are unreachable',
    description: `job=${job}`,
    labels: { alertname: 'TargetDown', job, severity: 'warning' },
    received_at: '2026-07-29T11:44:02Z',
    incident_id: 'INC-2026-0142',
    discarded: true,
    discard_reason: 'starts_at на две недели раньше остальных — затянувшийся алерт, к сбою не относится',
  })),
  {
    id: 'ALT-2026-0301',
    fingerprint: 'fp-loose-1',
    alertname: 'KubePodNotReady',
    severity: 'info',
    status: 'firing',
    namespace: 'monitoring',
    source: 'prometheus',
    starts_at: '2026-07-29T12:10:00Z',
    ends_at: '',
    summary: 'Pod has been in a non-ready state for longer than 15 minutes',
    description: 'pod=grafana-7d9f4b',
    labels: { alertname: 'KubePodNotReady', pod: 'grafana-7d9f4b', severity: 'warning' },
    received_at: '2026-07-29T12:10:30Z',
    incident_id: null,
    discarded: false,
  },
]

const mockChain: ChainEvent[] = [
  {
    id: 'evt-1',
    type: 'deploy',
    source: 'argocd',
    timestamp: '2026-02-19T14:10:00Z',
    title: 'Deploy payment-svc v2.4.0',
    data: { commit: 'a3f8c2d', image: 'payment-svc:v2.4.0' },
    is_root_cause: true,
  },
  {
    id: 'evt-2',
    type: 'k8s_event',
    source: 'kubernetes',
    timestamp: '2026-02-19T14:15:00Z',
    title: 'Pod payment-svc-7f8b9c-x2k4j OOMKilled',
    data: { reason: 'OOMKilled', exit_code: 137, memory_limit: '512Mi' },
    is_root_cause: false,
  },
  {
    id: 'evt-3',
    type: 'k8s_event',
    source: 'kubernetes',
    timestamp: '2026-02-19T14:18:00Z',
    title: 'Pod restarted (attempt 2/5)',
    data: { restart_count: 2 },
    is_root_cause: false,
  },
  {
    id: 'evt-4',
    type: 'prometheus_alert',
    source: 'prometheus',
    timestamp: '2026-02-19T14:22:00Z',
    title: 'KubePodCrashLooping firing',
    data: { alertname: 'KubePodCrashLooping', pod: 'payment-svc-7f8b9c-x2k4j', severity: 'critical' },
    is_root_cause: false,
  },
  {
    id: 'evt-5',
    type: 'log_event',
    source: 'loki',
    timestamp: '2026-02-19T14:25:00Z',
    title: 'Fatal: runtime out of memory',
    data: { stream: 'stderr', message: 'runtime: out of memory: cannot allocate 67108864-byte block' },
    is_root_cause: false,
  },
]

const mockAnalysis: AIAnalysis = {
  root_cause:
    'Memory leak in the payment processing goroutine introduced in v2.4.0. The `processPayment()` function allocates a large buffer for each transaction but fails to release it after the context deadline. Under load (>100 req/s), memory grows linearly until the 512Mi limit is hit, triggering OOMKill.\n\nThe leak was introduced in commit `a3f8c2d` which changed the buffer allocation strategy from pooled to per-request.',
  confidence: 87,
  recommendations: [
    {
      title: 'Restart with increased memory limit',
      description: 'Temporarily increase memory limit to 1Gi to stabilize the pod while a fix is deployed.',
      command: 'kubectl set resources deployment/payment-svc -n payments --limits=memory=1Gi',
      category: 'immediate',
    },
    {
      title: 'Rollback to v2.3.9',
      description: 'Revert to the previous version which did not have the memory leak.',
      command: 'argocd app rollback payment-svc --revision v2.3.9',
      category: 'fix',
    },
    {
      title: 'Add memory profiling to CI',
      description:
        'Add a benchmark test that monitors memory allocation in the payment processing pipeline to catch leaks before deploy.',
      command: null,
      category: 'prevent',
    },
  ],
  similar_incidents: [
    {
      id: 'INC-2026-0098',
      title: 'payment-svc OOMKilled after v2.2.0 deploy',
      date: '2026-01-15',
      resolution: 'Rolled back to v2.1.9, fixed buffer pool in v2.2.1',
      similarity: 92,
    },
    {
      id: 'INC-2025-0412',
      title: 'checkout-svc memory leak in gRPC handler',
      date: '2025-11-20',
      resolution: 'Fixed connection pool leak, added memory limit alerts',
      similarity: 78,
    },
  ],
  discarded_noise: [
    {
      alertname: 'TargetDown — botserver-node-exporter',
      reason: 'starts_at 2026-07-14, на две недели раньше остальных — затянувшийся алерт',
    },
    {
      alertname: 'TargetDown — white-vpn-lte1-node-exporter',
      reason: 'starts_at 2026-07-14, к текущему сбою не относится',
    },
  ],
  generated_at: '2026-02-19T14:28:00Z',
}

const mockLogs: LogEntry[] = [
  { timestamp: '2026-02-19T14:14:58Z', level: 'INFO', message: 'Starting payment-svc v2.4.0', is_stacktrace: false },
  {
    timestamp: '2026-02-19T14:15:01Z',
    level: 'INFO',
    message: 'Connected to PostgreSQL at postgres.payments.svc:5432',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:15:02Z',
    level: 'INFO',
    message: 'Listening on :8080',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:18:30Z',
    level: 'WARN',
    message: 'Memory usage at 80% (410Mi/512Mi)',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:20:12Z',
    level: 'WARN',
    message: 'GC pressure increasing, pause time 120ms',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:21:45Z',
    level: 'ERROR',
    message: 'Failed to allocate buffer: runtime: out of memory',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:21:45Z',
    level: 'FATAL',
    message:
      'goroutine 1 [running]:\nruntime.throw({0x1a2b3c, 0x20})\n\truntime/panic.go:1077\nruntime.mallocgc(0x4000000, 0x0, true)\n\truntime/malloc.go:1236\nmain.processPayment(0xc0001a2000)\n\t/app/handler.go:142',
    is_stacktrace: true,
  },
  {
    timestamp: '2026-02-19T14:22:00Z',
    level: 'ERROR',
    message: 'Container killed: OOMKilled (exit code 137)',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:22:05Z',
    level: 'INFO',
    message: 'Starting payment-svc v2.4.0 (restart 2)',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:25:30Z',
    level: 'WARN',
    message: 'Memory usage at 80% (410Mi/512Mi)',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:27:10Z',
    level: 'ERROR',
    message: 'Failed to allocate buffer: runtime: out of memory',
    is_stacktrace: false,
  },
  {
    timestamp: '2026-02-19T14:27:10Z',
    level: 'FATAL',
    message:
      'goroutine 1 [running]:\nruntime.throw({0x1a2b3c, 0x20})\n\truntime/panic.go:1077\nruntime.mallocgc(0x4000000, 0x0, true)\n\truntime/malloc.go:1236\nmain.processPayment(0xc0001a2000)\n\t/app/handler.go:142',
    is_stacktrace: true,
  },
]

const mockContext: IncidentContext = {
  namespace: 'payments',
  pod: 'payment-svc-7f8b9c-x2k4j',
  pod_status: 'CrashLoopBackOff',
  node: 'worker-2',
  node_ram: '16Gi',
  node_cpu: '8 cores',
  last_deploy_version: 'v2.4.0',
  last_deploy_at: '2026-02-19T14:10:00Z',
  last_deploy_source: 'ArgoCD',
  labels: { app: 'payment-svc', team: 'payments', tier: 'backend', env: 'production' },
}

const mockMetrics: MetricSeries[] = [
  {
    name: 'Memory Usage',
    unit: 'Mi',
    incident_start: '2026-02-19T14:22:00Z',
    data: Array.from({ length: 30 }, (_, i) => ({
      timestamp: new Date(Date.parse('2026-02-19T14:07:00Z') + i * 60000).toISOString(),
      value: 200 + Math.min(i * 12 + rand(-5, 5), 512),
    })),
  },
  {
    name: 'CPU Usage',
    unit: '%',
    incident_start: '2026-02-19T14:22:00Z',
    data: Array.from({ length: 30 }, (_, i) => ({
      timestamp: new Date(Date.parse('2026-02-19T14:07:00Z') + i * 60000).toISOString(),
      value: 25 + Math.min(i * 2.5 + rand(-3, 3), 98),
    })),
  },
  {
    name: 'Request Rate',
    unit: 'req/s',
    incident_start: '2026-02-19T14:22:00Z',
    data: Array.from({ length: 30 }, (_, i) => ({
      timestamp: new Date(Date.parse('2026-02-19T14:07:00Z') + i * 60000).toISOString(),
      value: i < 15 ? 120 + rand(-10, 10) : 40 + rand(-20, 20),
    })),
  },
]

export const mockIncidentDetail: IncidentDetail = {
  ...mockIncidents[0],
  alerts: mockAlerts.filter((a) => a.incident_id === 'INC-2026-0142'),
  chain: mockChain,
  root_cause_event_id: 'evt-1',
  logs: mockLogs,
  context: mockContext,
  analysis: mockAnalysis,
  raw_alert: {
    receiver: 'incidentgpt',
    status: 'firing',
    alerts: [
      {
        status: 'firing',
        labels: {
          alertname: 'KubePodCrashLooping',
          namespace: 'payments',
          pod: 'payment-svc-7f8b9c-x2k4j',
          severity: 'critical',
        },
        annotations: { summary: 'Pod is crash looping', description: 'Pod has restarted 3 times in last 10 minutes' },
        startsAt: '2026-02-19T14:22:00Z',
      },
    ],
  },
  enriched_data: {
    k8s: { pod_status: 'CrashLoopBackOff', restart_count: 3, last_termination_reason: 'OOMKilled' },
    prometheus: { memory_usage_bytes: 536870912, cpu_usage_percent: 92.3 },
    argocd: { app: 'payment-svc', revision: 'a3f8c2d', sync_status: 'Synced' },
  },
  metrics: mockMetrics,
  feedback: null,
}

export const mockDashboardStats: DashboardStats = {
  active_incidents: 3,
  active_critical: 1,
  mttr_reduction_pct: 47,
  noise_reduction_pct: 68,
  avg_analysis_time_sec: 192,
  alerts_today: 142,
  alerts_sparkline: [12, 8, 15, 22, 18, 34, 28, 19, 25, 31, 16, 42, 38, 27, 20, 14, 33, 29, 21, 17, 35, 26, 30, 23],
}

export const mockAlertVolume: AlertVolumeBucket[] = [
  { date: '2026-02-13', critical: 2, warning: 8, info: 15, unknown: 22 },
  { date: '2026-02-14', critical: 1, warning: 5, info: 12, unknown: 18 },
  { date: '2026-02-15', critical: 3, warning: 9, info: 18, unknown: 25 },
  { date: '2026-02-16', critical: 0, warning: 4, info: 10, unknown: 15 },
  { date: '2026-02-17', critical: 2, warning: 7, info: 14, unknown: 20 },
  { date: '2026-02-18', critical: 4, warning: 11, info: 22, unknown: 30 },
  { date: '2026-02-19', critical: 1, warning: 6, info: 13, unknown: 19 },
]

export const mockPredictions: Prediction[] = [
  {
    id: 'pred-001',
    resource: 'node-3 /data disk',
    resource_type: 'disk',
    current_value: 87,
    current_unit: '%',
    predicted_value: 100,
    time_to_critical: '5h',
    growth_rate: '+1.2 GB/hour',
    severity: 'warning',
    recommendation: 'Expand PV or clean /data/tmp (12GB of stale logs)',
    sparkline: [72, 74, 76, 78, 80, 82, 85, 87],
    node: 'node-3',
    namespace: 'storage',
  },
  {
    id: 'pred-002',
    resource: 'postgres-primary WAL',
    resource_type: 'disk',
    current_value: 78,
    current_unit: '%',
    predicted_value: 100,
    time_to_critical: '18h',
    growth_rate: '+0.8 GB/hour',
    severity: 'warning',
    recommendation: 'Run pg_archivecleanup or increase WAL retention volume',
    sparkline: [60, 62, 65, 68, 70, 73, 75, 78],
    node: 'node-1',
    namespace: 'databases',
  },
]

export const mockPredictionSummary: PredictionSummary = {
  healthy_count: 42,
  warning_count: 2,
  critical_count: 0,
  avg_cpu: 34,
  avg_memory: 52,
  avg_disk: 61,
  network_status: 'stable',
}

export const mockLicense: LicenseInfo = {
  plan: 'professional',
  key: 'CCII-PRO-2026-3A4F',
  expires_at: '2026-12-31',
  status: 'active',
  usage: { alerts_used: 4832, alerts_limit: 10000, clusters_used: 2, clusters_limit: 3, servers_used: 45, servers_limit: 100 },
  features: { enrichment: true, correlation: true, logs: true, predictions: false, postmortem: false },
  support_level: '8x5_chat',
}

export const mockAnalyticsSummary: AnalyticsSummary = {
  mttr_before_min: 12,
  mttr_after_min: 5,
  mttr_reduction_pct: 58,
  mtta_before_min: 8,
  mtta_after_min: 2,
  mtta_reduction_pct: 75,
  noise_before: 342,
  noise_after: 98,
  noise_reduction_pct: 71,
  time_saved_hours: 47,
}

export const mockMTTRTrend: { data: MTTRDataPoint[]; enabled_at: string } = {
  data: Array.from({ length: 30 }, (_, i) => {
    const date = new Date(2026, 0, 20 + i)
    const isBefore = i < 12
    return {
      date: date.toISOString().split('T')[0],
      mttr_min: isBefore ? 20 + rand(-5, 5) : 8 + rand(-3, 3),
      mtta_min: isBefore ? 10 + rand(-3, 3) : 3 + rand(-1, 2),
    }
  }),
  enabled_at: '2026-02-01',
}

export const mockAnalyticsQuality: AnalyticsQuality = {
  total_analyses: 142,
  helpful_pct: 82,
  not_helpful_pct: 11,
  edited_pct: 7,
  avg_confidence: 89,
  avg_response_time_sec: 8.2,
}

export const mockKnowledgeResults: KnowledgeResult[] = [
  {
    incident_id: 'INC-2026-0098',
    title: 'payment-svc OOMKilled after v2.2.0 deploy',
    severity: 'critical',
    date: '2026-01-15',
    root_cause: 'Buffer pool leak in processPayment() goroutine',
    resolution: 'Rolled back to v2.1.9, fixed buffer pool in v2.2.1',
    namespace: 'payments',
    service: 'payment-svc',
    similarity: 92,
    has_postmortem: true,
    resolved_in_min: 45,
  },
  {
    incident_id: 'INC-2025-0412',
    title: 'checkout-svc memory leak in gRPC handler',
    severity: 'critical',
    date: '2025-11-20',
    root_cause: 'Unclosed gRPC streams leaking memory per connection',
    resolution: 'Fixed connection pool leak, added memory limit alerts',
    namespace: 'checkout',
    service: 'checkout-svc',
    similarity: 78,
    has_postmortem: true,
    resolved_in_min: 120,
  },
  {
    incident_id: 'INC-2025-0389',
    title: 'api-gateway OOM under load test',
    severity: 'warning',
    date: '2025-10-05',
    root_cause: 'Request body not released in middleware chain',
    resolution: 'Added defer body.Close() in middleware, increased limits',
    namespace: 'default',
    service: 'api-gateway',
    similarity: 65,
    has_postmortem: false,
    resolved_in_min: 30,
  },
]

export const mockServiceNodes: ServiceNode[] = [
  { id: 'ingress', name: 'ingress-nginx', type: 'gateway', namespace: 'ingress', status: 'healthy' },
  { id: 'api-gw', name: 'api-gateway', type: 'service', namespace: 'default', status: 'healthy' },
  { id: 'payment', name: 'payment-svc', type: 'service', namespace: 'payments', status: 'incident' },
  { id: 'checkout', name: 'checkout-svc', type: 'service', namespace: 'checkout', status: 'healthy' },
  { id: 'notification', name: 'notification-svc', type: 'service', namespace: 'messaging', status: 'healthy' },
  { id: 'postgres', name: 'postgres-primary', type: 'database', namespace: 'databases', status: 'healthy' },
  { id: 'redis', name: 'redis-cluster', type: 'cache', namespace: 'cache', status: 'warning' },
  { id: 'kafka', name: 'kafka-broker', type: 'queue', namespace: 'messaging', status: 'healthy' },
]

export const mockServiceEdges: ServiceEdge[] = [
  { source: 'ingress', target: 'api-gw', protocol: 'HTTP' },
  { source: 'api-gw', target: 'payment', protocol: 'gRPC' },
  { source: 'api-gw', target: 'checkout', protocol: 'gRPC' },
  { source: 'payment', target: 'postgres', protocol: 'TCP' },
  { source: 'payment', target: 'redis', protocol: 'TCP' },
  { source: 'checkout', target: 'postgres', protocol: 'TCP' },
  { source: 'checkout', target: 'redis', protocol: 'TCP' },
  { source: 'checkout', target: 'kafka', protocol: 'TCP' },
  { source: 'notification', target: 'kafka', protocol: 'TCP' },
]

export const mockServiceDetail: ServiceDetail = {
  id: 'payment',
  name: 'payment-svc',
  namespace: 'payments',
  status: 'incident',
  metrics: { cpu_pct: 78, memory_pct: 92, replicas: '2/3' },
  active_incidents: 1,
  last_deploy: { version: 'v2.4.0', at: '2026-02-19T14:10:00Z', source: 'argocd' },
  inbound: ['api-gw', 'checkout'],
  outbound: ['postgres', 'redis'],
}

export const mockIntegrations: Integration[] = [
  { type: 'prometheus', name: 'Prometheus', category: 'data_source', status: 'connected', config: { url: 'http://prometheus:9090' }, last_heartbeat: '2026-02-19T14:30:00Z', enabled: true },
  { type: 'zabbix', name: 'Zabbix', category: 'data_source', status: 'connected', config: { url: 'http://zabbix:8080' }, last_heartbeat: '2026-02-19T14:28:00Z', enabled: true },
  { type: 'loki', name: 'Loki', category: 'data_source', status: 'connected', config: { url: 'http://loki:3100' }, last_heartbeat: '2026-02-19T14:29:00Z', enabled: true },
  { type: 'kubernetes', name: 'Kubernetes', category: 'data_source', status: 'connected', config: { cluster: 'prod-01' }, last_heartbeat: '2026-02-19T14:30:00Z', enabled: true },
  { type: 'logs', name: 'Логи', category: 'data_source', status: 'not_configured', config: {}, last_heartbeat: null, enabled: false },
  { type: 'telegram', name: 'Telegram', category: 'output_channel', status: 'connected', config: { chat_id: '-100123456' }, last_heartbeat: null, enabled: true },
  { type: 'slack', name: 'Slack', category: 'output_channel', status: 'connected', config: { channel: '#incidents' }, last_heartbeat: null, enabled: true },
  { type: 'jira', name: 'Jira', category: 'output_channel', status: 'not_configured', config: {}, last_heartbeat: null, enabled: false },
]

export const mockLLMConfig: LLMConfig = {
  provider: 'ollama',
  base_url: 'http://ollama:11434',
  api_key: '',
  model: 'qwen2.5:7b',
  temperature: 0.3,
  max_tokens: 2048,
  system_prompt: 'You are an SRE assistant analyzing Kubernetes incidents...',
}

export const mockPostMortem: PostMortem = {
  markdown: `# Post Mortem: INC-2026-0139

## Incident Summary
| Field | Value |
|-------|-------|
| **ID** | INC-2026-0139 |
| **Severity** | P1 |
| **Duration** | 2h 15m |
| **Services** | api-gateway, payment-svc |
| **Date** | 2026-02-18 |

## Summary
API gateway experienced >5% 5xx error rate on the /v2/orders endpoint for 2 hours 15 minutes due to upstream payment-svc timeout cascading into 502 Bad Gateway responses.

## Timeline
| Time | Event | Source |
|------|-------|--------|
| 22:15 | payment-svc response time spike to 30s | Prometheus |
| 22:18 | api-gateway 502 rate exceeds 5% threshold | Prometheus |
| 22:20 | Alert fired: APIGateway5xxRate | Alertmanager |
| 22:25 | IncidentGPT analysis complete (94% confidence) | IncidentGPT |
| 22:30 | On-call acknowledged, investigating | Manual |
| 23:45 | payment-svc connection pool fix deployed | ArgoCD |
| 00:30 | Error rate back to <0.1%, incident resolved | Prometheus |

## Root Cause
The payment-svc v2.3.8 introduced a database connection leak that exhausted the PostgreSQL connection pool under sustained load. When the pool was exhausted, payment-svc requests started timing out (30s default), causing api-gateway to return 502 to clients.

## Impact
- ~2400 failed API calls on /v2/orders
- Estimated 180 affected customers
- Revenue impact: ~$12,000 in delayed transactions (all recovered)

## Action Items
- [x] Deploy connection pool fix (v2.3.9)
- [x] Add connection pool utilization alert at 80%
- [ ] Implement circuit breaker in api-gateway
- [ ] Add integration test for connection pool exhaustion
- [ ] Review timeout settings across all services

## Lessons Learned
1. Connection pool monitoring was missing — we only had query-level metrics
2. IncidentGPT correctly identified the root cause within 5 minutes, saving ~30 minutes of investigation
3. Need circuit breakers to prevent cascading failures
`,
  generated_at: '2026-02-19T01:00:00Z',
  status: 'ready',
  last_edited_by: 'sersert',
  last_edited_at: '2026-02-19T10:00:00Z',
}

export const mockPredictionTrend: PredictionTrend = {
  actual: Array.from({ length: 24 }, (_, i) => ({
    timestamp: new Date(Date.parse('2026-02-18T14:00:00Z') + i * 3600000).toISOString(),
    value: 60 + i * 1.2 + rand(-1, 1),
  })),
  predicted: Array.from({ length: 12 }, (_, i) => ({
    timestamp: new Date(Date.parse('2026-02-19T14:00:00Z') + i * 3600000).toISOString(),
    value: 87 + i * 1.2,
  })),
  threshold: 100,
  threshold_label: 'Disk full',
}
