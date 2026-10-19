import { Link } from 'react-router-dom'
import { ArrowRight, ChevronRight, RefreshCw, Check } from 'lucide-react'
import { Button } from '@/components/ui/Button.tsx'
import { KPICard } from '@/components/KPICard.tsx'
import { SeverityBadge } from '@/components/ui/Badge.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { useDashboardStats, useActiveIncidents } from '@/hooks/useDashboard.ts'
import { formatTime, timeAgo, formatDuration } from '@/lib/time.ts'
import { countOf, words } from '@/lib/plural.ts'

const severityOrder = { critical: 0, warning: 1, info: 2, unknown: 3 }
export function Dashboard() {
  const statsQuery = useDashboardStats(),
    activeQuery = useActiveIncidents()
  const { data: stats, isLoading: statsLoading, isError: statsError } = statsQuery
  const { data: active = [], isLoading: activeLoading, isError: activeError } = activeQuery
  const oldest =
    !activeError && active.length
      ? Math.max(
          ...active.map((i) => Math.max(0, Math.floor((Date.now() - Date.parse(i.created_at)) / 1000))),
        )
      : null
  const prioritized = [...active].sort(
    (a, b) =>
      severityOrder[a.severity] - severityOrder[b.severity] ||
      Date.parse(a.created_at) - Date.parse(b.created_at),
  )
  const refreshing = statsQuery.isFetching || activeQuery.isFetching
  return (
    <div>
      <div className="mb-8 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="page-title">Обзор</h1>
          <p className="page-description hidden sm:block">Инциденты и входящие сигналы</p>
        </div>
        <div className="flex items-center gap-4">
          <span className="hidden text-xs text-text-muted sm:block">
            {statsError
              ? 'Обновление не удалось'
              : statsQuery.dataUpdatedAt
                ? `Обновлено в ${formatTime(new Date(statsQuery.dataUpdatedAt).toISOString())}`
                : 'Загрузка…'}
          </span>
          <Button
            variant="secondary"
            disabled={refreshing}
            onClick={() => {
              void statsQuery.refetch()
              void activeQuery.refetch()
            }}
          >
            <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />
            Обновить
          </Button>
        </div>
      </div>
      {statsError && (
        <p
          role="alert"
          className="mb-5 border-l-2 border-severity-critical bg-severity-critical/5 px-4 py-3 text-sm"
        >
          Метрики недоступны. Повторите загрузку или проверьте доступ.
        </p>
      )}
      <div className="metric-strip grid grid-cols-2 overflow-hidden rounded-lg border border-border-default bg-surface-secondary lg:grid-cols-4">
        <KPICard
          label="Активные инциденты"
          value={statsError ? '—' : (stats?.active_incidents ?? 0)}
          hint={
            statsError
              ? 'данные недоступны'
              : stats?.active_critical
                ? `${stats.active_critical} с critical`
                : 'критических нет'
          }
          tone={statsError ? 'mute' : stats?.active_critical ? 'bad' : 'mute'}
          loading={statsLoading}
        />
        <KPICard
          label="Алертов за сутки"
          value={statsError ? '—' : (stats?.alerts_today ?? 0)}
          hint={statsError ? 'данные недоступны' : 'получено от Alertmanager'}
          sparkline={statsError ? undefined : stats?.alerts_sparkline}
          loading={statsLoading}
        />
        <KPICard
          label="Время обработки анализа"
          value={
            !statsError && stats?.avg_analysis_time_sec != null
              ? formatDuration(stats.avg_analysis_time_sec)
              : '—'
          }
          hint={
            !statsError && stats?.avg_analysis_time_sec != null
              ? 'среднее по завершённым'
              : 'нет доступного измерения'
          }
          loading={statsLoading}
        />
        <KPICard
          label="Самый давний в выборке"
          value={oldest !== null ? formatDuration(oldest) : '—'}
          hint={
            activeError
              ? 'список недоступен'
              : oldest !== null
                ? 'с регистрации инцидента'
                : 'активных в выборке нет'
          }
          loading={activeLoading}
        />
      </div>
      <section
        aria-labelledby="active-heading"
        className="mt-8 overflow-hidden rounded-lg border border-border-default bg-surface-secondary"
      >
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border-soft px-5 py-4">
          <h2 id="active-heading" className="text-[15px] font-semibold tracking-tight">
            Требуют внимания
          </h2>
          <Link
            to="/incidents?status=active"
            className="inline-flex min-h-9 items-center gap-2 text-xs text-text-secondary hover:text-accent"
          >
            Все активные
            <ArrowRight size={14} />
          </Link>
        </div>
        {activeError ? (
          <div role="alert" className="p-6 text-sm text-text-secondary">
            Список недоступен. Отсутствие инцидентов не подтверждено.
            <Button variant="secondary" className="mt-3" size="sm" onClick={() => activeQuery.refetch()}>
              Повторить
            </Button>
          </div>
        ) : activeLoading ? (
          <div className="space-y-3 p-5">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-16" />
            ))}
          </div>
        ) : prioritized.length === 0 ? (
          <div className="px-5 py-12 text-center">
            <Check size={20} className="mx-auto mb-3 text-status-resolved" />
            <p className="text-sm font-medium">Активных инцидентов нет</p>
            <Link to="/alerts" className="mt-3 inline-block text-xs text-text-secondary hover:text-accent">
              Посмотреть входящие алерты →
            </Link>
          </div>
        ) : (
          <>
            <div className="hidden grid-cols-[90px_minmax(0,1fr)_125px_110px_16px] gap-4 border-b border-border-soft px-5 py-3 text-[11px] text-text-muted md:grid">
              <span>Критичность</span>
              <span>Инцидент</span>
              <span>Namespace</span>
              <span>Поступил</span>
              <span />
            </div>
            <div className="divide-y divide-border-soft">
              {prioritized.map((incident) => (
                <Link
                  key={incident.id}
                  to={`/incidents/${encodeURIComponent(incident.id)}`}
                  className="group grid items-center gap-x-4 gap-y-2 px-5 py-4 transition-colors hover:bg-surface-primary md:grid-cols-[90px_minmax(0,1fr)_125px_110px_16px]"
                >
                  <span className="hidden md:block">
                    <SeverityBadge severity={incident.severity} />
                  </span>
                  <div className="min-w-0">
                    <h2 className="break-words text-[13px] font-medium group-hover:text-accent">
                      {incident.title}
                    </h2>
                    <p className="mt-1 text-[11px] text-text-muted">
                      <span className="font-mono">{incident.id}</span>
                      <span className="mx-2">/</span>
                      {countOf(incident.alert_count, words.alert)}
                    </p>
                  </div>
                  <span className="hidden truncate text-xs text-text-secondary md:block">
                    {incident.namespace || 'неизвестен'}
                  </span>
                  <time dateTime={incident.created_at} className="hidden text-xs text-text-muted md:block">
                    {timeAgo(incident.created_at)}
                  </time>
                  <ChevronRight size={14} className="hidden text-text-muted md:block" />
                  <div className="flex flex-wrap items-center gap-3 text-[11px] text-text-muted md:hidden">
                    <SeverityBadge severity={incident.severity} />
                    <span className="break-all">{incident.namespace || 'namespace неизвестен'}</span>
                    <span className="ml-auto">{timeAgo(incident.created_at)}</span>
                  </div>
                </Link>
              ))}
            </div>
          </>
        )}
        <p className="border-t border-border-soft px-5 py-3 text-[11px] text-text-muted">
          До 10 последних активных инцидентов, по критичности и времени регистрации.
        </p>
      </section>
      <div className="mt-5 flex flex-wrap items-center gap-x-6 gap-y-3 text-xs text-text-muted">
        <Link to="/incidents" className="hover:text-accent">
          История инцидентов →
        </Link>
        <Link to="/alerts" className="hover:text-accent">
          Входящие алерты →
        </Link>
      </div>
    </div>
  )
}
