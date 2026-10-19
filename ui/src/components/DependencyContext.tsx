import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { get, post } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'

type Node = {id: string; name: string; namespace: string; kind: string}
type Edge = {id: string; from: string; to: string; kind: string; source: string; evidence_ref: string; last_seen: string}
export function DependencyContext({incidentId, alerts}: {incidentId: string; alerts: {id: string; alertname: string; correlation_reason?: string}[]}) {
 const [alertId, setAlertId] = useState(''); const [target, setTarget] = useState('');const qc = useQueryClient()
 const graph = useQuery({queryKey: ['dependency-graph', incidentId], queryFn: () => get<{nodes: Node[]; edges: Edge[]; status: string; stale_edges_omitted?: number; bounded?: boolean}>(`/services/graph?incident_id=${encodeURIComponent(incidentId)}`), retry: false})
 const move = useMutation({mutationFn: () => post<{target_incident_id: string}>(`/incidents/${encodeURIComponent(incidentId)}/regroup`, {alert_id: alertId, target_incident_id: target}), onSuccess: () => {qc.invalidateQueries({queryKey: ['incident', incidentId]});qc.invalidateQueries({queryKey: ['analyses', incidentId]})}})
 const name = (id: string) => {const n = graph.data?.nodes.find(n => n.id === id);return n ? `${n.namespace}/${n.name} (${n.kind})` : id}
 return <Card className="mt-3.5" title="Зависимости и состав инцидента">
 <p className="text-xs text-text-muted">Общая зависимость — предположение о связи симптомов. Ownership и placement не доказывают зависимости приложений. Состояние сервисов неизвестно.</p>
 {graph.isPending ? <p>Загрузка связей…</p> : graph.isError ? <p role="alert">Граф недоступен или доступ запрещён.</p> : <div className="mt-2 space-y-2 text-xs">{graph.data?.edges.length === 0 && <p>Свежие связи отсутствуют; топология неизвестна.</p>}{graph.data?.edges.slice(0, 30).map(e => <p key={e.id}>{name(e.from)} → {name(e.to)} · {e.kind} · {e.source}<br/>{e.evidence_ref} · {e.last_seen}</p>)}<p>Устаревшие связи пропущены: {graph.data?.stale_edges_omitted ?? 0}. {graph.data?.bounded || (graph.data?.edges.length ?? 0) > 30 ? 'Показан ограниченный набор связей.' : ''}</p></div>}
 <div className="mt-3 space-y-1 text-xs">{alerts.map(a => <p key={a.id}>{a.alertname}: {a.correlation_reason || 'Причина группировки в старой записи неизвестна.'}</p>)}</div>
 <label className="mt-3 block text-xs">Перенести алерт<select className="mt-1 w-full bg-surface-tertiary p-2" value={alertId} onChange={e => setAlertId(e.target.value)}><option value="">Выберите алерт</option>{alerts.map(a => <option key={a.id} value={a.id}>{a.alertname} · {a.id}</option>)}</select></label>
 <input className="mt-2 w-full bg-surface-tertiary p-2 text-xs" aria-label="Целевой инцидент" value={target} onChange={e => setTarget(e.target.value)} placeholder="ID целевого инцидента; пусто — новый инцидент"/>
 <Button className="mt-2" size="sm" disabled={!alertId || move.isPending} onClick={() => move.mutate()}>Перенести / отделить</Button>
 {move.isError && <p role="alert" className="text-xs">{move.error.message}</p>}{move.data && <p className="text-xs">Перенесён в {move.data.target_incident_id}. История сохранена; обновите контекст для анализа нового состава.</p>}
 </Card>
}
