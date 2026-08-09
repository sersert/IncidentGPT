import { CheckCircle2, RefreshCw } from 'lucide-react'
import { Card } from '@/components/ui/Card.tsx'
import { Button } from '@/components/ui/Button.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { EmptyState } from '@/components/ui/EmptyState.tsx'
import { usePredictions, usePredictionSummary } from '@/hooks/usePredictions.ts'
import { countOf, words } from '@/lib/plural.ts'

export function Predictions() {
  const { data: predictions = [], isLoading, isFetching, refetch } = usePredictions()
  const { data: summary } = usePredictionSummary()

  return (
    <div>
      <div className="mb-5 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-[22px] font-semibold tracking-[-0.02em]">Прогнозы</h1>
          <p className="mt-0.5 text-[13px] text-text-muted">
            Ресурсы, которые упрутся в предел, если тренд сохранится
          </p>
        </div>
        <Button variant="secondary" size="sm" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw size={12} className={isFetching ? 'animate-spin' : ''} />
          Обновить
        </Button>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 2 }, (_, i) => <Skeleton key={i} className="h-28 rounded-[10px]" />)}
        </div>
      ) : predictions.length === 0 ? (
        <EmptyState
          icon={<CheckCircle2 size={36} className="text-status-resolved" />}
          title="Все ресурсы в норме"
          description="Ни один ресурс не приближается к пределу"
        />
      ) : (
        <div className="space-y-3">
          {predictions.map((p) => {
            // Классы перечислены целиком: Tailwind не видит имена, собранные из кусков.
            const critical = p.current_value > 90 || p.severity === 'critical'
            const textTone = critical ? 'text-severity-critical' : 'text-severity-warning'
            const barTone = critical ? 'bg-severity-critical' : 'bg-severity-warning'
            return (
              <Card key={p.id}>
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <span className="text-sm font-semibold">{p.resource}</span>
                  <span className={`tnum font-mono text-xs ${textTone}`}>
                    {p.current_value}
                    {p.current_unit} → {p.predicted_value}
                    {p.current_unit} · предел через {p.time_to_critical}
                  </span>
                </div>
                <div
                  className="mt-2.5 h-[6px] overflow-hidden rounded-full bg-surface-tertiary"
                  role="meter"
                  aria-valuenow={p.current_value}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-label={`${p.resource}: ${p.current_value}${p.current_unit}`}
                >
                  <span
                    className={`block h-full rounded-full ${barTone}`}
                    style={{ width: `${Math.min(p.current_value, 100)}%` }}
                  />
                </div>
                <p className="mt-2 text-[12.5px] text-text-secondary">{p.recommendation}</p>
                <p className="mt-1 text-[11.5px] text-text-muted">
                  рост {p.growth_rate} · {p.node} · {p.namespace}
                </p>
              </Card>
            )
          })}
        </div>
      )}

      {summary && (
        <Card
          className="mt-3.5"
          title="Загрузка нод"
          action={
            <span className="text-xs text-text-muted">
              под наблюдением {countOf(summary.watched_count ?? 0, words.resource)}: диски и память нод
            </span>
          }
        >
          <div className="mb-3 flex items-center gap-2 text-[13px] text-status-resolved">
            <CheckCircle2 size={15} />
            {countOf(summary.healthy_count, words.resource)} в пределах нормы
          </div>

          {/* Показываем каждую ноду отдельно: среднее по кластеру скрывает
              ситуацию, когда одна нода почти заполнена, а остальные пусты. */}
          <div className="overflow-x-auto">
            <table className="w-full text-left text-[13px]">
              <thead className="text-xs text-text-muted">
                <tr>
                  <th className="pb-2 font-medium">Нода</th>
                  <th className="pb-2 font-medium">CPU</th>
                  <th className="pb-2 font-medium">Память</th>
                  <th className="pb-2 font-medium">Диск (самая полная ФС)</th>
                </tr>
              </thead>
              <tbody>
                {(summary.nodes ?? []).map((n) => (
                  <tr key={n.node} className="border-t border-border-soft">
                    <td className="py-2 pr-4 font-mono text-xs">{n.node}</td>
                    {[n.cpu, n.memory, n.disk].map((v, i) => (
                      <td key={i} className="py-2 pr-4">
                        <div className="flex items-center gap-2">
                          <span className="h-1.5 w-16 shrink-0 overflow-hidden rounded-full bg-surface-tertiary">
                            <span
                              className={`block h-full rounded-full ${
                                v >= 90 ? 'bg-severity-critical' : v >= 75 ? 'bg-severity-warning' : 'bg-status-resolved'
                              }`}
                              style={{ width: `${Math.min(v, 100)}%` }}
                            />
                          </span>
                          <span className="tnum text-xs">{v}%</span>
                        </div>
                      </td>
                    ))}
                  </tr>
                ))}
                {(summary.nodes ?? []).length === 0 && (
                  <tr>
                    <td colSpan={4} className="py-3 text-xs text-text-muted">
                      Нет данных от Prometheus
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  )
}
