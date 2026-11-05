import { Link } from 'react-router-dom'
import { useOperations } from '@/hooks/useOperations'

export function OperationsBar() {
  const query = useOperations()
  if (!query.data && query.isError) return <Link to="/operations" className="mb-4 block text-xs text-text-muted">Состояние системы →</Link>
  if (!query.data) return null
  const unknown = query.isError || !query.data.control_available
  const paused = !unknown && query.data.control?.enabled === false
  return <Link to="/operations" className="mb-4 flex flex-wrap items-center gap-2 text-xs text-text-secondary">
    <span aria-hidden="true" className={`h-2 w-2 rounded-full ${unknown ? 'bg-text-muted' : paused ? 'bg-severity-warning' : 'bg-status-resolved'}`} />
    {unknown ? 'Состояние AI неизвестно' : paused ? 'Запрошена пауза AI-анализа' : 'AI-анализ разрешён'} · Состояние системы →
  </Link>
}
