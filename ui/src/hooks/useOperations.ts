import { useQuery } from '@tanstack/react-query'
import { ApiError, get } from '@/api/client'

export type AIControl = { enabled: boolean; revision: number; updated_at: string; updated_by: string }
export type ComponentStatus = { id: string; name: string; status: string; detail: string; checked_at: string; latency_ms: number }
export type OperationsStatus = {
  checked_at: string
  components: ComponentStatus[]
  control: AIControl | null
  control_available: boolean
  worker?: { managed?: boolean; control_known?: boolean; control?: { enabled?: boolean; revision?: number }; active_provider_requests?: number }
}
export function useOperations() {
  return useQuery({ queryKey: ['operations'], queryFn: () => get<OperationsStatus>('/operations/status'), retry: false, refetchInterval: query => {
    const error = query.state.error
    if (error instanceof ApiError && [401, 403, 404].includes(error.status)) return false
    return error ? 60_000 : 10_000
  }, staleTime: 5_000 })
}
