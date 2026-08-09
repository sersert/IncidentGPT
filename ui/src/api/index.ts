import { get, post, put, patch } from './client.ts'
import {
  mock,
  mockIncidents,
  mockAlerts,
  mockIncidentDetail,
  mockDashboardStats,
  mockAlertVolume,
  mockPredictions,
  mockPredictionSummary,
  mockPredictionTrend,
  mockLicense,
  mockAnalyticsSummary,
  mockMTTRTrend,
  mockAnalyticsQuality,
  mockKnowledgeResults,
  mockServiceNodes,
  mockServiceEdges,
  mockServiceDetail,
  mockIntegrations,
  mockPostMortem,
} from './mocks.ts'
import type {
  Incident,
  Alert,
  IncidentDetail,
  LogEntry,
  DashboardStats,
  AlertVolumeBucket,
  Prediction,
  PredictionSummary,
  PredictionTrend,
  LicenseInfo,
  AnalyticsSummary,
  MTTRDataPoint,
  AnalyticsQuality,
  KnowledgeResult,
  ServiceNode,
  ServiceEdge,
  ServiceDetail,
  Integration,
  LLMStatus,
  IntegrationTestResult,
  PostMortem,
  PaginatedResponse,
} from './types.ts'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'

export const api = {
  // Incidents
  getIncidents(params?: string): Promise<PaginatedResponse<Incident>> {
    if (USE_MOCKS) return mock({ data: mockIncidents, total: mockIncidents.length })
    return get(`/incidents${params ? `?${params}` : ''}`)
  },

  getIncident(id: string): Promise<IncidentDetail> {
    if (USE_MOCKS) return mock(mockIncidentDetail)
    return get(`/incidents/${id}`)
  },

  // Alerts — сырой поток, отдельно от инцидентов
  getAlerts(params?: string): Promise<PaginatedResponse<Alert>> {
    if (USE_MOCKS) return mock({ data: mockAlerts, total: mockAlerts.length })
    return get(`/alerts${params ? `?${params}` : ''}`)
  },

  /** Логи инцидента из хранилища логов — отдельным запросом, они тяжелее самого инцидента. */
  getIncidentLogs(id: string, level?: string): Promise<{ data: LogEntry[]; reason?: string }> {
    if (USE_MOCKS) return mock({ data: mockIncidentDetail.logs })
    return get(`/incidents/${id}/logs${level ? `?level=${level}` : ''}`)
  },

  /** Действия дежурного: принять в работу, заглушить, назначить, закрыть. */
  updateIncident(id: string, patchBody: { status?: string; assignee?: string | null }): Promise<IncidentDetail> {
    if (USE_MOCKS) return mock(mockIncidentDetail)
    return patch(`/incidents/${id}`, patchBody)
  },

  submitFeedback(id: string, helpful: boolean, comment: string | null): Promise<void> {
    if (USE_MOCKS) return mock(undefined as never)
    return post(`/incidents/${id}/feedback`, { helpful, comment })
  },

  // Dashboard
  getDashboardStats(): Promise<DashboardStats> {
    if (USE_MOCKS) return mock(mockDashboardStats)
    return get('/dashboard/stats')
  },

  getActiveIncidents(limit = 10): Promise<Incident[]> {
    if (USE_MOCKS) return mock(mockIncidents.filter((i) => i.status === 'active').slice(0, limit))
    return get(`/dashboard/active?limit=${limit}`)
  },

  getAlertVolume(period = '7d'): Promise<AlertVolumeBucket[]> {
    if (USE_MOCKS) return mock(mockAlertVolume)
    return get(`/analytics/volume?period=${period}&group_by=day`)
  },

  // Predictions
  getPredictions(): Promise<Prediction[]> {
    if (USE_MOCKS) return mock(mockPredictions)
    return get('/predictions?severity=warning,critical')
  },

  getPredictionSummary(): Promise<PredictionSummary> {
    if (USE_MOCKS) return mock(mockPredictionSummary)
    return get('/predictions/summary')
  },

  getPredictionTrend(id: string): Promise<PredictionTrend> {
    if (USE_MOCKS) return mock(mockPredictionTrend)
    return get(`/predictions/${id}/trend`)
  },

  // License
  getLicense(): Promise<LicenseInfo> {
    if (USE_MOCKS) return mock(mockLicense)
    return get('/license')
  },

  // Analytics
  getAnalyticsSummary(period = '30d'): Promise<AnalyticsSummary> {
    if (USE_MOCKS) return mock(mockAnalyticsSummary)
    return get(`/analytics/summary?period=${period}`)
  },

  getMTTRTrend(period = '30d'): Promise<{ data: MTTRDataPoint[]; enabled_at: string }> {
    if (USE_MOCKS) return mock(mockMTTRTrend)
    return get(`/analytics/mttr?period=${period}&group_by=day`)
  },

  getAnalyticsQuality(period = '30d'): Promise<AnalyticsQuality> {
    if (USE_MOCKS) return mock(mockAnalyticsQuality)
    return get(`/analytics/quality?period=${period}`)
  },

  getTopSources(period = '30d'): Promise<{ source: string; count: number; severity: string }[]> {
    if (USE_MOCKS)
      return mock([
        { source: 'payment-svc', count: 42, severity: 'P1' },
        { source: 'api-gateway', count: 35, severity: 'P2' },
        { source: 'checkout-svc', count: 28, severity: 'P2' },
        { source: 'redis-cluster', count: 22, severity: 'P3' },
        { source: 'postgres-primary', count: 18, severity: 'P2' },
        { source: 'kafka-broker', count: 15, severity: 'P3' },
        { source: 'ingress-nginx', count: 12, severity: 'P3' },
        { source: 'cert-manager', count: 8, severity: 'P4' },
        { source: 'loki', count: 5, severity: 'P4' },
        { source: 'argocd', count: 3, severity: 'P4' },
      ])
    return get(`/analytics/top-sources?period=${period}&limit=10`)
  },

  // Knowledge Base
  searchKnowledge(q: string, params?: string): Promise<{ results: KnowledgeResult[]; total: number }> {
    if (USE_MOCKS) return mock({ results: mockKnowledgeResults, total: mockKnowledgeResults.length })
    return get(`/knowledge/search?q=${encodeURIComponent(q)}${params ? `&${params}` : ''}`)
  },

  getKnowledgePatterns(): Promise<{ pattern: string; count: number }[]> {
    if (USE_MOCKS)
      return mock([
        { pattern: 'OOMKilled', count: 12 },
        { pattern: 'Disk full', count: 8 },
        { pattern: 'Connection timeout', count: 6 },
        { pattern: 'CrashLoopBackOff', count: 5 },
        { pattern: '5xx spike', count: 4 },
      ])
    return get('/knowledge/patterns?limit=5')
  },

  // Service Map
  getServiceGraph(): Promise<{ nodes: ServiceNode[]; edges: ServiceEdge[] }> {
    if (USE_MOCKS) return mock({ nodes: mockServiceNodes, edges: mockServiceEdges })
    return get('/services/graph')
  },

  getServiceDetail(id: string): Promise<ServiceDetail> {
    if (USE_MOCKS) return mock(mockServiceDetail)
    return get(`/services/${id}`)
  },

  // Settings
  getIntegrations(): Promise<Integration[]> {
    if (USE_MOCKS) return mock(mockIntegrations)
    return get('/settings/integrations')
  },

  getLLMStatus(): Promise<LLMStatus> {
    if (USE_MOCKS)
      return mock({
        editable: false as const, managed_by: 'ai-worker',
        note: 'значения берутся из окружения ai-worker',
        reachable: true, model: 'google/gemini-2.5-flash', max_tokens: 2000,
        timeout: '1m0s', base_url_configured: true,
        telegram_channel_configured: true, telegram_parse_mode: 'Markdown',
        active_llm_slots: 0,
      })
    return get('/settings/llm')
  },

  testLLMConnection(): Promise<{ success: boolean; model?: string; latency_ms: number; error?: string }> {
    if (USE_MOCKS) return mock({ success: true, model: 'google/gemini-2.5-flash', latency_ms: 42 })
    return post('/settings/llm/test')
  },

  saveIntegration(type: string, config: Record<string, string>, enabled?: boolean): Promise<Integration> {
    if (USE_MOCKS) return mock(mockIntegrations[0])
    return put(`/settings/integrations/${type}`, { config, enabled })
  },

  testIntegration(type: string): Promise<IntegrationTestResult> {
    if (USE_MOCKS) return mock({ success: true, message: 'ответ получен за 24 мс', latency_ms: 24 })
    return post(`/settings/integrations/${type}/test`)
  },

  // Post Mortem
  getPostMortem(incidentId: string): Promise<PostMortem> {
    if (USE_MOCKS) return mock(mockPostMortem)
    return get(`/incidents/${incidentId}/postmortem`)
  },

  generatePostMortem(incidentId: string): Promise<{ status: string }> {
    if (USE_MOCKS) return mock({ status: 'generating' })
    return post(`/incidents/${incidentId}/postmortem`)
  },

  savePostMortem(incidentId: string, markdown: string): Promise<void> {
    if (USE_MOCKS) return mock(undefined as never)
    return put(`/incidents/${incidentId}/postmortem`, { markdown })
  },
}

export type { Incident, IncidentDetail, DashboardStats, Severity, IncidentStatus } from './types.ts'
