import type { ReactNode } from 'react'

interface CardProps {
  title?: string
  action?: ReactNode
  children: ReactNode
  className?: string
  /** Убрать внутренние отступы — когда содержимое само управляет ими (списки, таблицы). */
  flush?: boolean
}

export function Card({ title, action, children, className = '', flush = false }: CardProps) {
  const hasHead = title || action
  return (
    <div className={`rounded-[10px] border border-border-default bg-surface-secondary ${className}`}>
      {hasHead && (
        <div className="flex items-center gap-3 px-4 pt-3.5 pb-1">
          {title && <h3 className="text-sm font-semibold tracking-[-0.01em] text-text-primary">{title}</h3>}
          {action && <div className="ml-auto">{action}</div>}
        </div>
      )}
      <div className={flush ? '' : `px-4 pb-4 ${hasHead ? 'pt-2.5' : 'pt-4'}`}>{children}</div>
    </div>
  )
}
