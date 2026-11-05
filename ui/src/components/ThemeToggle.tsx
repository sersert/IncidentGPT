import { Moon, Sun } from 'lucide-react'
import { useTheme } from './ThemeProvider'

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()
  const dark = theme === 'dark'
  const label = dark ? 'Включить светлую тему' : 'Включить тёмную тему'
  const Icon = dark ? Moon : Sun
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-pressed={dark}
      onClick={() => setTheme(dark ? 'light' : 'dark')}
      className="flex min-h-10 shrink-0 items-center gap-2 rounded-md px-3 text-xs text-text-secondary hover:bg-surface-tertiary hover:text-text-primary"
    >
      <Icon size={16} strokeWidth={1.7} aria-hidden="true" />
      <span className="hidden md:inline">{dark ? 'Тёмная' : 'Светлая'}</span>
    </button>
  )
}
