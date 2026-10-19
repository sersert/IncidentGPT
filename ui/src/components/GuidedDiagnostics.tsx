import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { get, post } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { formatDate } from '@/lib/time'

type Operation = {operation_id: string; actor: string; action: string; question: string; status: string; collected_at?: string; created_at: string; evidence_ids: string[] | null}
export function GuidedDiagnostics({incidentId}: {incidentId: string}) {
 const [question, setQuestion] = useState('')
 const [changeId, setChangeId] = useState('')
 const qc = useQueryClient()
 const changes = useQuery({queryKey: ['incident-changes', incidentId, 0], queryFn: () => get<{items: {event_id: string; workload: string; revision?: string; source_timestamp?: string}[]}>(`/incidents/${encodeURIComponent(incidentId)}/changes?offset=0&limit=25`), retry: false})
 const state = useQuery({queryKey: ['investigations', incidentId], queryFn: () => get<{operations: Operation[]; jobs: {job_id: string; status: string}[]}>(`/incidents/${encodeURIComponent(incidentId)}/investigations`), refetchInterval: 5000, retry: false})
 const refresh = () => {qc.invalidateQueries({queryKey: ['investigations', incidentId]});qc.invalidateQueries({queryKey: ['analyses', incidentId]})}
 const run = useMutation({mutationFn: (action: string) => post(`/incidents/${encodeURIComponent(incidentId)}/investigations`, {action, question, change_event_id: changeId}), onSuccess: refresh})
 const cancel = useMutation({mutationFn: (job: string) => post(`/incidents/${encodeURIComponent(incidentId)}/investigations/${encodeURIComponent(job)}/cancel`), onSuccess: refresh})
 return <Card className="mt-3.5" title="Проверки по инциденту">
  <p className="mb-3 text-xs text-text-muted">Ограниченные проверки чтения: pod termination/events, rollout, node conditions, метрики и логи. Операции выполняются только для ресурсов инцидента.</p>
  <textarea className="mb-2 w-full rounded bg-surface-tertiary p-2 text-xs" aria-label="Вопрос или причина проверки" maxLength={500} value={question} onChange={e => setQuestion(e.target.value)} placeholder="OOM или отказ ноды?"/>
  <div className="flex flex-wrap gap-2"><Button size="sm" disabled={run.isPending} onClick={() => run.mutate('refresh')}>Обновить контекст</Button><Button size="sm" variant="secondary" disabled={run.isPending || !question.trim()} onClick={() => run.mutate('hypothesis')}>Проверить гипотезу</Button></div>
  <label className="mt-3 block text-xs">Изменение для сравнения<select className="mt-1 w-full bg-surface-tertiary p-2" value={changeId} onChange={e => setChangeId(e.target.value)}><option value="">Выберите изменение с известным временем источника</option>{changes.data?.items.map(e => <option key={e.event_id} value={e.event_id} disabled={!e.source_timestamp}>{e.workload} · {e.revision || 'revision неизвестна'} · {e.source_timestamp ? formatDate(e.source_timestamp) : 'время источника неизвестно'}</option>)}</select></label>{changes.isError && <p className="mt-1 text-xs">Изменения недоступны для сравнения.</p>}<Button className="mt-2" size="sm" variant="secondary" disabled={run.isPending || !changeId.trim()} onClick={() => run.mutate('compare')}>Сравнить до / после</Button>
  {(run.isError || cancel.isError) && <p role="alert" className="mt-2 text-xs">{run.error?.message || cancel.error?.message}</p>}
  {state.isPending ? <p className="mt-3 text-xs">Загрузка операций…</p> : state.isError ? <p role="alert" className="mt-3 text-xs">Операции недоступны или доступ запрещён.</p> : <div className="mt-3 space-y-2 text-xs">{state.data?.operations.length === 0 && <p>Проверок ещё нет.</p>}{state.data?.operations.map(op => {const job = state.data.jobs.find(j => j.job_id === op.operation_id);return <div key={op.operation_id} className="rounded bg-surface-tertiary p-3"><p>{op.action} · {job?.status || op.status} · результаты: {op.status} · {op.actor}</p><p>{op.question}</p><p>Собрано: {op.collected_at ? formatDate(op.collected_at) : 'ещё не собрано'}. Сохранённые данные не означают свежую проверку.</p><p>Evidence: {op.evidence_ids?.length ?? 0}; новая версия анализа доступна в истории.</p>{job && ['queued', 'running'].includes(job.status) && <Button size="sm" variant="secondary" disabled={cancel.isPending} onClick={() => cancel.mutate(op.operation_id)}>Отменить</Button>}</div>})}</div>}
 </Card>
}
