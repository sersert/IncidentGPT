import { useEffect, useState, useMemo } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { Search, X, AlertCircle, Layers } from 'lucide-react'
import type { Incident, IncidentStatus } from '@/api/types.ts'
import { SeverityBadge, SeverityDot } from '@/components/ui/Badge.tsx'
import { EmptyState } from '@/components/ui/EmptyState.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { Button } from '@/components/ui/Button.tsx'
import { Tooltip } from '@/components/ui/Tooltip.tsx'
import { useIncidents } from '@/hooks/useIncidents.ts'
import { timeAgo, formatDate, formatDuration } from '@/lib/time.ts'
import { countOf, words } from '@/lib/plural.ts'

const STATUS_FACETS: { value: IncidentStatus | ''; label: string }[] = [
  { value: 'active', label: 'Активные' },
  { value: 'acknowledged', label: 'Принятые' },
  { value: 'resolved', label: 'Закрытые' },
  { value: '', label: 'Все' },
]

/** Кусок текста, совпавший с запросом, подсвечивается — иначе непонятно, за что нашлось. */
function Highlight({ text, query }: { text: string; query: string }) {
  if (!query.trim()) return <>{text}</>
  const i = text.toLowerCase().indexOf(query.toLowerCase())
  if (i === -1) return <>{text}</>
  return (
    <>
      {text.slice(0, i)}
      <mark className="rounded-[3px] bg-accent/25 px-0.5 text-text-primary">{text.slice(i, i + query.length)}</mark>
      {text.slice(i + query.length)}
    </>
  )
}

function IncidentRow({ incident, query, onOpen }: { incident: Incident; query: string; onOpen: () => void }) {
  const isOpen = incident.status === 'active'
  const chips = [
    incident.namespace,
    incident.source,
    incident.log_errors > 0 ? `логи ${incident.log_errors}` : null,
    incident.duration_sec !== null ? `закрыт за ${formatDuration(incident.duration_sec)}` : null,
    incident.confidence !== null ? `AI ${incident.confidence}%` : null,
    incident.has_postmortem ? 'post mortem' : null,
  ].filter(Boolean) as string[]

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onOpen()
        }
      }}
      className={`grid cursor-pointer grid-cols-[auto_1fr_auto] items-start gap-3.5 rounded-[10px] border border-border-default bg-surface-secondary px-4 py-3.5 transition-colors hover:border-[#333A52] hover:bg-surface-tertiary ${
        incident.status === 'resolved' ? 'opacity-60' : ''
      }`}
    >
      <SeverityDot severity={incident.severity} halo={isOpen} className="mt-1.5" />

      <div className="min-w-0">
        <h3 className="text-[14.5px] font-semibold tracking-[-0.01em]">
          <Highlight text={incident.title} query={query} />
        </h3>
        {incident.analysis_summary && (
          <p className="mt-1 truncate text-[13px] text-text-secondary">
            <Highlight text={incident.analysis_summary} query={query} />
          </p>
        )}
        <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
          <SeverityBadge severity={incident.severity} />
          {/* Сколько алертов склеено — главный признак, что это группа, а не один алерт. */}
          {incident.alert_count > 1 && (
            <span className="inline-flex items-center gap-1 rounded-md bg-accent/12 px-[7px] py-0.5 text-[11px] font-medium text-[#B9B9FF]">
              <Layers size={10} />
              {countOf(incident.alert_count, words.alert)}
              {incident.discarded_count > 0 && (
                <span className="text-text-muted">· {incident.discarded_count} отброшено</span>
              )}
            </span>
          )}
          {chips.map((c) => (
            <span key={c} className="rounded-md bg-surface-tertiary px-[7px] py-0.5 text-[11px] text-text-muted">
              {c}
            </span>
          ))}
        </div>
      </div>

      <Tooltip content={formatDate(incident.created_at)}>
        <span className="tnum whitespace-nowrap pt-0.5 text-xs text-text-muted">{timeAgo(incident.created_at)}</span>
      </Tooltip>
    </div>
  )
}

export function IncidentList() {
  const [searchParams, setSearchParams] = useSearchParams()
  const navigate = useNavigate()
  const [search, setSearch] = useState(searchParams.get('q') || '')

  const status = searchParams.get('status') || ''
  const { data, isLoading, isError, error, refetch } = useIncidents(searchParams.toString())
  const incidents = useMemo(() => data?.data ?? [], [data])

  // Поиск уезжает в URL с задержкой, чтобы не дёргать бэкенд на каждую букву.
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearchParams(
        (prev) => {
          const p = new URLSearchParams(prev)
          if (search) p.set('q', search)
          else p.delete('q')
          return p
        },
        { replace: true },
      )
    }, 300)
    return () => clearTimeout(timer)
  }, [search, setSearchParams])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const tag = document.activeElement?.tagName
      if (e.key === '/' && tag !== 'INPUT' && tag !== 'TEXTAREA') {
        e.preventDefault()
        document.getElementById('incident-search')?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const setStatus = (value: string) => {
    setSearchParams(
      (prev) => {
        const p = new URLSearchParams(prev)
        if (value) p.set('status', value)
        else p.delete('status')
        p.delete('page')
        return p
      },
      { replace: true },
    )
  }

  // Счётчики фасетов считаем по загруженной странице — показываем объём до клика.
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const i of incidents) c[i.status] = (c[i.status] ?? 0) + 1
    return c
  }, [incidents])

  const visible = useMemo(
    () => (status ? incidents.filter((i) => i.status === status) : incidents),
    [incidents, status],
  )

  const activeCount = counts.active ?? 0
  const criticalCount = incidents.filter((i) => i.severity === 'critical' && i.status === 'active').length

  return (
    <div>
      <h1 className="text-[22px] font-semibold tracking-[-0.02em]">Инциденты</h1>
      <p className="mt-0.5 mb-5 text-[13px] text-text-muted">
        {isLoading
          ? 'Загружаем…'
          : `${countOf(activeCount, words.active)}${criticalCount > 0 ? `, ${countOf(criticalCount, words.critical)}` : ''} · всего ${data?.total ?? incidents.length}`}
      </p>

      <div className="mb-2.5 flex flex-wrap items-center gap-2">
        {STATUS_FACETS.map((f) => (
          <button
            key={f.label}
            onClick={() => setStatus(f.value)}
            className={`rounded-full border px-[11px] py-[5px] text-xs transition-colors ${
              status === f.value
                ? 'border-accent/30 bg-accent/12 text-[#B9B9FF]'
                : 'border-border-default bg-surface-secondary text-text-secondary hover:text-text-primary'
            }`}
          >
            {f.label}
            {f.value && counts[f.value] ? <span className="tnum ml-1.5 opacity-70">{counts[f.value]}</span> : null}
          </button>
        ))}

        <div className="relative ml-auto min-w-[250px]">
          <Search size={12} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-text-muted" />
          <input
            id="incident-search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                setSearch('')
                ;(e.target as HTMLInputElement).blur()
              }
            }}
            placeholder="Поиск по инцидентам…"
            aria-label="Поиск по инцидентам"
            className="w-full rounded-full border border-border-default bg-surface-secondary py-1.5 pl-8 pr-8 text-xs text-text-primary outline-none placeholder:text-text-muted focus:border-accent/50"
          />
          {search && (
            <button
              onClick={() => setSearch('')}
              aria-label="Очистить поиск"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-primary"
            >
              <X size={12} />
            </button>
          )}
        </div>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-[104px] rounded-[10px]" />
          ))}
        </div>
      ) : isError ? (
        <div className="flex flex-col items-center gap-3 rounded-[10px] border border-severity-critical/30 bg-severity-critical/5 py-12">
          <AlertCircle size={22} className="text-severity-critical" />
          <p className="text-sm text-text-secondary">
            Не удалось загрузить инциденты{error instanceof Error ? `: ${error.message}` : ''}
          </p>
          <Button variant="secondary" size="sm" onClick={() => refetch()}>
            Повторить
          </Button>
        </div>
      ) : visible.length === 0 ? (
        <EmptyState
          title="Инцидентов нет"
          description={search || status ? 'Под фильтры ничего не подошло' : 'Всё спокойно — активных инцидентов нет'}
        />
      ) : (
        <div className="flex flex-col gap-2">
          {visible.map((inc) => (
            <IncidentRow
              key={inc.id}
              incident={inc}
              query={search}
              onOpen={() => navigate(`/incidents/${inc.id}`)}
            />
          ))}
        </div>
      )}
    </div>
  )
}
