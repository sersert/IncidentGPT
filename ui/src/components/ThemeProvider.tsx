import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'

type Theme = 'light' | 'dark'
const STORAGE_KEY = 'incidentgpt-theme'
const SYSTEM_QUERY = '(prefers-color-scheme: dark)'
const ThemeContext = createContext<{ theme: Theme; setTheme: (theme: Theme) => void } | null>(null)

function storedTheme(): Theme | null {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    return value === 'light' || value === 'dark' ? value : null
  } catch {
    return null
  }
}
function systemTheme(): Theme {
  return matchMedia(SYSTEM_QUERY).matches ? 'dark' : 'light'
}
function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState<Theme | null>(storedTheme)
  const [system, setSystem] = useState<Theme>(systemTheme)
  const theme = preference ?? system

  useLayoutEffect(() => {
    applyTheme(theme)
  }, [theme])
  const setTheme = useCallback((next: Theme) => {
    // Apply tokens before notifying chart consumers, which read computed colors during render.
    applyTheme(next)
    setPreference(next)
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      /* Keep the current page usable without storage. */
    }
  }, [])
  useEffect(() => {
    const media = matchMedia(SYSTEM_QUERY)
    const changed = () => {
      const next = media.matches ? 'dark' : 'light'
      if (preference === null) applyTheme(next)
      setSystem(next)
    }
    media.addEventListener('change', changed)
    return () => media.removeEventListener('change', changed)
  }, [preference])
  useEffect(() => {
    const sync = (event: StorageEvent) => {
      if (event.key !== STORAGE_KEY && event.key !== null) return
      const next = storedTheme()
      const currentSystem = systemTheme()
      applyTheme(next ?? currentSystem)
      setSystem(currentSystem)
      setPreference(next)
    }
    window.addEventListener('storage', sync)
    return () => window.removeEventListener('storage', sync)
  }, [])

  const value = useMemo(() => ({ theme, setTheme }), [theme, setTheme])
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

export function useTheme() {
  const context = useContext(ThemeContext)
  if (!context) throw new Error('ThemeProvider is required')
  return context
}
