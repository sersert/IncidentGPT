import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/index.ts'

export function usePredictions() {
  return useQuery({
    queryKey: ['predictions'],
    queryFn: async () => (await api.getPredictions()) ?? [],
    refetchInterval: 5 * 60_000,
  })
}

export function usePredictionSummary() {
  return useQuery({
    queryKey: ['predictions', 'summary'],
    queryFn: () => api.getPredictionSummary(),
    refetchInterval: 5 * 60_000,
  })
}
