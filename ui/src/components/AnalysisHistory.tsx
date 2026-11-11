import { Link } from 'react-router-dom'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { get } from '@/api/client'
import { Card } from '@/components/ui/Card'
import { formatDate } from '@/lib/time'

type KnowledgeReference = { reference_id: string; source_id: string; provider: string; title: string; url: string; version: string; excerpt: string; retrieved_at: string }
type Analysis = { analysis_id: string; summary: string; completed_at: string; human_confirmation: string; missing_information: string[]; suggested_next_checks: string[]; composition_hash: string; knowledge?: { status: string; calls: number; references: KnowledgeReference[] }; hypotheses: { text: string; supporting_evidence_ids: string[]; contradicting_evidence_ids: string[]; knowledge_reference_ids?: string[] }[] }
type Evidence = { evidence_id: string; source_type: string; availability: string; excerpt: string; collection_time?: string; observation_start?: string; resource: string }
export function AnalysisHistory({ incidentId }: { incidentId: string }) {
 const [selected, setSelected] = useState('')
 const [evidenceId, setEvidenceId] = useState('')
 const history = useQuery({ queryKey: ['analyses', incidentId], queryFn: () => get<{items: Analysis[]; latest_analysis_id: string; enabled: boolean; composition_hash: string; history_sources?: string[]; jobs: {job_id: string; status: string; attempts: number; error_code?: string}[]}>(`/incidents/${encodeURIComponent(incidentId)}/analyses`), refetchInterval: 5000, retry: false })
 const evidence = useQuery({ queryKey: ['evidence', incidentId, evidenceId], queryFn: () => get<Evidence>(`/incidents/${encodeURIComponent(incidentId)}/evidence/${encodeURIComponent(evidenceId)}`), enabled: Boolean(evidenceId), retry: false })
 if (history.data?.enabled === false && history.data.items.length === 0) return null
 const a = history.data?.items.find(v => v.analysis_id === (selected || history.data.latest_analysis_id))
 return <Card className="mt-3.5" title="Анализ с доказательствами">
  {history.isPending ? <p>Загрузка истории…</p> : history.isError ? <p role="alert">История анализа недоступна или доступ запрещён.</p> : <>
   {history.data?.history_sources?.map(id => <p key={id} className="text-xs"><a className="text-accent underline" href={`/incidents/${encodeURIComponent(id)}`}>История связанного инцидента: {id}</a></p>)}
   {history.data?.jobs.map(j => <p key={j.job_id} className="mb-2 text-xs">Задание: {j.status} · попыток {j.attempts} {j.error_code && `· ${j.error_code}`}</p>)}
   <select aria-label="Версия анализа" className="mb-3 max-w-full bg-surface-tertiary p-1 text-xs" value={selected || history.data?.latest_analysis_id || ''} onChange={e => setSelected(e.target.value)}><option value="">Выберите версию</option>{history.data?.items.map(v => <option key={v.analysis_id} value={v.analysis_id}>{formatDate(v.completed_at)} · {v.analysis_id}</option>)}</select>
   {!a ? <p className="text-xs">Подтверждённой версии результата пока нет. Состояние задания показано выше.</p> : <>
    <p className="mb-2 text-xs text-text-muted">Подтверждение человеком: {a.human_confirmation}. Это гипотезы, а не установленная причина.</p>
    <Link className="mb-3 inline-block text-xs text-accent underline" to={`/evaluation?incident_id=${encodeURIComponent(incidentId)}&analysis_id=${encodeURIComponent(a.analysis_id)}`}>Создать тестовый кейс из этой версии</Link>
    <p className="text-sm">{a.summary}</p>{a.composition_hash !== history.data?.composition_hash && <p role="status" className="mt-2 text-xs">Анализ относится к предыдущему составу инцидента. Обновите контекст.</p>}
    {a.hypotheses.map((h, i) => <div key={i} className="mt-3 border-t border-border-soft pt-3 text-xs"><p>{h.text}</p>{[['Поддерживает', h.supporting_evidence_ids], ['Противоречит', h.contradicting_evidence_ids]].map(([label, ids]) => <p key={label as string} className="mt-1">{label}: {(ids as string[]).map(id => <button key={id} className="mr-2 text-accent underline" onClick={() => setEvidenceId(id)}>{id.slice(0, 22)}…</button>)}</p>)}</div>)}
    {a.knowledge && <section aria-label="Источники базы знаний" className="mt-3 border-t border-border-soft pt-3 text-xs">
      <p>База знаний: {({ not_requested: 'поиск не понадобился', available: 'доступна', partial: 'частично доступна', unavailable: 'недоступна' } as Record<string, string>)[a.knowledge.status] ?? 'состояние неизвестно'}. Обращений: {a.knowledge.calls}.</p>
      <p className="mt-1 text-text-muted">Документация — справочный материал, а не доказательство причины инцидента.</p>
      {(a.knowledge.references ?? []).map(ref => <details key={ref.reference_id} className="mt-2 rounded bg-surface-tertiary p-3">
        <summary>{ref.title} · {ref.source_id}</summary>
        <a className="mt-2 inline-block text-accent underline" href={ref.url} target="_blank" rel="noopener noreferrer">Открыть источник</a>
        <p className="mt-1 break-all">{ref.reference_id} · версия {ref.version} · получено {formatDate(ref.retrieved_at)}</p>
        <p className="mt-1">Ссылки из гипотез: {a.hypotheses.filter(h => h.knowledge_reference_ids?.includes(ref.reference_id)).map(h => h.text).join('; ') || 'документ получен для справки'}</p>
        <pre className="mt-2 whitespace-pre-wrap break-words">{ref.excerpt}</pre>
      </details>)}
    </section>}
    <p className="mt-3 text-xs">Не хватает данных: {a.missing_information?.join('; ') || 'модель не указала'}</p><p className="mt-2 text-xs">Следующие проверки: {a.suggested_next_checks?.join('; ') || 'не указаны'}</p>
   </>}
  </>}
  {evidenceId && <div className="mt-3 rounded bg-surface-tertiary p-3 text-xs"><button className="mb-2 text-accent" onClick={() => setEvidenceId('')}>Закрыть evidence</button>{evidence.isPending ? <p>Загрузка evidence…</p> : evidence.isError ? <p role="alert">Evidence недоступно или не принадлежит этому инциденту.</p> : evidence.data && <><p>{evidence.data.source_type} · {evidence.data.availability} · {evidence.data.resource}</p><p>Собрано: {evidence.data.collection_time ? formatDate(evidence.data.collection_time) : 'неизвестно'}</p><pre className="mt-2 whitespace-pre-wrap break-all">{evidence.data.excerpt}</pre></>}</div>}
 </Card>
}
