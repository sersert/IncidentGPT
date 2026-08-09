import type { ReactNode } from 'react'
import { Skeleton } from './ui/Skeleton.tsx'
import { chartColors } from '@/lib/colors.ts'

type Tone = 'good' | 'bad' | 'warn' | 'mute'

const toneStyles: Record<Tone, string> = {
  good: 'bg-status-resolved/12 text-status-resolved',
  bad: 'bg-severity-critical/12 text-severity-critical',
  warn: 'bg-severity-warning/12 text-severity-warning',
  mute: 'bg-surface-tertiary text-text-muted',
}

/** Спарклайн — форма ряда, а не точные значения: видно всплеск, не видно цифр. */
function Sparkline({ points }: { points: number[] }) {
  if (points.length < 2) return null
  const W = 96
  const H = 26
  const max = Math.max(...points)
  const min = Math.min(...points)
  const span = max - min || 1
  const step = W / (points.length - 1)
  const xy = points.map((v, i) => [i * step, H - ((v - min) / span) * (H - 3) - 1.5] as const)
  const line = xy.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
  const area = `${line} L${W},${H} L0,${H} Z`
  const [lastX, lastY] = xy[xy.length - 1]

  return (
    <svg width={W} height={H} viewBox={`0 0 ${W} ${H}`} className="mt-2.5 overflow-visible" aria-hidden="true">
      <path d={area} fill={chartColors.accent} opacity="0.12" />
      <path d={line} fill="none" stroke={chartColors.accent} strokeWidth="1.6" strokeLinejoin="round" strokeLinecap="round" />
      <circle cx={lastX} cy={lastY} r="2.4" fill={chartColors.accent} />
    </svg>
  )
}

interface KPICardProps {
  label: string
  value: ReactNode
  /** Подпись-пилюля под значением: уточнение, порог, что делать. */
  hint?: string
  tone?: Tone
  loading?: boolean
  /** Ряд для спарклайна — когда важна динамика, а не одно число. */
  sparkline?: number[]
}

export function KPICard({ label, value, hint, tone = 'mute', loading = false, sparkline }: KPICardProps) {
  return (
    <div className="rounded-[10px] border border-border-default bg-surface-secondary px-4 py-[15px]">
      <p className="text-xs text-text-muted">{label}</p>
      {loading ? (
        <Skeleton className="mt-2 h-[29px] w-20" />
      ) : (
        <p className="tnum mt-2 text-[29px] font-semibold leading-none tracking-[-0.03em]">{value}</p>
      )}
      {hint && !loading && (
        <span className={`mt-[9px] inline-flex items-center gap-1 rounded-full px-[7px] py-0.5 text-[11px] font-medium ${toneStyles[tone]}`}>
          {hint}
        </span>
      )}
      {sparkline && sparkline.length > 1 && !loading && <Sparkline points={sparkline} />}
    </div>
  )
}
