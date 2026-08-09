import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/index.ts'

/** Сырой поток алертов. Отдельная сущность от инцидентов. */
export function useAlerts(params: string) {
  return useQuery({
    queryKey: ['alerts', params],
    queryFn: async () => {
      const r = await api.getAlerts(params)
      return { data: r?.data ?? [], total: r?.total ?? 0 }
    },
    refetchInterval: 30_000,
  })
}
