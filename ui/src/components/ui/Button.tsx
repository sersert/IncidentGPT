import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md' | 'lg'

const variants: Record<Variant, string> = {
  primary: 'bg-accent hover:bg-accent-hover text-text-on-accent',
  secondary: 'bg-surface-secondary hover:bg-surface-tertiary/50 hover:border-text-muted text-text-primary border border-border-default',
  ghost: 'hover:bg-surface-tertiary text-text-secondary hover:text-text-primary',
  danger: 'bg-severity-critical/15 hover:bg-severity-critical/25 text-severity-critical border border-severity-critical/40',
}

const sizes: Record<Size, string> = {
  sm: 'min-h-9 px-3 py-2 text-xs rounded-md',
  md: 'min-h-10 px-4 py-2 text-[13px] rounded-md',
  lg: 'min-h-11 px-5 py-2.5 text-sm rounded-md',
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
}

export function Button({ variant = 'primary', size = 'md', className = '', children, type = 'button', ...props }: ButtonProps) {
  return (
    <button
      type={type}
      className={`inline-flex items-center justify-center gap-1.5 font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${variants[variant]} ${sizes[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  )
}
