import { useEffect, useRef, type ReactNode } from 'react'
import { X } from 'lucide-react'

/**
 * Простое модальное окно на <dialog>: браузер сам даёт фокус-ловушку,
 * закрытие по Esc и подложку, поэтому руками это не воспроизводим.
 */
export function Modal({
  open,
  title,
  onClose,
  children,
  footer,
}: {
  open: boolean
  title: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
}) {
  const ref = useRef<HTMLDialogElement>(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (open && !el.open) el.showModal()
    if (!open && el.open) el.close()
  }, [open])

  return (
    <dialog
      ref={ref}
      onClose={onClose}
      onClick={(e) => {
        // Клик по самой подложке (не по содержимому) закрывает окно.
        if (e.target === ref.current) onClose()
      }}
      className="m-auto w-[min(480px,calc(100vw-32px))] rounded-[10px] border border-border-default bg-surface-secondary p-0 text-text-primary backdrop:bg-black/60"
    >
      <div className="flex items-center gap-3 border-b border-border-soft px-4 py-3">
        <h2 className="text-sm font-semibold tracking-[-0.01em]">{title}</h2>
        <button
          onClick={onClose}
          aria-label="Закрыть"
          className="ml-auto rounded-md p-1 text-text-muted transition-colors hover:bg-surface-tertiary hover:text-text-primary"
        >
          <X size={14} />
        </button>
      </div>
      <div className="px-4 py-4">{children}</div>
      {footer && (
        <div className="flex items-center gap-2 border-t border-border-soft px-4 py-3">{footer}</div>
      )}
    </dialog>
  )
}
