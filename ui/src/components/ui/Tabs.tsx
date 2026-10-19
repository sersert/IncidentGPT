import { useId, useRef } from 'react'

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
  label?: string
  idPrefix?: string
  panelIdPrefix?: string
}

export function Tabs({ tabs, active, onChange, className = '', label, idPrefix, panelIdPrefix }: TabsProps) {
  const autoId = useId()
  const list = useRef<HTMLDivElement>(null)
  const prefix = idPrefix ?? autoId
  const activate = (index: number) => {
    const next = (index + tabs.length) % tabs.length
    onChange(tabs[next].id)
    list.current?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus()
  }
  return (
    <div
      ref={list}
      role="tablist"
      aria-label={label}
      className={`flex max-w-full gap-5 overflow-x-auto border-b border-border-soft ${className}`}
    >
      {tabs.map((tab, index) => (
        <button
          key={tab.id}
          id={`${prefix}-${tab.id}`}
          type="button"
          role="tab"
          aria-selected={active === tab.id}
          aria-controls={panelIdPrefix ? `${panelIdPrefix}-${tab.id}` : undefined}
          tabIndex={active === tab.id ? 0 : -1}
          onClick={() => onChange(tab.id)}
          onKeyDown={(event) => {
            const destination =
              event.key === 'ArrowRight'
                ? index + 1
                : event.key === 'ArrowLeft'
                  ? index - 1
                  : event.key === 'Home'
                    ? 0
                    : event.key === 'End'
                      ? tabs.length - 1
                      : null
            if (destination !== null) {
              event.preventDefault()
              activate(destination)
            }
          }}
          className={`min-h-11 shrink-0 border-b-2 px-1 text-[13px] font-medium transition-colors ${active === tab.id ? 'border-text-primary text-text-primary' : 'border-transparent text-text-muted hover:text-text-primary'}`}
        >
          {tab.label}
          {tab.count !== undefined && <span className="tnum ml-2 opacity-70">{tab.count}</span>}
        </button>
      ))}
    </div>
  )
}
