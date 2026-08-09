import { useState, type ReactNode } from 'react'

export function Tooltip({ content, children }: { content: string; children: ReactNode }) {
  const [show, setShow] = useState(false)

  return (
    <span className="relative inline-flex" onMouseEnter={() => setShow(true)} onMouseLeave={() => setShow(false)}>
      {children}
      {show && (
        <span className="absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 whitespace-nowrap rounded bg-surface-tertiary px-2 py-1 text-xs text-text-primary shadow-lg border border-border-default z-50">
          {content}
        </span>
      )}
    </span>
  )
}
