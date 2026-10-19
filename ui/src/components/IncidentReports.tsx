import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import ReactMarkdown from 'react-markdown'
import { get, post, put } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { formatDate } from '@/lib/time'

type Action = {text: string; owner: string; status: string; due_date?: string}
type Report = {report_id: string; revision: number; markdown: string; generated_at: string; review_status: string; action_items?: Action[]; source: {analysis_id?: string; composition_hash: string; resolution_revision: number; lifecycle_revision: number}}
function safeReportURL(url: string) {if ((url.startsWith('/') && !url.startsWith('//') && !url.includes('\\')) || url.startsWith('#')) return url;try {const parsed = new URL(url);return parsed.protocol === 'https:' && !parsed.username && !parsed.password && !parsed.search ? parsed.href : ''} catch {return ''}}
export function IncidentReports({incidentId}: {incidentId: string}) {
 const [kind, setKind] = useState('postmortem');const [selected, setSelected] = useState('');const [editing, setEditing] = useState<Report | null>(null);const [preview, setPreview] = useState(true);const qc = useQueryClient()
 const path = `/incidents/${encodeURIComponent(incidentId)}/${kind}`
 const query = useQuery({queryKey: ['reports', incidentId, kind], queryFn: () => get<{items: Report[]; jobs: {job_id: string; status: string; error_code?: string}[]; enabled: boolean}>(path), refetchInterval: 5000, retry: false})
 const report = query.data?.items.find(r => r.report_id === selected) || query.data?.items[0]
 const current = editing || report
 const generate = useMutation({mutationFn: () => post(path), onSuccess: () => qc.invalidateQueries({queryKey: ['reports', incidentId]})})
 const save = useMutation({mutationFn: () => put<Report>(path, {report_id: editing!.report_id, revision: editing!.revision, markdown: editing!.markdown, review_status: editing!.review_status, action_items: editing!.action_items || []}), onSuccess: r => {setSelected(r.report_id);setEditing(null);qc.invalidateQueries({queryKey: ['reports', incidentId]})}})
 const updateAction = (i: number, key: keyof Action, value: string) => {if (!editing) return;setEditing({...editing, action_items: editing.action_items?.map((a, j) => j === i ? {...a, [key]: value} : a)})}
 return <Card className="mt-3.5" title="Postmortem и передача дежурства">
 <div className="flex flex-wrap gap-2"><select aria-label="Тип отчёта" className="bg-surface-tertiary p-2 text-xs" disabled={Boolean(editing)} value={kind} onChange={e => {setKind(e.target.value);setSelected('')}}><option value="postmortem">Postmortem</option><option value="handoff">Передача дежурства</option></select><Button size="sm" disabled={generate.isPending || query.data?.enabled === false} onClick={() => generate.mutate()}>Создать новый черновик</Button></div>
 <p className="mt-2 text-xs text-text-muted">Черновик строится по сохранённым версиям данных. Неизвестные последствия, причина и восстановление остаются неизвестными. Новый черновик сохраняет правки старого.</p>
 {(generate.isError || save.isError) && <p role="alert" className="mt-2 text-xs">{generate.error?.message || save.error?.message}</p>}
 {query.isPending ? <p>Загрузка отчётов…</p> : query.isError ? <p role="alert">Отчёты недоступны или доступ запрещён.</p> : <>
 {query.data?.jobs.map(j => <p key={j.job_id} className="mt-2 text-xs">Генерация: {j.status} {j.error_code}</p>)}
 {query.data?.items.length === 0 && <p className="mt-3 text-xs">Отчётов ещё нет.</p>}
 {current && <><select aria-label="Версия отчёта" className="mt-3 max-w-full bg-surface-tertiary p-2 text-xs" disabled={Boolean(editing)} value={report?.report_id || ''} onChange={e => setSelected(e.target.value)}>{query.data?.items.map(r => <option key={r.report_id} value={r.report_id}>{formatDate(r.generated_at)} · revision {r.revision} · {r.review_status}</option>)}</select>
 <p className="mt-2 break-all text-xs">Анализ: {current.source.analysis_id || 'неизвестно'}; resolution revision {current.source.resolution_revision}; lifecycle revision {current.source.lifecycle_revision}. Состав: {current.source.composition_hash}</p>
 <div className="mt-3 flex flex-wrap gap-2">{!editing && <Button size="sm" onClick={() => setEditing({...report!, action_items: report!.action_items || []})}>Редактировать</Button>}<Button size="sm" variant="secondary" onClick={() => setPreview(!preview)}>{preview ? 'Исходный Markdown' : 'Предпросмотр'}</Button>{!editing && <a className="text-xs text-accent underline" href={`${import.meta.env.VITE_API_URL || '/api/v1'}${path}/export?report_id=${encodeURIComponent(current.report_id)}`}>Скачать Markdown</a>}</div>
 {editing && <><textarea aria-label="Markdown отчёта" className="mt-3 h-80 w-full bg-surface-tertiary p-3 font-mono text-xs" maxLength={32768} value={editing.markdown} onChange={e => setEditing({...editing, markdown: e.target.value})}/><label className="mt-2 block text-xs">Ревью<select className="ml-2 bg-surface-tertiary p-2" value={editing.review_status} onChange={e => setEditing({...editing, review_status: e.target.value})}><option value="draft">Черновик</option><option value="reviewed">Проверено человеком</option></select></label></>}
 {preview ? <div className="prose mt-3 max-w-none break-words text-xs"><ReactMarkdown skipHtml urlTransform={safeReportURL} components={{img: () => null, a: ({children, href}) => <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>}}>{current.markdown}</ReactMarkdown></div> : !editing && <pre className="mt-3 whitespace-pre-wrap break-words text-xs">{current.markdown}</pre>}
 <p className="mt-4 text-xs">Дальнейшие действия</p>{current.action_items?.map((a, i) => <div key={i} className="mt-2 flex flex-wrap gap-2 text-xs">{editing ? <><input aria-label="Действие" className="grow bg-surface-tertiary p-2" maxLength={1000} value={a.text} onChange={e => updateAction(i, 'text', e.target.value)}/><input aria-label="Ответственный" className="bg-surface-tertiary p-2" maxLength={200} value={a.owner} onChange={e => updateAction(i, 'owner', e.target.value)}/><select aria-label="Статус действия" className="bg-surface-tertiary p-2" value={a.status} onChange={e => updateAction(i, 'status', e.target.value)}><option value="open">Открыто</option><option value="in_progress">В работе</option><option value="done">Выполнено</option></select><input aria-label="Срок действия" type="date" className="bg-surface-tertiary p-2" value={a.due_date || ''} onChange={e => updateAction(i, 'due_date', e.target.value)}/><Button size="sm" variant="secondary" onClick={() => setEditing({...editing, action_items: editing.action_items?.filter((_, j) => j !== i)})}>Удалить</Button></> : <p>{a.text} · {a.owner || 'ответственный неизвестен'} · {a.status} · {a.due_date || 'без срока'}</p>}</div>)}
 {editing && <div className="mt-3 flex gap-2"><Button size="sm" variant="secondary" disabled={(editing.action_items?.length || 0) >= 30} onClick={() => setEditing({...editing, action_items: [...(editing.action_items || []), {text: '', owner: '', status: 'open'}]})}>Добавить действие</Button><Button size="sm" disabled={save.isPending} onClick={() => save.mutate()}>Сохранить revision {editing.revision + 1}</Button><Button size="sm" variant="secondary" onClick={() => setEditing(null)}>Отменить правки</Button></div>}
 </>}
 </>}
 </Card>
}
