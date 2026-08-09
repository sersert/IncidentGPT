import { CircleDot, CheckCircle2, BellOff, Flame } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { Severity, IncidentStatus } from '@/api/types.ts'

const severityStyles: Record<Severity, string> = {
  critical: 'bg-severity-critical/12 text-severity-critical',
  warning: 'bg-severity-warning/12 text-severity-warning',
  info: 'bg-severity-info/12 text-severity-info',
  unknown: 'bg-severity-unknown/20 text-severity-unknown',
}

const severityDots: Record<Severity, string> = {
  critical: 'bg-severity-critical',
  warning: 'bg-severity-warning',
  info: 'bg-severity-info',
  unknown: 'bg-severity-unknown',
}

const severityHalo: Record<Severity, string> = {
  critical: 'ring-4 ring-severity-critical/15',
  warning: 'ring-4 ring-severity-warning/15',
  info: 'ring-4 ring-severity-info/15',
  unknown: 'ring-4 ring-severity-unknown/15',
}

const statusStyles: Record<IncidentStatus, string> = {
  active: 'bg-status-active/12 text-status-active',
  resolved: 'bg-status-resolved/12 text-status-resolved',
  muted: 'bg-status-muted/20 text-status-muted',
  acknowledged: 'bg-status-acknowledged/12 text-status-acknowledged',
}

const statusIcons: Record<IncidentStatus, LucideIcon> = {
  active: Flame,
  resolved: CheckCircle2,
  muted: BellOff,
  acknowledged: CircleDot,
}

const statusLabels: Record<IncidentStatus, string> = {
  active: 'Firing',
  resolved: 'Resolved',
  muted: 'Muted',
  acknowledged: 'Acknowledged',
}

export function SeverityBadge({
  severity,
  className = '',
}: {
  severity: Severity
  className?: string
}) {
  return (
    <span
      className={`inline-flex items-center rounded-md px-1.5 py-0.5 text-[11px] font-medium ${severityStyles[severity]} ${className}`}
    >
      {severity}
    </span>
  )
}

/**
 * Точка-сигнал слева от инцидента. Цвет несёт severity, но никогда не в одиночку —
 * рядом всегда стоит SeverityBadge с текстом.
 */
export function SeverityDot({
  severity,
  halo = false,
  className = '',
}: {
  severity: Severity
  /** Ореол вокруг точки — для активных инцидентов, чтобы читались краем глаза. */
  halo?: boolean
  className?: string
}) {
  return (
    <span
      aria-hidden="true"
      className={`inline-block h-2 w-2 shrink-0 rounded-full ${severityDots[severity]} ${
        halo ? severityHalo[severity] : ''
      } ${className}`}
    />
  )
}

export function StatusBadge({ status, className = '' }: { status: IncidentStatus; className?: string }) {
  const Icon = statusIcons[status]
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[11px] font-medium ${statusStyles[status]} ${className}`}
    >
      <Icon size={11} strokeWidth={2.4} />
      {statusLabels[status]}
    </span>
  )
}
