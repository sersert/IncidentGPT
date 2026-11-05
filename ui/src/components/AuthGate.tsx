import { useCallback, useEffect, useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { ThemeToggle } from './ThemeToggle'

export function AuthGate({ children }: { children: ReactNode }) {
  const client = useQueryClient()
  const [state, setState] = useState<'loading' | 'signed-out' | 'signed-in'>(
    'loading',
  )
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [visible, setVisible] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const signOut = useCallback(() => {
    client.clear()
    setPassword('')
    setState('signed-out')
  }, [client])
  useEffect(() => {
    const controller = new AbortController()
    fetch('/api/v1/auth/session', {
      signal: controller.signal,
      cache: 'no-store',
    })
      .then((response) => setState(response.ok ? 'signed-in' : 'signed-out'))
      .catch(() => {
        if (!controller.signal.aborted) setState('signed-out')
      })
    window.addEventListener('incidentgpt:unauthorized', signOut)
    return () => {
      controller.abort()
      window.removeEventListener('incidentgpt:unauthorized', signOut)
    }
  }, [signOut])
  async function login(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      const response = await fetch('/api/v1/auth/session', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })
      if (!response.ok) {
        setError(
          response.status === 401
            ? 'Неверный логин или пароль. Попробуйте ещё раз.'
            : response.status === 429
              ? 'Слишком много попыток. Подождите минуту и повторите.'
              : 'Не удалось войти. Попробуйте ещё раз позже.',
        )
        return
      }
      setPassword('')
      setState('signed-in')
    } catch {
      setError('Нет соединения с сервером. Проверьте подключение и повторите.')
    } finally {
      setBusy(false)
    }
  }
  if (state === 'signed-in') return children
  return (
    <div className="login-shell">
      <header className="login-header">
        <a href="/" className="login-brand">
          IncidentGPT
        </a>
        <ThemeToggle />
      </header>
      <main className="login-main">
        <section className="login-panel" aria-labelledby="login-title">
          <h1 id="login-title">Вход в IncidentGPT</h1>
          {state === 'loading' ? (
            <p role="status">Проверяем сессию…</p>
          ) : (
            <form onSubmit={login} className="login-form">
              <label htmlFor="username">Логин</label>
              <input
                id="username"
                name="username"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                autoFocus
                aria-describedby={error ? 'login-error' : undefined}
              />
              <label htmlFor="password">Пароль</label>
              <div className="login-password">
                <input
                  id="password"
                  name="password"
                  type={visible ? 'text' : 'password'}
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  aria-describedby={error ? 'login-error' : undefined}
                />
                <button
                  type="button"
                  onClick={() => setVisible(!visible)}
                  aria-label={visible ? 'Скрыть пароль' : 'Показать пароль'}
                  aria-pressed={visible}
                >
                  {visible ? <EyeOff size={18} /> : <Eye size={18} />}
                </button>
              </div>
              {error && (
                <p role="alert" id="login-error" className="login-error">
                  {error}
                </p>
              )}
              <button className="login-submit" disabled={busy}>
                {busy ? 'Входим…' : 'Войти'}
              </button>
            </form>
          )}
          <p className="login-help">
            Доступ и восстановление пароля — у администратора.
          </p>
        </section>
      </main>
      <footer className="login-footer">
        <span>IncidentGPT</span>
        <a
          href="https://sersert.github.io/IncidentGPT/"
          target="_blank"
          rel="noreferrer"
        >
          Документация
        </a>
      </footer>
    </div>
  )
}

export function LogoutButton() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(false)
  return (
    <button
      type="button"
      disabled={busy}
      className="rounded-md px-3 py-2 text-xs text-text-secondary hover:bg-surface-tertiary"
      onClick={async () => {
        setBusy(true)
        setError(false)
        try {
          const response = await fetch('/api/v1/auth/session', {
            method: 'DELETE',
          })
          if (!response.ok) throw new Error('logout failed')
          window.dispatchEvent(new Event('incidentgpt:unauthorized'))
        } catch {
          setError(true)
        } finally {
          setBusy(false)
        }
      }}
    >
      {busy ? 'Выходим…' : error ? 'Повторить выход' : 'Выйти'}
    </button>
  )
}
