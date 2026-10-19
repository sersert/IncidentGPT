import type { ReactNode } from 'react'

interface CardProps {
  title?: string
  action?: ReactNode
  children: ReactNode
  className?: string
  /** Убрать внутренние отступы — когда содержимое само управляет ими (списки, таблицы). */
  flush?: boolean
  plain?: boolean
}

export function Card({ title, action, children, className = '', flush = false, plain = false }: CardProps) {
  const hasHead = title || action
  return (
    <div
      className={`min-w-0 ${plain ? '' : 'rounded-lg border border-border-default bg-surface-secondary'} ${className}`}
    >
      {hasHead && (
        <div
          className={`flex flex-wrap items-center gap-3 ${plain ? 'pb-2' : 'border-b border-border-soft px-5 py-4'}`}
        >
          {title && (
            <h2
              className={`${plain ? 'text-lg' : 'text-sm'} font-semibold tracking-[-0.02em] text-text-primary`}
            >
              {title}
            </h2>
          )}
          {action && <div className="ml-auto min-w-0 max-w-full">{action}</div>}
        </div>
      )}
      <div className={plain ? 'pt-3' : flush ? '' : `px-5 pb-5 ${hasHead ? 'pt-4' : 'pt-5'}`}>{children}</div>
    </div>
  )
}
