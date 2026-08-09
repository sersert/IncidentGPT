import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/index.ts'

export function useDashboardStats() {
  return useQuery({
    queryKey: ['dashboard', 'stats'],
    queryFn: () => api.getDashboardStats(),
    refetchInterval: 60_000,
  })
}

export function useActiveIncidents(limit = 10) {
  return useQuery({
    queryKey: ['dashboard', 'active', limit],
    // Пустой список бэкенд может отдать как null; дефолт `= []` в деструктуризации
    // на null не срабатывает, поэтому нормализуем здесь, а не на каждом экране.
    queryFn: async () => (await api.getActiveIncidents(limit)) ?? [],
    refetchInterval: 30_000,
  })
}

export function useAlertVolume(period = '7d') {
  return useQuery({
    queryKey: ['analytics', 'volume', period],
    queryFn: async () => (await api.getAlertVolume(period)) ?? [],
  })
}
