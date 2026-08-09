import { useState } from 'react'
import {
  LineChart, Line, BarChart, Bar, XAxis, YAxis, Tooltip as ReTooltip,
  ResponsiveContainer, CartesianGrid,
} from 'recharts'
import { Card } from '@/components/ui/Card.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { EmptyState } from '@/components/ui/EmptyState.tsx'
import type { MTTRDataPoint } from '@/api/types.ts'
import { useAnalyticsSummary, useMTTRTrend, useAnalyticsQuality, useTopSources } from '@/hooks/useAnalytics.ts'
import { chartColors, tooltipStyle, axisTick } from '@/lib/colors.ts'

const PERIODS = [
  { id: '7d', label: '7 дней' },
  { id: '30d', label: '30 дней' },
  { id: '90d', label: '90 дней' },
]

/** Один ряд, одна ось, одна единица измерения. */
function TrendChart({
  title,
  hint,
  data,
  dataKey,
  unit,
  color,
}: {
  title: string
  hint: string
  data: MTTRDataPoint[]
  dataKey: 'analysis_sec' | 'close_min'
  unit: string
  color: string
}) {
  const points = data.filter((d) => d[dataKey] !== undefined)
  if (points.length === 0) {
    return (
      <Card title={title} action={<span className="text-xs text-text-muted">{hint}</span>}>
        <p className="py-8 text-center text-[13px] text-text-muted">Пока нет данных за этот период</p>
      </Card>
    )
  }

  return (
    <Card title={title} action={<span className="text-xs text-text-muted">{hint}</span>}>
      <ResponsiveContainer width="100%" height={220}>
        <LineChart data={points} margin={{ top: 6, right: 10, left: -14, bottom: 0 }}>
          <CartesianGrid stroke={chartColors.grid} vertical={false} />
          <XAxis
            dataKey="date"
            tick={axisTick}
            tickLine={false}
            axisLine={false}
            tickFormatter={(d: string) => d.slice(5)}
          />
          <YAxis
            tick={axisTick}
            tickLine={false}
            axisLine={false}
            width={52}
            allowDecimals={false}
            tickFormatter={(v: number) => `${Math.round(v)}${unit}`}
          />
          <ReTooltip contentStyle={tooltipStyle} formatter={(v: number) => [`${v}${unit}`, title]} />
          {/* Одна точка линией не нарисуется — показываем её маркером. */}
          <Line
            type="monotone"
            dataKey={dataKey}
            stroke={color}
            strokeWidth={2}
            dot={points.length === 1 ? { r: 4, fill: color } : false}
            activeDot={{ r: 5 }}
            name={title}
          />
        </LineChart>
      </ResponsiveContainer>
    </Card>
  )
}

export function Analytics() {
  const [period, setPeriod] = useState('30d')
  const { data: summary, isLoading } = useAnalyticsSummary(period)
  const { data: mttr } = useMTTRTrend(period)
  const { data: quality } = useAnalyticsQuality(period)
  const { data: topSources = [] } = useTopSources(period)

  // Только операционные показатели. «MTTR до/после» и «сэкономлено 47 часов» —
  // это слайд для руководства, а не то, по чему инженер принимает решения.
  const kpis = summary
    ? [
        { label: 'Алертов пришло', value: `${summary.noise_before}`, hint: 'за период' },
        { label: 'Стало инцидентов', value: `${summary.noise_after}`, hint: `склеено ${summary.noise_reduction_pct}%` },
        {
          label: 'Среднее время до разбора',
          value: quality ? `${quality.avg_response_time_sec} с` : '—',
          hint: 'от алерта до ответа модели',
        },
        {
          label: 'Среднее время до закрытия',
          value: summary.mttr_after_min > 0 ? `${summary.mttr_after_min} мин` : '—',
          hint: 'от алерта до resolved',
        },
      ]
    : []

  return (
    <div>
      <div className="mb-5 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-[22px] font-semibold tracking-[-0.02em]">Аналитика</h1>
          <p className="mt-0.5 text-[13px] text-text-muted">Поток алертов, шумные источники и качество разбора</p>
        </div>
        <div className="flex gap-1.5">
          {PERIODS.map((p) => (
            <button
              key={p.id}
              onClick={() => setPeriod(p.id)}
              className={`rounded-full border px-[11px] py-[5px] text-xs transition-colors ${
                period === p.id
                  ? 'border-accent/30 bg-accent/12 text-[#B9B9FF]'
                  : 'border-border-default bg-surface-secondary text-text-secondary hover:text-text-primary'
              }`}
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      {isLoading ? (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-[104px] rounded-[10px]" />)}
        </div>
      ) : !summary ? (
        <EmptyState title="Данных пока нет" description="Аналитика появится, когда накопится история инцидентов" />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {kpis.map((kpi) => (
            <div key={kpi.label} className="rounded-[10px] border border-border-default bg-surface-secondary px-4 py-[15px]">
              <p className="text-xs text-text-muted">{kpi.label}</p>
              <p className="tnum mt-2 text-[26px] font-semibold leading-none tracking-[-0.03em]">{kpi.value}</p>
              <p className="mt-2 text-[11.5px] text-text-muted">{kpi.hint}</p>
            </div>
          ))}
        </div>
      )}

      {/* Два графика вместо одного: разбор измеряется секундами, закрытие —
          минутами, и на общей оси секунды сплющивались бы в ноль. */}
      {mttr && mttr.data?.length > 0 && (
        <div className="mt-3.5 grid items-start gap-3.5 lg:grid-cols-2">
          <TrendChart
            title="Время до разбора"
            hint="от появления инцидента до ответа модели"
            data={mttr.data}
            dataKey="analysis_sec"
            unit=" с"
            color={chartColors.accent}
          />
          <TrendChart
            title="Время до закрытия"
            hint="от появления инцидента до статуса resolved"
            data={mttr.data}
            dataKey="close_min"
            unit=" мин"
            color={chartColors.info}
          />
        </div>
      )}

      <div className="mt-3.5 grid items-start gap-3.5 lg:grid-cols-2">
        {topSources.length > 0 && (
          <Card title="Самые шумные источники">
            <ResponsiveContainer width="100%" height={250}>
              <BarChart data={topSources} layout="vertical" margin={{ top: 0, right: 12, left: 0, bottom: 0 }}>
                <CartesianGrid stroke={chartColors.grid} horizontal={false} />
                <XAxis type="number" tick={axisTick} tickLine={false} axisLine={false} />
                <YAxis
                  type="category"
                  dataKey="source"
                  tick={{ fill: 'var(--color-text-secondary)', fontSize: 11 }}
                  tickLine={false}
                  axisLine={false}
                  width={112}
                />
                <ReTooltip contentStyle={tooltipStyle} cursor={{ fill: chartColors.surface, opacity: 0.4 }} />
                <Bar dataKey="count" name="Алертов" fill={chartColors.info} radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </Card>
        )}

        {quality && (
          <Card title="Качество анализа">
            {quality.total_analyses < 10 ? (
              <p className="py-6 text-center text-[13px] text-text-muted">
                Пока мало оценок — нужно хотя бы 10, сейчас {quality.total_analyses}
              </p>
            ) : (
              <>
                <div className="grid grid-cols-3 gap-3">
                  {[
                    { v: quality.helpful_pct, l: 'полезен', c: 'text-status-resolved' },
                    { v: quality.not_helpful_pct, l: 'бесполезен', c: 'text-severity-critical' },
                    { v: quality.edited_pct, l: 'правили', c: 'text-severity-warning' },
                  ].map((x) => (
                    <div key={x.l}>
                      <p className={`tnum text-xl font-semibold ${x.c}`}>{x.v}%</p>
                      <p className="mt-0.5 text-[11px] text-text-muted">{x.l}</p>
                    </div>
                  ))}
                </div>
                <dl className="mt-4 space-y-2 border-t border-border-soft pt-3 text-xs">
                  {[
                    ['Средняя уверенность', `${quality.avg_confidence}%`],
                    ['Время до анализа', `${quality.avg_response_time_sec} с`],
                    ['Всего разборов', `${quality.total_analyses}`],
                  ].map(([k, v]) => (
                    <div key={k} className="flex justify-between">
                      <dt className="text-text-secondary">{k}</dt>
                      <dd className="tnum">{v}</dd>
                    </div>
                  ))}
                </dl>
              </>
            )}
          </Card>
        )}
      </div>
    </div>
  )
}
