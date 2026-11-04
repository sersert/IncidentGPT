import { useTheme } from './ThemeProvider'
import type { ReactNode } from 'react'
import { Skeleton } from './ui/Skeleton.tsx'
import { chartColors } from '@/lib/colors.ts'

type Tone = 'good' | 'bad' | 'warn' | 'mute'
const tones: Record<Tone, string> = {
  good: 'text-text-muted',
  bad: 'text-severity-critical',
  warn: 'text-severity-warning',
  mute: 'text-text-muted',
}
function Sparkline({ points }: { points: number[] }) {
  useTheme()
  if (points.length < 2) return null
  const min = Math.min(...points),
    span = Math.max(...points) - min || 1
  const line = points
    .map(
      (value, index) =>
        `${index ? 'L' : 'M'}${(index * 90) / (points.length - 1)},${23 - ((value - min) / span) * 20}`,
    )
    .join(' ')
  return (
    <svg width="90" height="26" viewBox="0 0 90 26" aria-hidden="true" className="mt-3">
      <path
        d={line}
        fill="none"
        stroke={chartColors.axis}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
interface KPICardProps {
  label: string
  value: ReactNode
  hint?: string
  tone?: Tone
  loading?: boolean
  sparkline?: number[]
}
export function KPICard({ label, value, hint, tone = 'mute', loading = false, sparkline }: KPICardProps) {
  return (
    <div className="metric-cell">
      <p className="text-xs text-text-secondary">{label}</p>
      {loading ? (
        <Skeleton className="mt-3 h-9 w-20" />
      ) : (
        <p className="tnum mt-3 text-[27px] font-medium leading-tight tracking-[-.045em] sm:text-[34px]">
          {value}
        </p>
      )}
      {hint && !loading && (
        <span className={`mt-2 block text-[11px] leading-relaxed ${tones[tone]}`}>{hint}</span>
      )}
      {sparkline && !loading && <Sparkline points={sparkline} />}
    </div>
  )
}
