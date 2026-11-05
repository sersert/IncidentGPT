import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ApiError, post, put } from '@/api/client'
import { useOperations } from '@/hooks/useOperations'
import type { AIControl, ComponentStatus } from '@/hooks/useOperations'
import { formatDate } from '@/lib/time'

const labels: Record<string, string> = { healthy: 'Доступен', unhealthy: 'Ошибка', unknown: 'Не проверен', not_configured: 'Не настроен', disabled: 'Выключен' }
const colors: Record<string, string> = { healthy: 'bg-status-resolved', unhealthy: 'bg-severity-critical', unknown: 'bg-text-muted', not_configured: 'bg-text-muted', disabled: 'bg-text-muted' }
function ComponentLamp({ component, failed }: { component: ComponentStatus; failed: boolean }) {
  const age = Date.now() - Date.parse(component.checked_at)
  const stale = !Number.isFinite(age) || age < -60_000 || age > (component.id === 'model' ? 300_000 : 90_000)
  const state = failed || stale ? 'unknown' : component.status
  return <article className="min-w-0 rounded-lg border border-border-default bg-surface-secondary p-4">
    <h2 className="break-words text-sm font-semibold">{component.name}</h2>
    <p className="mt-2 flex items-center gap-2 text-xs"><span aria-hidden="true" className={`h-2.5 w-2.5 shrink-0 rounded-full ${colors[state] ?? colors.unknown}`} />{labels[state] ?? labels.unknown}</p>
    <p className="mt-2 break-words text-xs text-text-secondary">{failed ? 'Не удалось обновить состояние; прошлый результат не подтверждает доступность.' : stale ? 'Результат устарел; доступность неизвестна.' : component.detail}</p>
    <p className="mt-2 break-words text-[11px] text-text-muted">{component.checked_at && !component.checked_at.startsWith('0001') ? formatDate(component.checked_at) : 'Проверки ещё не было'}{component.latency_ms > 0 ? ` · ${component.latency_ms} мс` : ''}</p>
  </article>
}
const actionStyle = 'rounded-md border border-border-default px-3 py-2 text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed'
export function Operations() {
  const query = useOperations()
  const qc = useQueryClient()
  const [probeResult, setProbeResult] = useState<string | null>(null)
  const control = query.data?.control
  const change = useMutation({ mutationFn: () => put<AIControl>('/operations/ai-control', { enabled: !control!.enabled, revision: control!.revision }), onSuccess: () => { qc.invalidateQueries({ queryKey: ['operations'] }); qc.invalidateQueries({ queryKey: ['ai-control'] }) }, onError: () => qc.invalidateQueries({ queryKey: ['operations'] }) })
  const probe = useMutation({ mutationFn: () => post<{ success: boolean; latency_ms: number }>('/operations/model-probe', {}), onSuccess: result => { setProbeResult(result.success ? `Модель ответила за ${result.latency_ms} мс` : 'Проверка модели не удалась'); qc.invalidateQueries({ queryKey: ['operations'] }) } })
  if (!query.data) return <main className="mx-auto max-w-[1320px] space-y-4 p-4 sm:p-8"><Link to="/" className="text-xs text-accent underline">← IncidentGPT</Link><h1 className="text-2xl font-semibold">Состояние системы</h1>{query.isPending ? <p>Проверяем компоненты…</p> : <p role="alert">{query.error instanceof ApiError && query.error.status === 404 ? 'Мониторинг ещё не включён администратором.' : 'Состояние недоступно. Проверьте доступ и повторите запрос.'}</p>}<button className={actionStyle} onClick={() => query.refetch()}>Обновить</button></main>
  const managed = query.data.worker?.managed === true
  const canChange = !query.isError && query.data.control_available && control && (control.enabled || managed)
  const applied = managed && control && query.data.worker?.control_known && query.data.worker?.control?.revision === control.revision && query.data.worker?.control?.enabled === control.enabled
  return <main className="mx-auto max-w-[1320px] min-w-0 space-y-5 p-4 sm:p-8">
    <Link to="/" className="text-xs text-accent underline">← IncidentGPT</Link>
    <div className="flex flex-wrap items-start justify-between gap-3"><div><h1 className="break-words text-2xl font-semibold">Состояние системы</h1><p className="mt-2 text-xs text-text-muted">Автообновление без генерации текста моделью. Цвет дополняется подписью.</p></div><button className={actionStyle} disabled={query.isFetching} onClick={() => query.refetch()}>Обновить</button></div>
    {query.isError && <p role="alert" className="text-sm text-severity-critical">Обновление не удалось. Доступность компонентов неизвестна.</p>}
    <section aria-label="Управление AI" className="rounded-lg border border-border-default bg-surface-secondary p-4">
      <h2 className="text-base font-semibold">AI-анализ</h2>
      <p className="mt-2 text-sm">{query.isError || !query.data.control_available ? 'Управление недоступно: нет подтверждённого хранилища.' : control?.enabled ? 'Новые обращения к модели разрешены.' : 'Запрошена пауза обращений к модели.'}</p>
      {!query.isError && control && <p className="mt-2 text-xs text-text-muted">{applied ? 'Опрошенный worker подтвердил состояние.' : 'Ожидаем подтверждение worker; применение состояния пока не подтверждено.'} Активных запросов на опрошенном worker: {query.data.worker?.active_provider_requests ?? 'неизвестно'}.</p>}
      <p className="mt-2 text-xs text-text-secondary">Алерты, инциденты и обычные уведомления продолжают поступать. Уже принятый провайдером запрос может потратить токены. Очередь durable-анализов продолжит работу после включения.</p>
      <div className="mt-4 flex flex-wrap gap-2"><button className={actionStyle} disabled={!canChange || change.isPending} onClick={() => change.mutate()}>{change.isPending ? 'Сохраняем…' : control?.enabled ? 'Выключить AI' : 'Включить AI'}</button><button className={actionStyle} disabled={!canChange || !managed || !control?.enabled || probe.isPending || change.isPending} onClick={() => { setProbeResult(null); probe.mutate() }}>{probe.isPending ? 'Проверяем…' : 'Проверить модель · до 8 токенов'}</button></div>
      <p className="mt-2 text-[11px] text-text-muted">Ручная проверка вызывает провайдера и может быть платной; не чаще одного раза в минуту. Лимит 8 токенов относится к ответу, входной текст оплачивается отдельно.</p>
      {change.isError && <p role="alert" className="mt-3 text-xs text-severity-critical">Изменение не сохранено. Обновите состояние и повторите.</p>}
      {probe.isError && <p role="alert" className="mt-3 text-xs text-severity-critical">Проверка не удалась или временно ограничена. Автоматического платного повтора нет.</p>}
      {probeResult && <p role="status" className="mt-3 text-xs">{probeResult}</p>}
    </section>
    <section aria-label="Доступность компонентов" className="grid min-w-0 gap-3 lg:grid-cols-2 xl:grid-cols-3">{query.data.components.map(component => <ComponentLamp key={component.id} component={component} failed={query.isError} />)}</section>
    <Link className="inline-block text-xs text-accent underline" to="/incidents">К инцидентам</Link>
  </main>
}
