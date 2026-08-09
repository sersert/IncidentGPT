import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import {
  ArrowLeft, Bot, Copy, Check, ThumbsUp, ThumbsDown, ChevronDown, ChevronRight,
  Rocket, Zap, Activity, FileText, Boxes, AlertCircle, Layers,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import type { IncidentDetail as IncidentDetailType, ChainEvent, LogEntry, ChainEventType, Alert } from '@/api/types.ts'
import { SeverityBadge, StatusBadge, SeverityDot } from '@/components/ui/Badge.tsx'
import { Card } from '@/components/ui/Card.tsx'
import { Button } from '@/components/ui/Button.tsx'
import { Tabs } from '@/components/ui/Tabs.tsx'
import { Modal } from '@/components/ui/Modal.tsx'
import { Input } from '@/components/ui/Input.tsx'
import { Skeleton, SkeletonText } from '@/components/ui/Skeleton.tsx'
import { useIncident, useSubmitFeedback, useUpdateIncident, useIncidentLogs } from '@/hooks/useIncidents.ts'
import { formatDate, formatDuration, formatTime } from '@/lib/time.ts'
import { countOf, words } from '@/lib/plural.ts'

const eventIcons: Record<ChainEventType, LucideIcon> = {
  deploy: Rocket,
  pod_event: Zap,
  prometheus_alert: Activity,
  log_event: FileText,
  k8s_event: Boxes,
}

function CopyButton({ text, label = 'Копировать' }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      onClick={() => {
        navigator.clipboard.writeText(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }}
      className="ml-auto inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-md border border-border-default bg-surface-tertiary px-2 py-1 text-[11px] text-text-secondary transition-colors hover:text-text-primary"
    >
      {copied ? <Check size={11} /> : <Copy size={11} />}
      {copied ? 'Скопировано' : label}
    </button>
  )
}

function ChainTimeline({ events }: { events: ChainEvent[] }) {
  if (events.length === 0) {
    return <p className="py-6 text-center text-[13px] text-text-muted">Цепочка событий пуста</p>
  }
  return (
    <div>
      {events.map((evt, i) => {
        const Icon = eventIcons[evt.type] ?? Activity
        const last = i === events.length - 1
        return (
          <div key={evt.id} className={`grid grid-cols-[16px_1fr] gap-3 ${last ? '' : 'pb-[18px]'}`}>
            <div className="relative">
              <span
                className={`mx-auto mt-1 block h-2.5 w-2.5 rounded-full border-2 ${
                  evt.is_root_cause
                    ? 'border-severity-critical bg-severity-critical/30 ring-4 ring-severity-critical/10'
                    : 'border-text-muted bg-surface-tertiary'
                }`}
              />
              {!last && (
                <span className="absolute left-1/2 top-4 bottom-[-18px] w-0.5 -translate-x-1/2 rounded bg-border-default" />
              )}
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2.5">
                <span className="tnum font-mono text-[11.5px] text-text-muted">{formatTime(evt.timestamp)}</span>
                {evt.is_root_cause && (
                  <span className="rounded-full bg-severity-critical/12 px-[7px] py-0.5 text-[10px] font-semibold tracking-[0.04em] text-severity-critical">
                    ПЕРВОПРИЧИНА
                  </span>
                )}
              </div>
              <p className="mt-[3px] flex items-center gap-2 text-[13.5px] font-medium">
                <Icon size={13} className="shrink-0 text-text-muted" />
                {evt.title}
              </p>
              <p className="mt-0.5 text-xs text-text-muted">
                {evt.source}
                {Object.entries(evt.data || {}).map(([k, v]) => ` · ${k}=${String(v)}`)}
              </p>
            </div>
          </div>
        )
      })}
    </div>
  )
}

/**
 * Алерты, из которых собран инцидент. Отброшенные показываем здесь же, приглушённо
 * и с причиной — иначе непонятно, почему из 8 пришедших в разбор пошло 6.
 */
function AlertsInGroup({ alerts }: { alerts: Alert[] }) {
  if (alerts.length === 0) {
    return <p className="py-6 text-center text-[13px] text-text-muted">Алертов не привязано</p>
  }
  const kept = alerts.filter((a) => !a.discarded)
  const dropped = alerts.filter((a) => a.discarded)

  // Каскад из одного правила даёт десятки одинаковых имён. Если имя у всех одно,
  // не повторяем его в каждой строке — читать нужно то, чем они различаются.
  const names = new Set(alerts.map((a) => a.alertname))
  const singleName = names.size === 1 ? alerts[0].alertname : null

  const row = (a: Alert) => (
    <div key={a.id} className={`flex items-baseline gap-2.5 px-2 py-1.5 ${a.discarded ? 'opacity-55' : ''}`}>
      <SeverityDot severity={a.severity} className="translate-y-[3px]" />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-2">
          {!singleName && <span className="text-[13px] font-medium">{a.alertname}</span>}
          <span className="truncate font-mono text-xs text-text-secondary">{a.description || a.summary}</span>
          {!singleName && <SeverityBadge severity={a.severity} />}
        </div>
        {a.discarded && a.discard_reason && (
          <p className="mt-0.5 text-[11px] text-text-muted">{a.discard_reason}</p>
        )}
      </div>
      <span className="tnum shrink-0 font-mono text-[11px] text-text-muted">{formatTime(a.starts_at)}</span>
    </div>
  )

  return (
    <div>
      {/* Сводка сверху: что за алерты и сколько из них в деле — видно до прокрутки. */}
      <div className="mb-2 flex flex-wrap items-center gap-2 text-xs">
        {singleName && (
          <span className="rounded-md bg-surface-tertiary px-2 py-0.5 font-medium">
            {singleName} × {alerts.length}
          </span>
        )}
        <span className="text-text-secondary">{countOf(kept.length, words.alert)} в разборе</span>
        {dropped.length > 0 && <span className="text-text-muted">· {dropped.length} отброшено</span>}
      </div>

      {/* Каскад бывает и на сотню алертов — держим высоту, чтобы разбор справа
          не уезжал за экран. */}
      <div className="-mx-2 max-h-[380px] overflow-y-auto">
        {kept.map(row)}
        {dropped.length > 0 && (
          <>
            <p className="sticky top-0 mx-2 mb-1 mt-3 border-t border-border-soft bg-surface-secondary pt-3 text-xs font-semibold text-text-secondary">
              Отброшено как шум · {dropped.length}
            </p>
            {dropped.map(row)}
          </>
        )}
      </div>
    </div>
  )
}

function LogsViewer({
  logs,
  loading,
  reason,
}: {
  logs: LogEntry[]
  loading: boolean
  reason?: string
}) {
  const [level, setLevel] = useState('all')
  const [expanded, setExpanded] = useState<Set<number>>(new Set())

  const levelStyles: Record<string, string> = {
    INFO: 'text-text-secondary',
    WARN: 'text-severity-warning bg-severity-warning/5',
    ERROR: 'text-severity-critical bg-severity-critical/5',
    FATAL: 'text-severity-critical bg-severity-critical/10 font-semibold',
  }
  const filtered = level === 'all' ? logs : logs.filter((l) => l.level === level)

  if (loading) {
    return <SkeletonText lines={6} className="py-2" />
  }
  if (logs.length === 0) {
    return (
      <div className="py-6 text-center">
        <p className="text-[13px] text-text-muted">Логов за окно инцидента не нашлось</p>
        {reason && <p className="mt-1 text-[11.5px] text-text-muted">{reason}</p>}
      </div>
    )
  }

  return (
    <div>
      <div className="mb-2.5 flex items-center gap-1">
        <Tabs
          label="Фильтр логов по уровню"
          tabs={[
            { id: 'all', label: 'Все' },
            { id: 'ERROR', label: 'Ошибки' },
            { id: 'WARN', label: 'Предупреждения' },
            { id: 'INFO', label: 'Инфо' },
          ]}
          active={level}
          onChange={setLevel}
        />
        <CopyButton text={filtered.map((l) => `${l.timestamp} ${l.level} ${l.message}`).join('\n')} label="Копировать логи" />
      </div>
      <div className="max-h-[420px] overflow-auto rounded-lg border border-border-default bg-surface-primary p-2 font-mono text-[11.5px]">
        {filtered.map((log, i) => (
          <div key={i} className={`flex gap-2.5 rounded px-1.5 py-0.5 ${levelStyles[log.level] ?? ''}`}>
            <span className="tnum shrink-0 text-text-muted">{formatTime(log.timestamp)}</span>
            <span className="w-11 shrink-0">{log.level}</span>
            {log.is_stacktrace ? (
              <div className="min-w-0 flex-1">
                <button
                  onClick={() =>
                    setExpanded((prev) => {
                      const s = new Set(prev)
                      if (s.has(i)) s.delete(i)
                      else s.add(i)
                      return s
                    })
                  }
                  className="inline-flex items-center gap-1 text-text-muted hover:text-text-primary"
                >
                  {expanded.has(i) ? <ChevronDown size={10} /> : <ChevronRight size={10} />}
                  Stack trace
                </button>
                {expanded.has(i) && <pre className="mt-1 whitespace-pre-wrap text-text-muted">{log.message}</pre>}
              </div>
            ) : (
              <span className="min-w-0 flex-1 break-all">{log.message}</span>
            )}
          </div>
        ))}
        {filtered.length === 0 && <p className="py-4 text-center text-text-muted">Под этот фильтр логов нет</p>}
      </div>
    </div>
  )
}

/**
 * Логи, которыми обогатился алерт — ровно то, что ушло в модель.
 * Отличается от вкладки «Логи»: там весь поток, здесь выжимка со схлопнутыми
 * повторами, на которой строился разбор.
 */
function EnrichmentLogs({ data }: { data: Record<string, unknown> }) {
  const logs = (data?.recent_logs as string[] | undefined) ?? []
  if (logs.length === 0) return null

  return (
    <div className="mb-4">
      <div className="mb-1.5 flex items-center gap-2">
        <span className="text-xs font-medium text-text-secondary">Логи, ушедшие в модель</span>
        <span className="rounded-md bg-surface-tertiary px-[7px] py-0.5 text-[11px] text-text-muted">
          {logs.length} групп
        </span>
        <CopyButton text={logs.join('\n')} />
      </div>
      <div className="max-h-[320px] overflow-auto rounded-lg border border-border-default bg-surface-primary p-2 font-mono text-[11.5px]">
        {logs.map((l, i) => {
          const level = l.startsWith('[FATAL') ? 'text-severity-critical font-semibold'
            : l.startsWith('[ERROR') ? 'text-severity-critical'
              : l.startsWith('[WARN') ? 'text-severity-warning'
                : 'text-text-secondary'
          return (
            <div key={i} className={`px-1.5 py-0.5 ${level}`}>
              {l}
            </div>
          )
        })}
      </div>
      <p className="mt-1.5 text-[11px] text-text-muted">
        Повторы схлопнуты: ×N — сколько раз встретилась одна и та же ошибка.
      </p>
    </div>
  )
}

function RawViewer({ data, title }: { data: Record<string, unknown>; title: string }) {
  const [open, setOpen] = useState(true)
  const json = JSON.stringify(data ?? {}, null, 2)
  return (
    <div className="mb-4">
      <div className="mb-1.5 flex items-center gap-2">
        <button
          onClick={() => setOpen(!open)}
          aria-expanded={open}
          className="flex items-center gap-1.5 text-xs font-medium text-text-secondary hover:text-text-primary"
        >
          {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          {title}
        </button>
        <CopyButton text={json} label="Копировать JSON" />
      </div>
      {open && (
        <pre className="max-h-[320px] overflow-auto rounded-lg border border-border-default bg-surface-primary p-2.5 font-mono text-[11.5px] text-text-secondary">
          {json}
        </pre>
      )}
    </div>
  )
}

function Feedback({ incidentId, existing }: { incidentId: string; existing: IncidentDetailType['feedback'] }) {
  const [helpful, setHelpful] = useState<boolean | null>(null)
  const [comment, setComment] = useState('')
  const mutation = useSubmitFeedback(incidentId)

  if (existing || mutation.isSuccess) {
    return (
      <p className="mt-5 border-t border-border-soft pt-4 text-xs text-status-resolved">
        Спасибо, оценка учтена.
      </p>
    )
  }

  return (
    <div className="mt-5 border-t border-border-soft pt-4">
      <p className="text-xs text-text-secondary">Анализ оказался полезным?</p>
      <div className="mt-2 flex gap-2">
        <Button variant={helpful === true ? 'primary' : 'secondary'} size="sm" onClick={() => setHelpful(true)}>
          <ThumbsUp size={12} /> Да
        </Button>
        <Button variant={helpful === false ? 'danger' : 'secondary'} size="sm" onClick={() => setHelpful(false)}>
          <ThumbsDown size={12} /> Нет
        </Button>
      </div>
      {helpful !== null && (
        <div className="mt-2.5 space-y-2">
          <textarea
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            rows={2}
            placeholder="Что бы вы изменили? (необязательно)"
            aria-label="Комментарий к оценке"
            className="w-full resize-none rounded-lg border border-border-default bg-surface-primary p-2 text-xs outline-none placeholder:text-text-muted focus:border-accent/50"
          />
          <Button
            size="sm"
            disabled={mutation.isPending}
            onClick={() => mutation.mutate({ helpful, comment: comment || null })}
          >
            {mutation.isPending ? 'Отправляем…' : 'Отправить'}
          </Button>
        </div>
      )}
    </div>
  )
}

const recTone: Record<string, string> = {
  immediate: 'bg-severity-critical/12 text-severity-critical',
  fix: 'bg-severity-warning/12 text-severity-warning',
  prevent: 'bg-severity-info/12 text-severity-info',
}
const recLabel: Record<string, string> = {
  immediate: 'сейчас',
  fix: 'починить',
  prevent: 'предотвратить',
}

export function IncidentDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [tab, setTab] = useState('alerts')
  const { data: incident, isLoading, isError } = useIncident(id)
  const [elapsed, setElapsed] = useState(0)
  const [assignOpen, setAssignOpen] = useState(false)
  const [assignee, setAssignee] = useState('')
  const update = useUpdateIncident(id ?? '')
  // Логи тянем только когда вкладка открыта.
  const logsQuery = useIncidentLogs(id, tab === 'logs')
  const logs = logsQuery.data?.data ?? []

  // Для активного инцидента таймер идёт вживую — видно, сколько уже горит.
  useEffect(() => {
    if (!incident || incident.status !== 'active') return
    const tick = () => setElapsed(Math.floor((Date.now() - new Date(incident.created_at).getTime()) / 1000))
    tick()
    const t = setInterval(tick, 1000)
    return () => clearInterval(t)
  }, [incident])

  if (isLoading) {
    return (
      <div>
        <Skeleton className="h-5 w-40" />
        <Skeleton className="mt-3 h-7 w-96" />
        <div className="mt-5 grid gap-3.5 lg:grid-cols-2">
          <Skeleton className="h-80 rounded-[10px]" />
          <Skeleton className="h-80 rounded-[10px]" />
        </div>
      </div>
    )
  }

  if (isError || !incident) {
    return (
      <div className="flex flex-col items-center gap-3 py-20">
        <AlertCircle size={24} className="text-severity-critical" />
        <p className="text-sm text-text-secondary">Инцидент не найден</p>
        <Button variant="secondary" size="sm" onClick={() => navigate('/incidents')}>
          К списку инцидентов
        </Button>
      </div>
    )
  }

  const { analysis, context } = incident
  const duration = incident.duration_sec ?? elapsed

  return (
    <div>
      <nav className="mb-3 flex items-center gap-1.5 text-xs text-text-muted">
        <Link to="/incidents" className="inline-flex items-center gap-1.5 hover:text-text-primary">
          <ArrowLeft size={12} /> Инциденты
        </Link>
        <span>/</span>
        <span className="font-mono">{incident.id}</span>
      </nav>

      <div className="mb-[18px] flex flex-wrap items-center gap-2.5">
        <SeverityBadge severity={incident.severity} />
        <StatusBadge status={incident.status} />
        <span className="tnum rounded-md bg-surface-tertiary px-2 py-0.5 text-[11px] text-text-secondary">
          {formatDuration(duration)}
        </span>
        {incident.alert_count > 1 && (
          <span className="inline-flex items-center gap-1 rounded-md bg-accent/12 px-2 py-0.5 text-[11px] font-medium text-[#B9B9FF]">
            <Layers size={11} />
            {countOf(incident.alert_count, ['связанный алерт', 'связанных алерта', 'связанных алертов'])}
          </span>
        )}
        <div className="ml-auto flex flex-wrap gap-2">
          <Button
            variant="secondary"
            size="sm"
            disabled={update.isPending}
            onClick={() => {
              setAssignee(incident.assignee ?? '')
              setAssignOpen(true)
            }}
          >
            {incident.assignee ? `Назначен: ${incident.assignee}` : 'Назначить'}
          </Button>

          {incident.status !== 'muted' && incident.status !== 'resolved' && (
            <Button
              variant="secondary"
              size="sm"
              disabled={update.isPending}
              onClick={() => update.mutate({ status: 'muted' })}
            >
              Заглушить
            </Button>
          )}

          {incident.status !== 'resolved' && (
            <Button
              variant="secondary"
              size="sm"
              disabled={update.isPending}
              onClick={() => update.mutate({ status: 'resolved' })}
            >
              Закрыть
            </Button>
          )}

          {incident.status === 'active' && (
            <Button size="sm" disabled={update.isPending} onClick={() => update.mutate({ status: 'acknowledged' })}>
              {update.isPending ? 'Сохраняем…' : 'Принять в работу'}
            </Button>
          )}
          {incident.status === 'acknowledged' && (
            <Button
              variant="secondary"
              size="sm"
              disabled={update.isPending}
              onClick={() => update.mutate({ status: 'active' })}
            >
              Вернуть в активные
            </Button>
          )}
        </div>
        <h1 className="basis-full text-[21px] font-semibold tracking-[-0.025em]">{incident.title}</h1>
      </div>

      <div className="grid items-start gap-3.5 lg:grid-cols-2">
        <Card
          title="Состав инцидента"
          action={
            <Tabs
              label="Разделы инцидента"
              tabs={[
                { id: 'alerts', label: 'Алерты', count: incident.alerts?.length ?? incident.alert_count },
                { id: 'chain', label: 'Цепочка', count: incident.chain?.length ?? 0 },
                { id: 'logs', label: 'Логи', count: logs.length || undefined },
                { id: 'raw', label: 'Raw' },
              ]}
              active={tab}
              onChange={setTab}
            />
          }
        >
          {tab === 'alerts' && <AlertsInGroup alerts={incident.alerts ?? []} />}
          {tab === 'chain' && <ChainTimeline events={incident.chain ?? []} />}
          {tab === 'logs' && <LogsViewer logs={logs} loading={logsQuery.isLoading} reason={logsQuery.data?.reason} />}
          {tab === 'raw' && (
            <>
              <EnrichmentLogs data={incident.enriched_data} />
              <RawViewer data={incident.raw_alert} title="Исходный алерт" />
              <RawViewer data={incident.enriched_data} title="Обогащённые данные" />
            </>
          )}
        </Card>

        <Card
          title="Анализ AI"
          action={
            <span className="inline-flex items-center gap-1.5 font-mono text-[11px] text-text-muted">
              <Bot size={12} />
              {analysis ? formatDate(analysis.generated_at) : 'ожидание'}
            </span>
          }
        >
          {!analysis ? (
            <div className="py-2">
              <p className="mb-3 text-[13px] text-text-secondary">Модель разбирает инцидент, обычно это 10–30 секунд.</p>
              <SkeletonText lines={4} />
            </div>
          ) : (
            <>
              <p className="mb-2 text-xs font-semibold text-text-secondary">Первопричина</p>
              <div className="prose-sm text-[13.5px] leading-relaxed [&_a]:text-accent [&_code]:font-mono">
                <ReactMarkdown>{analysis.root_cause}</ReactMarkdown>
              </div>
              {/* Полосу показываем, только если модель дала оценку: выдуманное
                  число читалось бы как её уверенность. */}
              {analysis.confidence > 0 && (
                <div className="mt-3 flex items-center gap-2.5">
                  <span className="h-[5px] flex-1 overflow-hidden rounded-full bg-surface-tertiary">
                    <span
                      className={`block h-full rounded-full ${
                        analysis.confidence > 80
                          ? 'bg-status-resolved'
                          : analysis.confidence > 50
                            ? 'bg-severity-warning'
                            : 'bg-severity-critical'
                      }`}
                      style={{ width: `${analysis.confidence}%` }}
                    />
                  </span>
                  <span className="tnum text-xs font-semibold">уверенность {analysis.confidence}%</span>
                </div>
              )}

              {analysis.recommendations?.length > 0 && (
                <>
                  <p className="mb-2.5 mt-5 text-xs font-semibold text-text-secondary">Что делать</p>
                  {analysis.recommendations.map((rec, i) => (
                    <div key={i} className="mb-2 rounded-[9px] bg-surface-tertiary p-3">
                      <div className="flex items-center gap-2.5">
                        <span
                          className={`rounded-full px-2 py-0.5 text-[10.5px] font-semibold uppercase tracking-[0.03em] ${
                            recTone[rec.category] ?? recTone.prevent
                          }`}
                        >
                          {recLabel[rec.category] ?? rec.category}
                        </span>
                        <span className="text-[13.5px] font-semibold">{rec.title}</span>
                      </div>
                      <p className="mt-1.5 text-[12.5px] text-text-secondary">{rec.description}</p>
                      {rec.command && (
                        <div className="mt-2.5 flex items-center gap-2.5 overflow-x-auto rounded-lg border border-border-default bg-surface-primary px-2.5 py-2">
                          <code className="whitespace-pre font-mono text-xs">{rec.command}</code>
                          <CopyButton text={rec.command} />
                        </div>
                      )}
                    </div>
                  ))}
                </>
              )}

              {analysis.discarded_noise && analysis.discarded_noise.length > 0 && (
                <>
                  <p className="mb-2.5 mt-5 text-xs font-semibold text-text-secondary">
                    Отброшенный шум · {analysis.discarded_noise.length}
                  </p>
                  <div className="rounded-[9px] border border-border-soft p-3">
                    {analysis.discarded_noise.map((d, i) => (
                      <div key={i} className={i > 0 ? 'mt-2.5 border-t border-border-soft pt-2.5' : ''}>
                        <p className="text-[12.5px] font-medium text-text-secondary">{d.alertname}</p>
                        <p className="mt-0.5 text-[11.5px] text-text-muted">{d.reason}</p>
                      </div>
                    ))}
                  </div>
                </>
              )}

              {analysis.similar_incidents?.length > 0 && (
                <>
                  <p className="mb-2.5 mt-5 text-xs font-semibold text-text-secondary">Похожие инциденты</p>
                  {analysis.similar_incidents.map((sim) => (
                    <Link
                      key={sim.id}
                      to={`/incidents/${sim.id}`}
                      className="mb-2 block rounded-[9px] bg-surface-tertiary p-3 transition-colors hover:bg-[#222634]"
                    >
                      <div className="flex items-center justify-between">
                        <span className="text-[12.5px] font-semibold">{sim.id}</span>
                        <span className="tnum text-[11.5px] text-accent">{sim.similarity}% совпадение</span>
                      </div>
                      <p className="mt-1 text-xs text-text-muted">
                        {sim.date} · {sim.title} · {sim.resolution}
                      </p>
                    </Link>
                  ))}
                </>
              )}

              <Feedback incidentId={incident.id} existing={incident.feedback} />
            </>
          )}
        </Card>
      </div>

      <Modal
        open={assignOpen}
        title="Назначить инцидент"
        onClose={() => setAssignOpen(false)}
        footer={
          <>
            <Button
              size="sm"
              onClick={() => {
                update.mutate({ assignee: assignee.trim() || null })
                setAssignOpen(false)
              }}
            >
              Сохранить
            </Button>
            <Button size="sm" variant="secondary" onClick={() => setAssignOpen(false)}>
              Отмена
            </Button>
          </>
        }
      >
        <Input
          label="Кто занимается"
          value={assignee}
          placeholder="имя или ник дежурного"
          onChange={(e) => setAssignee(e.target.value)}
        />
        <p className="mt-2 text-[11.5px] text-text-muted">Пустое поле снимет назначение.</p>
      </Modal>

      {incident.history && incident.history.length > 0 && (
        <Card className="mt-3.5" title="Что делали с инцидентом">
          <div className="space-y-1.5">
            {[...incident.history].reverse().map((h, i) => (
              <div key={i} className="flex flex-wrap items-baseline gap-2 text-[12.5px]">
                <span className="tnum font-mono text-[11.5px] text-text-muted">{formatDate(h.at)}</span>
                <StatusBadge status={h.status} />
                <span className="text-text-secondary">
                  {h.by}
                  {h.assignee ? ` · назначен на ${h.assignee}` : ''}
                </span>
              </div>
            ))}
          </div>
        </Card>
      )}

      {context?.namespace && (
        <div className="mt-3.5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {[
            { k: 'Namespace', v: context.namespace, s: null },
            { k: 'Под', v: context.pod, s: context.pod_status },
            { k: 'Нода', v: context.node, s: [context.node_ram, context.node_cpu].filter(Boolean).join(' · ') },
            {
              k: 'Последний деплой',
              v: context.last_deploy_version,
              s: [context.last_deploy_at && formatDate(context.last_deploy_at), context.last_deploy_source]
                .filter(Boolean)
                .join(' · '),
            },
          ].map((c) => (
            <div key={c.k} className="rounded-[10px] border border-border-default bg-surface-secondary px-4 py-3.5">
              <p className="text-[11.5px] text-text-muted">{c.k}</p>
              <p className="mt-1.5 truncate text-[13.5px] font-medium">{c.v || '—'}</p>
              {c.s && <p className="mt-0.5 text-[11.5px] text-text-muted">{c.s}</p>}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
