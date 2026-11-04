import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { get, put } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { formatDate } from '@/lib/time'
type Lifecycle = {revision: number; first_known_symptom?: string; alert_received_at?: string; acknowledged_at?: string; analysis_started_at?: string; analysis_completed_at?: string; verified_recovery_at?: string; administratively_closed_at?: string; recovery_actor?: string}
export function IncidentLifecycle({incidentId}: {incidentId: string}) {
 const [at, setAt] = useState('');const [evidence, setEvidence] = useState('');const qc = useQueryClient()
 const path = `/incidents/${encodeURIComponent(incidentId)}`
 const life = useQuery({queryKey: ['lifecycle', incidentId], queryFn: () => get<Lifecycle>(`${path}/lifecycle`), refetchInterval: 5000, retry: false})
 const analyses = useQuery({queryKey: ['lifecycle-evidence', incidentId], queryFn: () => get<{items: {input_evidence_ids: string[] | null}[]}>(`${path}/analyses`), retry: false})
 const ids = [...new Set(analyses.data?.items.flatMap(a => a.input_evidence_ids || []) || [])]
 const verify = useMutation({mutationFn: () => put(`${path}/lifecycle`, {revision: life.data!.revision, verified_recovery_at: new Date(at).toISOString(), evidence_ids: [evidence]}), onSuccess: () => qc.invalidateQueries({queryKey: ['lifecycle', incidentId]})})
 return <Card className="mt-3.5" title="События жизненного цикла">
 {life.isPending ? <p>Загрузка событий…</p> : life.isError ? <p role="alert">События недоступны или доступ запрещён.</p> : <dl className="space-y-1 text-xs">{([['Первый известный симптом', 'first_known_symptom'], ['Получение алерта', 'alert_received_at'], ['Принят в работу', 'acknowledged_at'], ['Начало AI-анализа', 'analysis_started_at'], ['Конец AI-анализа', 'analysis_completed_at'], ['Проверенное восстановление', 'verified_recovery_at'], ['Административное закрытие', 'administratively_closed_at']] as const).map(([label, key]) => <div key={key} className="flex justify-between gap-2"><dt>{label}</dt><dd>{life.data?.[key] ? formatDate(life.data[key]!) : 'неизвестно'}</dd></div>)}</dl>}
 <p className="mt-3 text-xs text-text-muted">Resolved от источника — сигнал, а не доказательство восстановления. Подтвердите время по проверенным evidence; действие записывает ваше имя и время ввода.</p>
 <input type="datetime-local" aria-label="Проверенное время восстановления" className="mt-2 bg-surface-tertiary p-2 text-xs" value={at} onChange={e => setAt(e.target.value)}/><select aria-label="Evidence восстановления" className="mt-2 max-w-full bg-surface-tertiary p-2 text-xs" value={evidence} onChange={e => setEvidence(e.target.value)}><option value="">Выберите evidence успешной проверки</option>{ids.map(id => <option key={id} value={id}>{id}</option>)}</select>
 <Button className="mt-2" size="sm" disabled={!life.data || !at || !evidence || verify.isPending} onClick={() => verify.mutate()}>Подтвердить восстановление</Button>
 {verify.isError && <p role="alert" className="mt-2 text-xs">{verify.error.message}</p>}{analyses.isError && <p className="text-xs">Evidence недоступно.</p>}
 </Card>
}
