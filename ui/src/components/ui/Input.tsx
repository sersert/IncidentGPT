import { useId } from 'react'
import type { InputHTMLAttributes } from 'react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

export function Input({ label, error, className = '', id, ...props }: InputProps) {
  const autoId = useId()
  const inputId = id ?? autoId
  const errorId = `${inputId}-error`

  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label htmlFor={inputId} className="text-xs font-medium text-text-secondary">
          {label}
        </label>
      )}
      <input
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        className={`rounded-lg border bg-surface-primary px-3 py-1.5 text-[13px] text-text-primary outline-none transition-colors placeholder:text-text-muted focus:border-accent/60 ${
          error ? 'border-severity-critical' : 'border-border-default'
        } ${className}`}
        {...props}
      />
      {error && (
        <span id={errorId} className="text-xs text-severity-critical">
          {error}
        </span>
      )}
    </div>
  )
}
