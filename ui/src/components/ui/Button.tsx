import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md' | 'lg'

const variants: Record<Variant, string> = {
  primary: 'bg-accent hover:bg-accent-hover text-surface-primary',
  secondary: 'bg-surface-tertiary hover:border-[#3A4159] text-text-primary border border-border-default',
  ghost: 'hover:bg-surface-tertiary text-text-secondary hover:text-text-primary',
  danger: 'bg-severity-critical/15 hover:bg-severity-critical/25 text-severity-critical border border-severity-critical/40',
}

const sizes: Record<Size, string> = {
  sm: 'px-2.5 py-1 text-xs rounded-md',
  md: 'px-3.5 py-1.5 text-[13px] rounded-lg',
  lg: 'px-4 py-2 text-sm rounded-lg',
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
}

export function Button({ variant = 'primary', size = 'md', className = '', children, ...props }: ButtonProps) {
  return (
    <button
      className={`inline-flex items-center justify-center gap-1.5 font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${variants[variant]} ${sizes[size]} ${className}`}
      {...props}
    >
      {children}
    </button>
  )
}
