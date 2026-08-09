import { useState } from 'react'
import type { ReactElement, ReactNode } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Cpu, Plug, Eye, Loader2, Check, AlertCircle, Info } from 'lucide-react'
import { api } from '@/api/index.ts'
import type { Integration, IntegrationTestResult } from '@/api/types.ts'
import { Card } from '@/components/ui/Card.tsx'
import { Button } from '@/components/ui/Button.tsx'
import { Input } from '@/components/ui/Input.tsx'
import { Select } from '@/components/ui/Select.tsx'
import { Modal } from '@/components/ui/Modal.tsx'
import { Skeleton } from '@/components/ui/Skeleton.tsx'
import { useIntegrations } from '@/hooks/useAnalytics.ts'

const sections = [
  { id: 'llm', label: 'Модель', icon: Cpu },
  { id: 'integrations', label: 'Интеграции', icon: Plug },
  { id: 'output', label: 'Формат ответа', icon: Eye },
]

function StatusLine({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-1.5">
      <dt className="text-xs text-text-secondary">{label}</dt>
      <dd className="tnum text-right text-[13px]">{value}</dd>
    </div>
  )
}

function LLMSection() {
  const { data, isLoading } = useQuery({ queryKey: ['settings', 'llm'], queryFn: () => api.getLLMStatus() })
  const [result, setResult] = useState<{ success: boolean; model?: string; latency_ms: number; error?: string } | null>(
    null,
  )
  const test = useMutation({ mutationFn: () => api.testLLMConnection(), onSuccess: setResult })

  if (isLoading || !data) return <Skeleton className="h-56 rounded-[10px]" />

  return (
    <div className="space-y-3.5">
      {/* Раньше здесь была форма с кнопкой «Сохранить», которая ничего не сохраняла:
          моделью управляет ai-worker через своё окружение. */}
      <div className="flex gap-2.5 rounded-[10px] border border-border-default bg-surface-secondary px-4 py-3">
        <Info size={15} className="mt-0.5 shrink-0 text-text-muted" />
        <p className="text-[12.5px] text-text-secondary">
          Моделью управляет <span className="font-medium text-text-primary">{data.managed_by}</span> — значения берутся
          из его окружения. Чтобы сменить модель или лимит токенов, поправь values ai-worker и передеплой его;
          из интерфейса это не меняется.
        </p>
      </div>

      <Card title="Текущая конфигурация">
        {!data.reachable ? (
          <div className="flex items-center gap-2 py-2 text-[13px] text-severity-critical">
            <AlertCircle size={15} />
            Нет связи с ai-worker{data.error ? `: ${data.error}` : ''}
          </div>
        ) : (
          <dl className="divide-y divide-border-soft">
            <StatusLine label="Модель" value={<span className="font-mono">{data.model || '—'}</span>} />
            <StatusLine label="Максимум токенов" value={data.max_tokens ?? '—'} />
            <StatusLine label="Таймаут запроса" value={<span className="font-mono">{data.timeout || '—'}</span>} />
            <StatusLine
              label="Endpoint задан"
              value={data.base_url_configured ? 'да' : <span className="text-severity-warning">нет</span>}
            />
            <StatusLine label="Активных запросов к модели" value={data.active_llm_slots ?? 0} />
          </dl>
        )}
      </Card>

      <Card title="Доставка в Telegram">
        <dl className="divide-y divide-border-soft">
          <StatusLine
            label="Канал настроен"
            value={data.telegram_channel_configured ? 'да' : <span className="text-severity-warning">нет</span>}
          />
          <StatusLine
            label="Формат разметки"
            value={<span className="font-mono">{data.telegram_parse_mode || '—'}</span>}
          />
        </dl>
      </Card>

      <div className="flex flex-wrap items-center gap-3">
        <Button variant="secondary" disabled={test.isPending} onClick={() => test.mutate()}>
          {test.isPending && <Loader2 size={13} className="animate-spin" />}
          Проверить связь
        </Button>
        {result && (
          <span className={`text-xs ${result.success ? 'text-status-resolved' : 'text-severity-critical'}`}>
            {result.success
              ? `Ответ получен за ${result.latency_ms} мс${result.model ? ` · ${result.model}` : ''}`
              : `Ошибка: ${result.error}`}
          </span>
        )}
      </div>
    </div>
  )
}

const statusText: Record<string, string> = {
  connected: 'подключено',
  disconnected: 'нет связи',
  not_configured: 'не настроено',
}
const statusDot: Record<string, string> = {
  connected: 'bg-status-resolved',
  disconnected: 'bg-severity-critical',
  not_configured: 'bg-text-muted',
}

function IntegrationCard({ integration }: { integration: Integration }) {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState<Record<string, string>>(integration.config)
  const [test, setTest] = useState<IntegrationTestResult | null>(null)

  const save = useMutation({
    mutationFn: () => api.saveIntegration(integration.type, form),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings', 'integrations'] })
      setOpen(false)
    },
  })

  const runTest = useMutation({
    mutationFn: () => api.testIntegration(integration.type),
    onSuccess: (r) => {
      setTest(r)
      qc.invalidateQueries({ queryKey: ['settings', 'integrations'] })
    },
  })

  return (
    <Card>
      <div className="flex items-center justify-between gap-3">
        <span className="flex items-baseline gap-2">
          <span className="text-[13.5px] font-semibold">{integration.name}</span>
          {/* Тип важнее названия продукта: по нему видно, каким API мы ходим. */}
          {integration.config?.kind && (
            <span className="font-mono text-[11px] text-text-muted">{integration.config.kind}</span>
          )}
        </span>
        <span className="flex items-center gap-1.5 text-[11.5px] text-text-secondary">
          <span className={`h-1.5 w-1.5 rounded-full ${statusDot[integration.status]}`} />
          {statusText[integration.status]}
        </span>
      </div>

      {/* Показываем что-то одно: результат последней проверки перекрывает
          сохранённую ошибку, иначе один и тот же текст печатался дважды. */}
      {!test && integration.last_error && (
        <p className="mt-1.5 text-[11.5px] text-severity-critical">{integration.last_error}</p>
      )}

      {integration.info && Object.keys(integration.info).length > 0 && (
        <dl className="mt-2 space-y-1">
          {Object.entries(integration.info).map(([k, v]) => (
            <div key={k} className="flex items-baseline justify-between gap-3 text-[11.5px]">
              <dt className="text-text-muted">{k}</dt>
              <dd className="truncate font-mono text-text-secondary">{v}</dd>
            </div>
          ))}
        </dl>
      )}

      <div className="mt-2.5 flex flex-wrap items-center gap-2">
        {/* Пустая форма выглядела поломкой: у Kubernetes и Telegram настройки
            живут в другом месте, править тут нечего. */}
        {(integration.fields?.length ?? 0) > 0 ? (
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              setForm(integration.config)
              setOpen(true)
            }}
          >
            Настроить
          </Button>
        ) : (
          <span className="text-[11.5px] text-text-muted">настраивается вне интерфейса</span>
        )}
        <Button size="sm" variant="ghost" disabled={runTest.isPending} onClick={() => runTest.mutate()}>
          {runTest.isPending && <Loader2 size={11} className="animate-spin" />}
          Проверить
        </Button>
        {test && (
          <span
            className={`min-w-0 flex-1 truncate text-[11.5px] ${
              test.success ? 'text-status-resolved' : 'text-severity-critical'
            }`}
            title={test.message}
          >
            {test.success && <Check size={11} className="mb-0.5 mr-1 inline" />}
            {test.message}
          </span>
        )}
      </div>

      <Modal
        open={open}
        title={integration.name}
        onClose={() => setOpen(false)}
        footer={
          <>
            <Button size="sm" disabled={save.isPending} onClick={() => save.mutate()}>
              {save.isPending && <Loader2 size={11} className="animate-spin" />}
              Сохранить
            </Button>
            <Button size="sm" variant="secondary" onClick={() => setOpen(false)}>
              Отмена
            </Button>
            {save.isError && <span className="text-[11.5px] text-severity-critical">Не удалось сохранить</span>}
          </>
        }
      >
        <div className="space-y-3">
          {integration.fields?.map((f) =>
            f.kind === 'select' ? (
              <Select
                key={f.key}
                label={f.required ? `${f.label} *` : f.label}
                value={form[f.key] ?? ''}
                onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
                options={(f.options ?? []).map((o) => ({ value: o.value, label: o.label }))}
              />
            ) : (
              <Input
                key={f.key}
                label={f.required ? `${f.label} *` : f.label}
                type={f.kind === 'password' ? 'password' : f.kind === 'number' ? 'number' : 'text'}
                value={form[f.key] ?? ''}
                placeholder={f.hint}
                onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}
              />
            ),
          )}
          {integration.fields?.some((f) => f.kind === 'password') && (
            <p className="text-[11.5px] text-text-muted">
              Секреты показаны маской. Оставь поле как есть, чтобы сохранить прежнее значение.
            </p>
          )}
        </div>
      </Modal>
    </Card>
  )
}

function IntegrationsSection() {
  const { data: integrations = [], isLoading } = useIntegrations()

  if (isLoading) return <Skeleton className="h-48 rounded-[10px]" />

  const groups: [string, Integration[]][] = [
    ['Источники данных', integrations.filter((i) => i.category === 'data_source')],
    ['Каналы доставки', integrations.filter((i) => i.category === 'output_channel')],
  ]

  return (
    <div className="space-y-5">
      {groups.map(([title, items]) => (
        <div key={title}>
          <h3 className="mb-2.5 text-[13px] font-semibold text-text-secondary">{title}</h3>
          <div className="grid gap-2.5 sm:grid-cols-2">
            {items.map((intg) => (
              <IntegrationCard key={intg.type} integration={intg} />
            ))}
            {items.length === 0 && <p className="text-xs text-text-muted">Ничего не подключено</p>}
          </div>
        </div>
      ))}
    </div>
  )
}

function OutputSection() {
  const [mode, setMode] = useState('standard')
  const modes = [
    { id: 'brief', label: 'Кратко', hint: '2–3 предложения: первопричина и одна рекомендация' },
    { id: 'standard', label: 'Стандартно', hint: 'Полный разбор с рекомендациями и похожими инцидентами' },
    { id: 'expert', label: 'Экспертно', hint: 'Максимум деталей, технический язык, готовые команды' },
  ]
  return (
    <Card title="Формат ответа">
      <div className="mb-3 flex gap-2.5 text-[12.5px] text-text-secondary">
        <Info size={15} className="mt-0.5 shrink-0 text-text-muted" />
        Пока не применяется: формат разбора задаётся промптом в ai-worker.
      </div>
      <div className="space-y-3">
        {modes.map((m) => (
          <label key={m.id} className="flex cursor-pointer items-start gap-2.5">
            <input
              type="radio"
              name="output-mode"
              checked={mode === m.id}
              onChange={() => setMode(m.id)}
              className="mt-1 accent-accent"
            />
            <span>
              <span className="block text-[13.5px] font-medium">{m.label}</span>
              <span className="text-xs text-text-muted">{m.hint}</span>
            </span>
          </label>
        ))}
      </div>
    </Card>
  )
}

const sectionComponents: Record<string, () => ReactElement> = {
  llm: LLMSection,
  integrations: IntegrationsSection,
  output: OutputSection,
}

export function Settings() {
  const { section } = useParams()
  const navigate = useNavigate()
  const active = section && sectionComponents[section] ? section : 'llm'
  const ActiveSection = sectionComponents[active]

  return (
    <div>
      <h1 className="mb-5 text-[22px] font-semibold tracking-[-0.02em]">Настройки</h1>
      <div className="flex flex-col gap-5 lg:flex-row">
        <nav className="flex shrink-0 gap-1 overflow-x-auto lg:w-48 lg:flex-col">
          {sections.map((s) => (
            <button
              key={s.id}
              onClick={() => navigate(`/settings/${s.id}`, { replace: true })}
              className={`flex items-center gap-2.5 whitespace-nowrap rounded-[7px] px-2.5 py-2 text-[13px] transition-colors ${
                active === s.id
                  ? 'bg-accent/12 text-[#B9B9FF]'
                  : 'text-text-secondary hover:bg-surface-tertiary hover:text-text-primary'
              }`}
            >
              <s.icon size={14} strokeWidth={1.9} />
              {s.label}
            </button>
          ))}
        </nav>
        <div className="min-w-0 flex-1">
          <ActiveSection />
        </div>
      </div>
    </div>
  )
}
