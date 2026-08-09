import { NavLink } from 'react-router-dom'
import {
  AlertTriangle,
  Bell,
  LayoutDashboard,
  BarChart3,
  TrendingUp,
  Plug,
  Settings as SettingsIcon,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

interface Item {
  to: string
  label: string
  icon: LucideIcon
  end?: boolean
  /** Счётчик справа — сколько объектов требует внимания. */
  count?: number
}

interface Group {
  title: string
  items: Item[]
}

export function Sidebar({ activeIncidents, warnings }: { activeIncidents?: number; warnings?: number }) {
  const groups: Group[] = [
    {
      title: 'Инциденты',
      items: [
        { to: '/incidents', label: 'Инциденты', icon: AlertTriangle, count: activeIncidents },
        { to: '/alerts', label: 'Алерты', icon: Bell },
        { to: '/', label: 'Дашборд', icon: LayoutDashboard, end: true },
      ],
    },
    {
      title: 'Аналитика',
      items: [
        { to: '/analytics', label: 'Аналитика', icon: BarChart3 },
        { to: '/predictions', label: 'Прогнозы', icon: TrendingUp, count: warnings },
      ],
    },
    {
      title: 'Система',
      items: [
        { to: '/settings/integrations', label: 'Интеграции', icon: Plug },
        { to: '/settings/llm', label: 'Модель', icon: SettingsIcon },
      ],
    },
  ]

  return (
    <aside className="sticky top-0 flex h-screen w-[216px] shrink-0 flex-col border-r border-border-default bg-surface-secondary px-2.5 py-3.5">
      <NavLink to="/" className="flex items-center gap-2.5 px-2 pb-4 text-[15px] font-semibold tracking-[-0.015em]">
        {/* Восьмигранник — план дозорной башни сверху, внутри пульс: смотрим и живы. */}
        <svg width="19" height="19" viewBox="0 0 32 32" fill="none" aria-hidden="true">
          <path
            d="M12 5h8l7 7v8l-7 7h-8l-7-7v-8l7-7z"
            fill="var(--color-accent)"
            fillOpacity="0.12"
            stroke="var(--color-accent)"
            strokeWidth="2"
            strokeLinejoin="round"
          />
          <path
            d="M9.5 16.6h2.4l1.7-4.4 2.7 8.6 1.9-5.2 1.1 1h3.2"
            stroke="var(--color-accent)"
            strokeWidth="2.2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        IncidentGPT
      </NavLink>

      <nav className="flex flex-col gap-0.5 overflow-y-auto">
        {groups.map((group) => (
          <div key={group.title}>
            <p className="px-2 pt-3.5 pb-1.5 text-[10px] font-medium uppercase tracking-[0.1em] text-text-muted">
              {group.title}
            </p>
            {group.items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `flex items-center gap-2.5 rounded-[7px] px-2 py-[7px] text-[13px] transition-colors ${
                    isActive
                      ? 'bg-accent/12 text-[#B9B9FF]'
                      : 'text-text-secondary hover:bg-surface-tertiary hover:text-text-primary'
                  }`
                }
              >
                <item.icon size={15} strokeWidth={1.9} />
                {item.label}
                {item.count ? (
                  <span className="tnum ml-auto font-mono text-[11px] text-text-muted">{item.count}</span>
                ) : null}
              </NavLink>
            ))}
          </div>
        ))}
      </nav>

      <div className="mt-auto flex items-center gap-2.5 border-t border-border-soft px-2 pt-3 text-[13px] text-text-secondary">
        <span className="grid h-6 w-6 place-items-center rounded-full bg-accent text-[11px] font-semibold text-surface-primary">
          S
        </span>
        sersert
      </div>
    </aside>
  )
}
