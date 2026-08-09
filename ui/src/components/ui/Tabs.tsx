interface Tab {
  id: string
  label: string
  count?: number
}

interface TabsProps {
  tabs: Tab[]
  active: string
  onChange: (id: string) => void
  className?: string
  /** Подпись для скринридера — что именно переключают эти вкладки. */
  label?: string
}

export function Tabs({ tabs, active, onChange, className = '', label }: TabsProps) {
  const move = (dir: 1 | -1) => {
    const i = tabs.findIndex((t) => t.id === active)
    const next = tabs[(i + dir + tabs.length) % tabs.length]
    onChange(next.id)
  }

  return (
    <div role="tablist" aria-label={label} className={`flex gap-1 ${className}`}>
      {tabs.map((tab) => (
        <button
          key={tab.id}
          role="tab"
          aria-selected={active === tab.id}
          tabIndex={active === tab.id ? 0 : -1}
          onClick={() => onChange(tab.id)}
          onKeyDown={(e) => {
            if (e.key === 'ArrowRight') { e.preventDefault(); move(1) }
            if (e.key === 'ArrowLeft') { e.preventDefault(); move(-1) }
          }}
          className={`rounded-[7px] px-[11px] py-[5px] text-xs font-medium transition-colors ${
            active === tab.id
              ? 'bg-surface-tertiary text-text-primary'
              : 'text-text-muted hover:text-text-secondary'
          }`}
        >
          {tab.label}
          {tab.count !== undefined && <span className="tnum ml-1.5 opacity-70">{tab.count}</span>}
        </button>
      ))}
    </div>
  )
}
