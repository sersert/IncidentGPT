import { NavLink } from 'react-router-dom'
import {
  AlertTriangle,
  Bell,
  LayoutDashboard,
  BarChart3,
  TrendingUp,
  Plug,
  Settings,
  BookOpen,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

interface Item {
  to: string
  label: string
  icon: LucideIcon
  end?: boolean
  count?: number
}
interface SidebarProps {
  activeIncidents?: number
  warnings?: number
  mobile?: boolean
  onNavigate?: () => void
}

export function Sidebar({ activeIncidents, warnings, mobile = false, onNavigate }: SidebarProps) {
  const groups: { title: string; items: Item[] }[] = [
    {
      title: 'Мониторинг',
      items: [
        { to: '/', label: 'Обзор', icon: LayoutDashboard, end: true },
        { to: '/incidents', label: 'Инциденты', icon: AlertTriangle, count: activeIncidents },
        { to: '/alerts', label: 'Алерты', icon: Bell },
        { to: '/analytics', label: 'Аналитика', icon: BarChart3 },
        { to: '/predictions', label: 'Прогнозы', icon: TrendingUp, count: warnings },
      ],
    },
    {
      title: 'Настройки',
      items: [
        { to: '/settings/integrations', label: 'Интеграции', icon: Plug },
        { to: '/settings/llm', label: 'Модель', icon: Settings },
      ],
    },
  ]
  return (
    <aside
      className={
        mobile
          ? 'flex min-h-full flex-col bg-surface-secondary p-5'
          : 'sticky top-0 hidden h-dvh w-[220px] shrink-0 flex-col border-r border-border-default bg-surface-secondary px-4 py-5 lg:flex'
      }
    >
      <NavLink
        to="/"
        onClick={onNavigate}
        className="mb-10 flex items-center gap-2.5 px-2 text-[19px] font-semibold tracking-[-.04em]"
      >
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M6 4h12v16H6z" stroke="currentColor" strokeWidth="1.6" />
          <path d="M10 8h4m-4 4h4m-4 4h4" stroke="currentColor" strokeWidth="1.6" />
        </svg>
        IncidentGPT
      </NavLink>
      <nav aria-label="Основная навигация" className="min-h-0 space-y-8 overflow-y-auto">
        {groups.map((group) => (
          <div key={group.title}>
            <p className="mb-2 px-3 text-[11px] font-medium text-text-muted">{group.title}</p>
            <div className="space-y-1">
              {group.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  onClick={onNavigate}
                  className={({ isActive }) =>
                    `flex min-h-10 items-center gap-3 rounded-md px-3 text-[13px] transition-colors ${isActive ? 'bg-surface-tertiary font-semibold text-text-primary' : 'text-text-secondary hover:bg-surface-primary hover:text-text-primary'}`
                  }
                >
                  <item.icon size={16} strokeWidth={1.6} aria-hidden="true" />
                  {item.label}
                  {item.count != null && item.count > 0 && (
                    <span className="tnum ml-auto text-[11px] font-medium text-text-muted">{item.count}</span>
                  )}
                </NavLink>
              ))}
            </div>
          </div>
        ))}
      </nav>
      <NavLink
        to="/help"
        onClick={onNavigate}
        className={({ isActive }) =>
          `mt-auto flex min-h-11 items-center gap-3 rounded-md px-3 text-[13px] ${isActive ? 'bg-surface-tertiary font-medium text-text-primary' : 'text-text-secondary hover:bg-surface-primary'}`
        }
      >
        <BookOpen size={16} strokeWidth={1.6} />
        Руководство
      </NavLink>
    </aside>
  )
}
