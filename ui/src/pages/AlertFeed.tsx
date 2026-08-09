import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { AlertCircle, Link2, Unlink } from 'lucide-react'
import { SeverityBadge, SeverityDot } from '@/components/ui/Badge.tsx'
import { EmptyState } from '@/components/ui/EmptyState.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { Button } from '@/components/ui/Button.tsx'
import { Tooltip } from '@/components/ui/Tooltip.tsx'
import { useAlerts } from '@/hooks/useAlerts.ts'
import { timeAgo, formatDate } from '@/lib/time.ts'
import { countOf, words } from '@/lib/plural.ts'

const FACETS = [
  { key: '', label: 'Все' },
  { key: 'grouped=true', label: 'В инцидентах' },
  { key: 'grouped=false', label: 'Без инцидента' },
  { key: 'status=firing', label: 'Горят' },
]

export function AlertFeed() {
  const [searchParams, setSearchParams] = useSearchParams()
  const active = useMemo(() => {
    const g = searchParams.get('grouped')
    const s = searchParams.get('status')
    if (g === 'true') return 'grouped=true'
    if (g === 'false') return 'grouped=false'
    if (s === 'firing') return 'status=firing'
    return ''
  }, [searchParams])

  const { data, isLoading, isError, refetch } = useAlerts(searchParams.toString())
  const alerts = data?.data ?? []

  const setFacet = (key: string) => {
    const p = new URLSearchParams()
    if (key) {
      const [k, v] = key.split('=')
      p.set(k, v)
    }
    setSearchParams(p, { replace: true })
  }

  return (
    <div>
      <h1 className="text-[22px] font-semibold tracking-[-0.02em]">Алерты</h1>
      <p className="mt-0.5 mb-5 text-[13px] text-text-muted">
        Сырой поток от систем мониторинга. Связанные алерты собираются в инциденты.
      </p>

      <div className="mb-2.5 flex flex-wrap items-center gap-2">
        {FACETS.map((f) => (
          <button
            key={f.label}
            onClick={() => setFacet(f.key)}
            className={`rounded-full border px-[11px] py-[5px] text-xs transition-colors ${
              active === f.key
                ? 'border-accent/30 bg-accent/12 text-[#B9B9FF]'
                : 'border-border-default bg-surface-secondary text-text-secondary hover:text-text-primary'
            }`}
          >
            {f.label}
          </button>
        ))}
        <span className="ml-auto text-xs text-text-muted">
          {isLoading ? 'Загружаем…' : countOf(data?.total ?? alerts.length, words.alert)}
        </span>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-1.5">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="h-[62px] rounded-[10px]" />
          ))}
        </div>
      ) : isError ? (
        <div className="flex flex-col items-center gap-3 rounded-[10px] border border-severity-critical/30 bg-severity-critical/5 py-12">
          <AlertCircle size={22} className="text-severity-critical" />
          <p className="text-sm text-text-secondary">Не удалось загрузить алерты</p>
          <Button variant="secondary" size="sm" onClick={() => refetch()}>
            Повторить
          </Button>
        </div>
      ) : alerts.length === 0 ? (
        <EmptyState title="Алертов нет" description="Под выбранный фильтр ничего не подошло" />
      ) : (
        <div className="flex flex-col gap-1.5">
          {alerts.map((a) => (
            <div
              key={a.id}
              className={`grid grid-cols-[auto_1fr_auto] items-start gap-3 rounded-[10px] border border-border-default bg-surface-secondary px-4 py-3 ${
                a.discarded ? 'opacity-55' : ''
              }`}
            >
              <SeverityDot severity={a.severity} halo={a.status === 'firing' && !a.discarded} className="mt-1.5" />

              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-[13.5px] font-semibold">{a.alertname}</span>
                  <SeverityBadge severity={a.severity} />
                  {a.discarded && (
                    <Tooltip content={a.discard_reason || 'Не относится к первопричине'}>
                      <span className="rounded-md bg-surface-tertiary px-[7px] py-0.5 text-[11px] text-text-muted">
                        отброшен как шум
                      </span>
                    </Tooltip>
                  )}
                </div>
                <p className="mt-1 truncate text-[12.5px] text-text-secondary">
                  {a.summary}
                  {a.description ? ` · ${a.description}` : ''}
                </p>
                <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-[11px] text-text-muted">
                  <span>{a.namespace || '—'}</span>
                  <span>·</span>
                  <span>{a.source}</span>
                  <span>·</span>
                  {a.incident_id ? (
                    <Link
                      to={`/incidents/${a.incident_id}`}
                      className="inline-flex items-center gap-1 text-accent hover:underline"
                    >
                      <Link2 size={10} />
                      {a.incident_id}
                    </Link>
                  ) : (
                    <span className="inline-flex items-center gap-1">
                      <Unlink size={10} />в инцидент не собран
                    </span>
                  )}
                </div>
              </div>

              <Tooltip content={formatDate(a.starts_at)}>
                <span className="tnum whitespace-nowrap pt-0.5 text-xs text-text-muted">{timeAgo(a.starts_at)}</span>
              </Tooltip>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
