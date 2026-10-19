import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Search, X, AlertCircle, RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react'
import type { Incident } from '@/api/types.ts'
import { SeverityBadge, StatusBadge } from '@/components/ui/Badge.tsx'
import { EmptyState } from '@/components/ui/EmptyState.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { Button } from '@/components/ui/Button.tsx'
import { useIncidents } from '@/hooks/useIncidents.ts'
import { formatDate, timeAgo } from '@/lib/time.ts'
import { countOf, words } from '@/lib/plural.ts'

const statuses = [
  { value: '', label: 'Все' },
  { value: 'active', label: 'Активные' },
  { value: 'acknowledged', label: 'В работе' },
  { value: 'resolved', label: 'Закрытые' },
  { value: 'muted', label: 'Заглушённые' },
]
const PAGE_SIZE = 25

function Highlight({ text, query }: { text: string; query: string }) {
  const index = query.trim() ? text.toLowerCase().indexOf(query.toLowerCase()) : -1
  if (index < 0) return <>{text}</>
  return (
    <>
      {text.slice(0, index)}
      <mark className="rounded bg-accent/20 text-text-primary">
        {text.slice(index, index + query.length)}
      </mark>
      {text.slice(index + query.length)}
    </>
  )
}

function IncidentRow({ incident, query }: { incident: Incident; query: string }) {
  return (
    <Link
      to={`/incidents/${encodeURIComponent(incident.id)}`}
      className="group flex min-w-0 flex-col gap-3 border-b border-border-soft px-5 py-4 transition-colors last:border-0 hover:bg-surface-primary md:grid md:grid-cols-[minmax(0,1fr)_110px_90px_100px] md:items-center md:gap-5"
    >
      <div className="min-w-0">
        <div className="mb-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-text-muted">
          <span className="font-mono">{incident.id}</span>
          <span className="break-all">{incident.namespace || 'namespace неизвестен'}</span>
          <span>{countOf(incident.alert_count, words.alert)}</span>
          {incident.has_postmortem && <span>Постмортем</span>}
        </div>
        <h2 className="break-words text-[14px] font-medium tracking-[-.01em] group-hover:text-accent">
          <Highlight text={incident.title} query={query} />
        </h2>
        {incident.analysis_summary && (
          <p className="mt-1 line-clamp-1 text-xs leading-relaxed text-text-muted">
            <Highlight text={incident.analysis_summary} query={query} />
          </p>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-3 md:contents">
        <span>
          <StatusBadge status={incident.status} />
        </span>
        <span>
          <SeverityBadge severity={incident.severity} />
        </span>
        <time
          dateTime={incident.created_at}
          title={formatDate(incident.created_at)}
          className="tnum ml-auto text-[11px] text-text-muted md:ml-0"
        >
          {timeAgo(incident.created_at)}
        </time>
      </div>
    </Link>
  )
}

export function IncidentList() {
  const [params, setParams] = useSearchParams()
  const query = params.get('q') ?? ''
  const [search, setSearch] = useState(query)
  const status = statuses.some((item) => item.value === params.get('status'))
    ? (params.get('status') ?? '')
    : ''
  const severity = ['critical', 'warning', 'info', 'unknown'].includes(params.get('severity') ?? '')
    ? params.get('severity')!
    : ''
  const rawOffset = Number(params.get('offset') ?? 0)
  const offset = Number.isSafeInteger(rawOffset) && rawOffset >= 0 ? rawOffset : 0
  const apiParams = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(offset) })
  if (query) apiParams.set('q', query)
  if (status) apiParams.set('status', status)
  if (severity) apiParams.set('severity', severity)
  const { data, isLoading, isError, isFetching, refetch } = useIncidents(apiParams.toString())
  const incidents = useMemo(() => data?.data ?? [], [data])

  useEffect(() => {
    setSearch(query)
  }, [query])
  useEffect(() => {
    if (search === query) return
    const timer = setTimeout(
      () =>
        setParams(
          (previous) => {
            const next = new URLSearchParams(previous)
            if (search.trim()) next.set('q', search.trim())
            else next.delete('q')
            next.delete('offset')
            return next
          },
          { replace: true },
        ),
      300,
    )
    return () => clearTimeout(timer)
  }, [search, query, setParams])
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (
        event.key === '/' &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey &&
        !(
          event.target instanceof HTMLElement &&
          (event.target.isContentEditable || /INPUT|TEXTAREA|SELECT/.test(event.target.tagName))
        )
      ) {
        event.preventDefault()
        document.getElementById('incident-search')?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  const change = (key: string, value: string) =>
    setParams((previous) => {
      const next = new URLSearchParams(previous)
      if (value) next.set(key, value)
      else next.delete(key)
      if (key !== 'offset') next.delete('offset')
      return next
    })
  const reset = () => {
    setSearch('')
    setParams({})
  }
  const total = data?.total ?? 0

  return (
    <div>
      <div className="mb-7 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="page-title">Инциденты</h1>
          <p className="page-description hidden sm:block">Текущая работа и история разборов.</p>
        </div>
        <Button variant="secondary" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw size={14} className={isFetching ? 'animate-spin' : ''} />
          Обновить
        </Button>
      </div>
      <div className="mb-5 border-b border-border-default pb-0">
        <div className="flex flex-wrap gap-3">
          <div className="relative min-w-0 flex-1 basis-60 md:max-w-md">
            <Search size={17} className="pointer-events-none absolute top-3 left-3 text-text-muted" />
            <input
              id="incident-search"
              type="search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Escape') setSearch('')
              }}
              placeholder="Название или текст анализа…"
              aria-label="Поиск по инцидентам"
              className="h-11 w-full rounded-md border border-border-default bg-surface-secondary pr-10 pl-10 text-sm placeholder:text-text-muted focus:border-accent"
            />
            {search && (
              <button
                onClick={() => setSearch('')}
                aria-label="Очистить поиск"
                className="absolute top-0 right-0 grid size-11 place-items-center text-text-secondary"
              >
                <X size={15} />
              </button>
            )}
          </div>
          <label className="flex items-center gap-2 text-xs text-text-muted">
            <span className="sr-only">Критичность</span>
            <select
              aria-label="Критичность"
              value={severity}
              onChange={(event) => change('severity', event.target.value)}
              className="h-11 rounded-md border border-border-default bg-surface-secondary px-3 text-xs text-text-secondary"
            >
              <option value="">Любая критичность</option>
              <option value="critical">critical</option>
              <option value="warning">warning</option>
              <option value="info">info</option>
              <option value="unknown">unknown</option>
            </select>
          </label>
        </div>
        <div
          className="mt-4 flex items-center gap-3 overflow-x-auto"
          role="group"
          aria-label="Статус инцидента"
        >
          {statuses.map((item) => (
            <button
              key={item.label}
              onClick={() => change('status', item.value)}
              aria-pressed={status === item.value}
              className={`min-h-11 shrink-0 whitespace-nowrap border-b-2 px-2 text-xs font-medium ${status === item.value ? 'border-text-primary text-text-primary' : 'border-transparent text-text-muted hover:text-text-primary'}`}
            >
              {item.label}
            </button>
          ))}
          {(query || status || severity) && (
            <button
              onClick={reset}
              className="ml-auto min-h-9 px-2 text-xs text-text-muted hover:text-accent"
            >
              Сбросить фильтры
            </button>
          )}
        </div>
      </div>
      <p className="mb-3 text-xs text-text-muted" role="status">
        {isLoading
          ? 'Загружаем инциденты…'
          : isError
            ? 'Список недоступен'
            : `${total} по выбранным условиям · показано ${incidents.length ? offset + 1 : 0}–${offset + incidents.length}`}
      </p>
      {isLoading ? (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-40 rounded-xl" />
          ))}
        </div>
      ) : isError ? (
        <div
          role="alert"
          className="rounded-xl border border-severity-critical/30 bg-severity-critical/5 p-8 text-center"
        >
          <AlertCircle size={24} className="mx-auto mb-3 text-severity-critical" />
          <p>Не удалось загрузить инциденты.</p>
          <p className="mt-2 text-sm text-text-secondary">
            Проверьте подключение и доступ. Ошибка загрузки не означает, что инцидентов нет.
          </p>
          <Button className="mt-4" variant="secondary" onClick={() => refetch()}>
            Повторить
          </Button>
        </div>
      ) : incidents.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border-default bg-surface-secondary px-4">
          <EmptyState
            title="Инцидентов не найдено"
            description={
              query || status || severity
                ? 'Измените условия поиска или сбросьте фильтры.'
                : 'Новые группы алертов появятся здесь после получения данных.'
            }
          />
          {(query || status || severity || offset > 0) && (
            <div className="pb-6 text-center">
              <Button variant="secondary" onClick={reset}>
                Показать все инциденты
              </Button>
            </div>
          )}
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border-default bg-surface-secondary">
          <div className="hidden grid-cols-[minmax(0,1fr)_110px_90px_100px] gap-5 border-b border-border-soft px-5 py-3 text-[11px] text-text-muted md:grid">
            <span>Инцидент</span>
            <span>Статус</span>
            <span>Критичность</span>
            <span>Поступил</span>
          </div>
          {incidents.map((incident) => (
            <IncidentRow key={incident.id} incident={incident} query={query} />
          ))}
        </div>
      )}
      {!isError && total > PAGE_SIZE && (
        <nav aria-label="Страницы инцидентов" className="mt-5 flex items-center justify-between gap-3">
          <Button
            variant="secondary"
            disabled={offset === 0 || isFetching}
            onClick={() => change('offset', String(Math.max(0, offset - PAGE_SIZE)))}
          >
            <ChevronLeft size={15} />
            Назад
          </Button>
          <span className="tnum text-xs text-text-muted">
            {Math.floor(offset / PAGE_SIZE) + 1} / {Math.ceil(total / PAGE_SIZE)}
          </span>
          <Button
            variant="secondary"
            disabled={offset + PAGE_SIZE >= total || isFetching}
            onClick={() => change('offset', String(offset + PAGE_SIZE))}
          >
            Далее
            <ChevronRight size={15} />
          </Button>
        </nav>
      )}
    </div>
  )
}
