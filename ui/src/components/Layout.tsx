import { useEffect, useRef, useState } from 'react'
import { Link, Outlet, useLocation } from 'react-router-dom'
import { BookOpen, ChevronRight, Menu, X } from 'lucide-react'
import { ThemeToggle } from './ThemeToggle'
import { Sidebar } from './Sidebar.tsx'
import { useDashboardStats } from '@/hooks/useDashboard.ts'
import { usePredictions } from '@/hooks/usePredictions.ts'

const names: Record<string, string> = {
  incidents: 'Инциденты',
  alerts: 'Поток алертов',
  analytics: 'Аналитика',
  predictions: 'Прогнозы',
  settings: 'Настройки',
  help: 'Руководство',
}

export function Layout() {
  const { data: stats, isError } = useDashboardStats()
  const { data: predictions, isError: predictionsError } = usePredictions()
  const { pathname, hash } = useLocation()
  const [menuOpen, setMenuOpen] = useState(false)
  const dialog = useRef<HTMLDialogElement>(null)
  const section = names[pathname.split('/')[1]] ?? 'Обзор'

  useEffect(() => {
    if (hash) {
      const frame = requestAnimationFrame(() => document.getElementById(hash.slice(1))?.scrollIntoView())
      return () => cancelAnimationFrame(frame)
    }
    window.scrollTo({ top: 0, left: 0, behavior: 'instant' })
  }, [pathname, hash])
  useEffect(() => {
    document.title = `${section} · IncidentGPT`
  }, [section])
  useEffect(() => {
    const element = dialog.current
    if (menuOpen && !element?.open) element?.showModal()
    if (!menuOpen && element?.open) element.close()
    if (!menuOpen) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [menuOpen])

  return (
    <div className="flex min-h-dvh bg-surface-primary">
      <a href="#main-content" className="skip-link">
        Перейти к содержимому
      </a>
      <Sidebar
        activeIncidents={isError ? undefined : stats?.active_incidents}
        warnings={predictionsError ? undefined : predictions?.length}
      />
      <dialog
        ref={dialog}
        id="mobile-navigation"
        aria-label="Меню навигации"
        onClose={() => setMenuOpen(false)}
        onClick={(event) => {
          if (event.target === event.currentTarget) setMenuOpen(false)
        }}
        className="fixed inset-y-0 right-auto left-0 m-0 h-dvh max-h-none w-[min(320px,90vw)] max-w-none border-0 bg-surface-secondary p-0 text-text-primary backdrop:bg-black/70"
      >
        <button
          onClick={() => setMenuOpen(false)}
          autoFocus
          aria-label="Закрыть меню"
          className="absolute top-4 right-3 z-10 rounded-lg p-3 text-text-secondary hover:bg-surface-tertiary"
        >
          <X size={19} />
        </button>
        <div className="h-full overflow-y-auto pt-6">
          <Sidebar
            mobile
            onNavigate={() => setMenuOpen(false)}
            activeIncidents={isError ? undefined : stats?.active_incidents}
            warnings={predictionsError ? undefined : predictions?.length}
          />
        </div>
      </dialog>
      <div className="min-w-0 flex-1">
        <header className="sticky top-0 z-20 flex min-h-14 items-center gap-3 border-b border-border-soft bg-surface-secondary px-4 sm:px-7 lg:px-9">
          <button
            aria-label="Открыть меню"
            aria-controls="mobile-navigation"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen(true)}
            className="-ml-2 rounded-lg p-3 text-text-secondary hover:bg-surface-tertiary lg:hidden"
          >
            <Menu size={20} />
          </button>
          <nav
            aria-label="Текущий раздел"
            className="flex min-w-0 items-center gap-2 text-xs text-text-muted"
          >
            <Link to="/" className="hidden hover:text-text-primary sm:block">
              IncidentGPT
            </Link>
            <ChevronRight size={12} className="hidden sm:block" />
            <span className="truncate text-text-primary">{section}</span>
          </nav>
          <div className="ml-auto flex items-center gap-1 sm:gap-3">
            <ThemeToggle />
            <Link
              to="/help"
              className="flex min-h-10 items-center gap-2 rounded-lg px-2 text-xs text-text-secondary hover:bg-surface-tertiary hover:text-accent"
            >
              <BookOpen size={16} />
              <span className="hidden sm:block">Руководство</span>
              <span className="sr-only sm:hidden">Руководство</span>
            </Link>
          </div>
        </header>
        {import.meta.env.VITE_USE_MOCKS === 'true' && (
          <p className="border-b border-severity-warning/20 bg-severity-warning/10 px-5 py-2 text-center text-xs text-severity-warning">
            Демонстрационный режим · примеры данных, без подключения к инфраструктуре
          </p>
        )}
        <main
          id="main-content"
          tabIndex={-1}
          className="min-w-0 px-4 pt-8 pb-16 outline-none sm:px-8 lg:px-10 lg:pt-10"
        >
          <div className="mx-auto max-w-[1280px]">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
