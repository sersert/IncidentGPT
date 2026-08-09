import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/index.ts'

/** Список инцидентов. Параметры — строка query, она же ключ кэша. */
export function useIncidents(params: string) {
  return useQuery({
    queryKey: ['incidents', params],
    queryFn: async () => {
      const r = await api.getIncidents(params)
      return { data: r?.data ?? [], total: r?.total ?? 0 }
    },
    // Список живой: пока вкладка открыта, подтягиваем новые инциденты.
    refetchInterval: 30_000,
  })
}

/**
 * Один инцидент. Пока AI не прислал анализ, опрашиваем часто — иначе панель
 * «Анализируем…» висит вечно. Как только анализ пришёл, опрос прекращается.
 */
export function useIncident(id: string | undefined) {
  return useQuery({
    queryKey: ['incident', id],
    queryFn: () => api.getIncident(id!),
    enabled: Boolean(id),
    refetchInterval: (query) => (query.state.data?.analysis ? false : 3_000),
  })
}

/**
 * Логи инцидента. Запрашиваем только когда открыта вкладка «Логи»: поход
 * в хранилище логов дороже, чем чтение самого инцидента.
 */
export function useIncidentLogs(incidentId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: ['incident', incidentId, 'logs'],
    queryFn: () => api.getIncidentLogs(incidentId!),
    enabled: Boolean(incidentId) && enabled,
    staleTime: 60_000,
  })
}

/** Действия дежурного над инцидентом: статус и назначение. */
export function useUpdateIncident(incidentId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (patch: { status?: string; assignee?: string | null }) =>
      api.updateIncident(incidentId, patch),
    onSuccess: (updated) => {
      // Кладём ответ прямо в кэш: список статусов обновится без лишнего запроса.
      qc.setQueryData(['incident', incidentId], updated)
      qc.invalidateQueries({ queryKey: ['incidents'] })
      qc.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })
}

export function useSubmitFeedback(incidentId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ helpful, comment }: { helpful: boolean; comment: string | null }) =>
      api.submitFeedback(incidentId, helpful, comment),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['incident', incidentId] }),
  })
}
