import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar.tsx'
import { useDashboardStats } from '@/hooks/useDashboard.ts'
import { usePredictions } from '@/hooks/usePredictions.ts'

export function Layout() {
  // Счётчики в сайдбаре берутся из тех же запросов, что и экраны, — TanStack Query
  // отдаёт их из кэша, лишнего похода в сеть нет.
  const { data: stats } = useDashboardStats()
  const { data: predictions } = usePredictions()

  return (
    <div className="flex min-h-screen bg-surface-primary">
      <Sidebar activeIncidents={stats?.active_incidents} warnings={predictions?.length} />
      <main className="min-w-0 flex-1 px-7 pt-6 pb-16">
        <div className="mx-auto max-w-[1320px]">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
