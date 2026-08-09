import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/index.ts'

export function useAnalyticsSummary(period: string) {
  return useQuery({
    queryKey: ['analytics', 'summary', period],
    queryFn: () => api.getAnalyticsSummary(period),
  })
}

export function useMTTRTrend(period: string) {
  return useQuery({
    queryKey: ['analytics', 'mttr', period],
    queryFn: () => api.getMTTRTrend(period),
  })
}

export function useAnalyticsQuality(period: string) {
  return useQuery({
    queryKey: ['analytics', 'quality', period],
    queryFn: () => api.getAnalyticsQuality(period),
  })
}

export function useTopSources(period: string) {
  return useQuery({
    queryKey: ['analytics', 'top-sources', period],
    queryFn: async () => (await api.getTopSources(period)) ?? [],
  })
}

export function useLicense() {
  return useQuery({ queryKey: ['license'], queryFn: () => api.getLicense() })
}

export function useIntegrations() {
  return useQuery({ queryKey: ['settings', 'integrations'], queryFn: () => api.getIntegrations() })
}
