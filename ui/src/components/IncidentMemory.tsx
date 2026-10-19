import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { get, put } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'

type Resolution = { revision: number; status: string; confirmed_cause: string; restoring_action: string; verification_evidence_ids: string[]; unsuccessful_attempts: string[]; environment: string; versions: string; failure_mode: string; error_signature: string; links: string[]; confirming_actor?: string; confirmed_at?: string }
const empty: Resolution = { revision: 0, status: 'unknown', confirmed_cause: '', restoring_action: '', verification_evidence_ids: [], unsuccessful_attempts: [], environment: '', versions: '', failure_mode: '', error_signature: '', links: [] }
export function IncidentMemory({ incidentId }: { incidentId: string }) {
 const qc = useQueryClient()
 const [draft, setDraft] = useState<Resolution | null>(null)
 const current = useQuery({queryKey: ['resolution', incidentId], queryFn: () => get<{resolution: Resolution | null}>(`/incidents/${encodeURIComponent(incidentId)}/resolution`), retry: false})
 const matches = useQuery({queryKey: ['memory', incidentId], queryFn: () => get<{results: {id: string; resolution: Resolution; matching_features: string[]; differences: string[]}[]; runbooks: {document_id: string; version_hash: string; title: string; excerpt: string; collected_at: string}[]; runbook_status: string; candidate_limit_applied: boolean}>(`/knowledge/search?incident_id=${encodeURIComponent(incidentId)}`), retry: false})
 const save = useMutation({mutationFn: (value: Resolution) => put(`/incidents/${encodeURIComponent(incidentId)}/resolution`, value), onSuccess: () => {setDraft(null);qc.invalidateQueries({queryKey: ['resolution', incidentId]});qc.invalidateQueries({queryKey: ['memory']})}})
 return <Card className="mt-3.5" title="Подтверждённые решения и runbooks">
  <p className="mb-3 text-xs text-text-muted">Полезный ответ AI и закрытый инцидент не подтверждают причину. Решение подтверждает инженер, с evidence проверки.</p>
  {current.isPending ? <p>Загрузка решения…</p> : current.isError ? <p role="alert">Решение недоступно или нет доступа.</p> : <>
   <p className="mb-2 text-xs">Текущее подтверждение: {current.data?.resolution?.status ?? 'нет записи'}</p>
   <Button size="sm" variant="secondary" onClick={() => setDraft(current.data?.resolution ? {...current.data.resolution} : {...empty})}>Записать / исправить решение</Button>
  </>}
  {draft && <div className="mt-3 space-y-2 text-xs">
   <label className="block">Состояние <select className="ml-2 bg-surface-tertiary" value={draft.status} onChange={e => setDraft({...draft, status: e.target.value})}><option value="unknown">Причина неизвестна</option><option value="confirmed">Подтверждено человеком</option><option value="withdrawn">Отозвано</option></select></label>
   {([['confirmed_cause', 'Подтверждённая причина'], ['restoring_action', 'Действие, восстановившее сервис'], ['environment', 'Среда'], ['versions', 'Версии'], ['failure_mode', 'Тип сбоя / alertname'], ['error_signature', 'Сигнатура ошибки']] as const).map(([key, label]) => <label key={key} className="block">{label}<textarea className="mt-1 block w-full rounded bg-surface-tertiary p-2" value={draft[key]} maxLength={key === 'confirmed_cause' || key === 'restoring_action' ? 4000 : 200} onChange={e => setDraft({...draft, [key]: e.target.value})}/></label>)}
   <label className="block">Evidence IDs проверки (через запятую)<input className="mt-1 block w-full bg-surface-tertiary p-2" value={draft.verification_evidence_ids.join(', ')} onChange={e => setDraft({...draft, verification_evidence_ids: e.target.value.split(',').map(v => v.trim()).filter(Boolean)})}/></label>
   <label className="block">Неуспешные попытки (по одной на строке)<textarea className="mt-1 block w-full bg-surface-tertiary p-2" value={draft.unsuccessful_attempts.join('\n')} onChange={e => setDraft({...draft, unsuccessful_attempts: e.target.value.split('\n').filter(Boolean)})}/></label>
   <label className="block">Безопасные HTTPS ссылки (по одной на строке)<textarea className="mt-1 block w-full bg-surface-tertiary p-2" value={draft.links.join('\n')} onChange={e => setDraft({...draft, links: e.target.value.split('\n').filter(Boolean)})}/></label>
   {save.isError && <p role="alert">Не удалось сохранить: {save.error.message}. При конфликте перезагрузите запись.</p>}
   <Button size="sm" disabled={save.isPending} onClick={() => save.mutate(draft)}>Сохранить версию</Button><Button size="sm" variant="secondary" onClick={() => setDraft(null)}>Отмена</Button>
  </div>}
  <div className="mt-4 text-xs">{matches.isPending ? <p>Поиск решений…</p> : matches.isError ? <p role="alert">Поиск недоступен; отсутствие результатов не подтверждено.</p> : <>
   {matches.data?.results.length === 0 && <p>Подходящих подтверждённых решений пока нет.</p>}
   {matches.data?.results.map(m => <Link key={m.id} to={`/incidents/${m.id}`} className="mt-2 block rounded bg-surface-tertiary p-3"><p>{m.id} · {m.resolution.status}</p><p>{m.resolution.confirmed_cause}</p><p className="mt-1">Решение: {m.resolution.restoring_action}</p><p className="mt-1">Совпадает: {m.matching_features.join('; ') || 'общая область поиска'}</p><p>Различия: {m.differences.join('; ') || 'не выявлены в записанных данных'}</p></Link>)}
   {matches.data?.candidate_limit_applied && <p className="mt-2">Поиск ограничен последними 200 кандидатами.</p>}
   {matches.data?.runbook_status !== 'available' && <p className="mt-2">Runbooks: {matches.data?.runbook_status}. Другие решения показаны независимо.</p>}
   {matches.data?.runbooks.map(b => <details key={b.document_id} className="mt-3 rounded bg-surface-tertiary p-3"><summary>{b.title} · {b.version_hash.slice(0, 12)}</summary><p className="mt-1">Собрано: {b.collected_at}. Текст — источник данных, не команда к выполнению.</p><pre className="mt-2 whitespace-pre-wrap">{b.excerpt}</pre></details>)}
  </>}</div>
 </Card>
}
