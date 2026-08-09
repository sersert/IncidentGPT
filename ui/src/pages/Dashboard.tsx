import { Link } from 'react-router-dom'
import { CheckCircle2, ArrowRight } from 'lucide-react'
import { Card } from '@/components/ui/Card.tsx'
import { KPICard } from '@/components/KPICard.tsx'
import { SeverityBadge, SeverityDot } from '@/components/ui/Badge.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { useDashboardStats, useActiveIncidents } from '@/hooks/useDashboard.ts'
import { timeAgo, formatDuration } from '@/lib/time.ts'
import { plural } from '@/lib/plural.ts'

export function Dashboard() {
  const { data: stats, isLoading: statsLoading } = useDashboardStats()
  const { data: active = [], isLoading: activeLoading } = useActiveIncidents()

  // Самый старый активный инцидент — тот, к которому дежурному идти первым.
  const oldest = active.length
    ? active
        .map((i) => ({
          title: i.title,
          ageSec: Math.floor((Date.now() - new Date(i.created_at).getTime()) / 1000),
        }))
        .sort((a, b) => b.ageSec - a.ageSec)[0]
    : null

  // Всплеск считаем по спарклайну: последняя точка заметно выше предыдущей.
  const spark = stats?.alerts_sparkline ?? []
  const surge = spark.length >= 2 && spark[spark.length - 1] > spark[spark.length - 2] * 1.5

  return (
    <div>
      <h1 className="text-[22px] font-semibold tracking-[-0.02em]">Дашборд</h1>
      <p className="mt-0.5 mb-5 text-[13px] text-text-muted">Что происходит прямо сейчас</p>

      {/* Каждая карточка отвечает на вопрос дежурного, а не показывает достижение:
          что горит → к чему идти первым → жив ли AI-пайплайн → шторм или норма. */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <KPICard
          label="Активные инциденты"
          value={stats?.active_incidents ?? 0}
          hint={
            stats?.active_critical
              ? `${stats.active_critical} ${plural(stats.active_critical, 'критический', 'критических', 'критических')}`
              : 'критических нет'
          }
          tone={stats?.active_critical ? 'bad' : 'good'}
          loading={statsLoading}
        />
        <KPICard
          label="Дольше всех горит"
          value={oldest ? formatDuration(oldest.ageSec) : '—'}
          hint={oldest ? oldest.title : 'активных нет'}
          tone={oldest && oldest.ageSec > 1800 ? 'bad' : oldest ? 'warn' : 'good'}
          loading={activeLoading}
        />
        <KPICard
          label="Время до анализа"
          value={formatDuration(stats?.avg_analysis_time_sec ?? 0)}
          hint={stats && stats.avg_analysis_time_sec > 300 ? 'дольше нормы в 5 мин' : 'в пределах нормы'}
          tone={stats && stats.avg_analysis_time_sec > 300 ? 'warn' : 'good'}
          loading={statsLoading}
        />
        <KPICard
          label="Алертов за сутки"
          value={stats?.alerts_today ?? 0}
          hint={surge ? 'всплеск против вчерашнего' : 'ровный поток'}
          tone={surge ? 'warn' : 'mute'}
          sparkline={spark}
          loading={statsLoading}
        />
      </div>

      <Card
        className="mt-3.5"
        title="Активные инциденты"
        action={
          <Link to="/incidents" className="inline-flex items-center gap-1 text-xs text-accent hover:underline">
            Все инциденты <ArrowRight size={11} />
          </Link>
        }
      >
        {activeLoading ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }, (_, i) => (
              <Skeleton key={i} className="h-11" />
            ))}
          </div>
        ) : active.length === 0 ? (
          <div className="flex items-center justify-center gap-2 py-10 text-status-resolved">
            <CheckCircle2 size={18} /> Всё спокойно — активных инцидентов нет
          </div>
        ) : (
          <div className="-mx-2">
            {active.map((inc) => (
              <Link
                key={inc.id}
                to={`/incidents/${inc.id}`}
                className="flex items-center gap-3 rounded-lg px-2 py-2.5 transition-colors hover:bg-surface-tertiary"
              >
                <SeverityDot severity={inc.severity} halo />
                <SeverityBadge severity={inc.severity} />
                <span className="min-w-0 flex-1 truncate text-[13.5px]">{inc.title}</span>
                <span className="hidden shrink-0 text-xs text-text-muted sm:block">{inc.namespace}</span>
                <span className="tnum shrink-0 text-xs text-text-muted">{timeAgo(inc.created_at)}</span>
              </Link>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
