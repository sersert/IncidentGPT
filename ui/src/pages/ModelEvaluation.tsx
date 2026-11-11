import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { FlaskConical, Plus, Play, ArrowLeft, ArrowRight } from "lucide-react";
import { ApiError, get, post, put } from "@/api/client";
import { Card } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { formatDate } from "@/lib/time";

type Profile = {
  id: string;
  label: string;
  model: string;
  prompt_version: string;
  max_tokens: number;
  revision: string;
};
type Observation = {
  evidence_id: string;
  source_type: string;
  availability: string;
  excerpt: string;
  collection_time?: string;
};
type CaseSummary = {
  case_id: string;
  name: string;
  source: string;
  source_incident_id?: string;
  source_analysis_id?: string;
  created_at: string;
  snapshot_hash: string;
  has_reference?: boolean;
};
type TestCase = CaseSummary & {
  created_by: string;
  source_completed_at?: string;
  reference: {
    cause: string;
    kind: string;
    actor: string;
    resolution_revision?: number;
  };
  input: { evidence: Observation[] };
};
type Review = {
  revision: number;
  verdict: string;
  note: string;
  reviewed_by?: string;
  reviewed_at?: string;
};
type Result = {
  job_id: string;
  status: string;
  error_code?: string;
  profile: Profile;
  review: Review;
  result?: {
    summary: string;
    model: string;
    prompt_version: string;
    started_at: string;
    completed_at: string;
    usage?: Record<string, number>;
    hypotheses?: {
      text: string;
      supporting_evidence_ids?: string[];
      contradicting_evidence_ids?: string[];
    }[];
    missing_information?: string[];
    suggested_next_checks?: string[];
  };
};
type Run = {
  run: {
    run_id: string;
    test_case: TestCase;
    created_at: string;
    created_by: string;
    profiles: Profile[];
  };
  results: Result[];
  paused: boolean;
};
const textareaStyle =
  "min-h-24 w-full rounded-md border border-border-default bg-surface-primary p-3 text-[13px] text-text-primary";
const path = (value: string) => encodeURIComponent(value);
function ErrorMessage({ error }: { error: unknown }) {
  return (
    <p role="alert" className="mt-3 break-words text-sm text-severity-critical">
      {error instanceof ApiError
        ? `Запрос не выполнен: ${error.message}`
        : "Данные недоступны. Проверьте подключение и повторите запрос."}
    </p>
  );
}

function CaseForm({
  incident,
  analysis,
  onCreated,
}: {
  incident: string;
  analysis: string;
  onCreated: (id: string) => void;
}) {
  const [mode, setMode] = useState(incident ? "incident" : "manual");
  const [name, setName] = useState("");
  const [source, setSource] = useState(incident);
  const [sourceAnalysis, setSourceAnalysis] = useState(analysis);
  const [scenario, setScenario] = useState("");
  const [expected, setExpected] = useState("");
  const history = useQuery({
    queryKey: ["evaluation-source", source],
    queryFn: () =>
      get<{
        items: { analysis_id: string; completed_at: string }[];
        latest_analysis_id: string;
      }>(`/incidents/${path(source)}/analyses`),
    enabled: mode === "incident" && source.trim().length > 0,
    retry: false,
  });
  const selected = sourceAnalysis || history.data?.latest_analysis_id || "";
  const create = useMutation({
    mutationFn: () =>
      post<TestCase>("/evaluation/cases", {
        name,
        expected_cause: expected,
        ...(mode === "manual"
          ? { scenario }
          : { incident_id: source, analysis_id: selected }),
      }),
    onSuccess: (value) => {
      onCreated(value.case_id);
      setName("");
      setScenario("");
      setExpected("");
    },
  });
  return (
    <details
      className="rounded-lg border border-border-default bg-surface-secondary"
      open={incident ? true : undefined}
    >
      <summary className="cursor-pointer px-5 py-4 text-sm font-semibold">
        <Plus className="mr-2 inline" size={16} />
        Создать тестовый кейс
      </summary>
      <form
        className="space-y-4 border-t border-border-soft p-5"
        onSubmit={(event) => {
          event.preventDefault();
          create.mutate();
        }}
      >
        <div className="flex flex-wrap gap-2" aria-label="Источник кейса">
          <Button
            size="sm"
            variant={mode === "manual" ? "primary" : "secondary"}
            aria-pressed={mode === "manual"}
            onClick={() => setMode("manual")}
          >
            Свой сценарий
          </Button>
          <Button
            size="sm"
            variant={mode === "incident" ? "primary" : "secondary"}
            aria-pressed={mode === "incident"}
            onClick={() => setMode("incident")}
          >
            Из инцидента
          </Button>
        </div>
        <Input
          label="Название кейса"
          required
          maxLength={160}
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Например: checkout после обновления"
        />
        {mode === "manual" ? (
          <>
            <label className="block text-xs font-medium text-text-secondary">
              Данные для модели
              <textarea
                className={`${textareaStyle} mt-2 min-h-36`}
                required
                maxLength={8000}
                value={scenario}
                onChange={(event) => setScenario(event.target.value)}
                placeholder="Симптомы, события, фрагменты метрик и логов. Ожидаемую причину укажите отдельно."
              />
            </label>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setName("Учебный пример: OOMKilled");
                setScenario(
                  "Учебный сценарий. Pod checkout-api перезапустился 4 раза за 10 минут. Last termination reason: OOMKilled. Memory limit: 512 MiB. Working set перед рестартом: 510 MiB. Данных о состоянии node нет.",
                );
                setExpected(
                  "Превышение лимита памяти контейнера; причины роста потребления требуют отдельной проверки.",
                );
              }}
            >
              Заполнить учебный пример
            </Button>
          </>
        ) : (
          <>
            <Input
              label="ID исходного инцидента"
              required
              maxLength={100}
              value={source}
              onChange={(event) => {
                setSource(event.target.value);
                setSourceAnalysis("");
              }}
              placeholder="INC-2026-0042"
            />
            {history.isPending && source && (
              <p className="text-xs text-text-muted">
                Загружаем версии анализа…
              </p>
            )}
            {history.isError && <ErrorMessage error={history.error} />}
            <Select
              label="Снимок анализа"
              value={selected}
              onChange={(event) => setSourceAnalysis(event.target.value)}
              required
              options={[
                { value: "", label: "Выберите сохранённую версию" },
                ...(history.data?.items ?? []).map((item) => ({
                  value: item.analysis_id,
                  label: `${formatDate(item.completed_at)} · ${item.analysis_id}`,
                })),
              ]}
            />
            <p className="text-xs text-text-muted">
              Сохраняются наблюдения выбранной версии. Предыдущие выводы,
              подтверждённые решения и поиск новых данных исключены. Версии без
              сохранённых доказательств использовать нельзя.
            </p>
          </>
        )}
        <label className="block text-xs font-medium text-text-secondary">
          Ожидаемая причина · необязательно
          <textarea
            className={`${textareaStyle} mt-2`}
            maxLength={4000}
            value={expected}
            onChange={(event) => setExpected(event.target.value)}
            placeholder={
              mode === "incident"
                ? "Если оставить пустым, используем подтверждённую причину для того же состава инцидента, когда она есть."
                : "Эталон для ручной оценки. Модель его не получает."
            }
          />
        </label>
        <p className="text-xs text-text-muted">
          Создание кейса не вызывает модель. Снимок и эталон сохраняются на 90
          дней; изменение данных оформляется новым кейсом.
        </p>
        <Button
          type="submit"
          disabled={
            create.isPending ||
            !name.trim() ||
            (mode === "manual"
              ? !scenario.trim()
              : !selected || history.isError)
          }
        >
          {create.isPending ? "Сохраняем…" : "Сохранить кейс"}
        </Button>
        {create.isError && <ErrorMessage error={create.error} />}
      </form>
    </details>
  );
}

function ResultCard({
  item,
  runID,
  snapshot,
}: {
  item: Result;
  runID: string;
  snapshot: TestCase;
}) {
  const qc = useQueryClient();
  const [verdict, setVerdict] = useState(item.review.verdict || "unrated");
  const [note, setNote] = useState(item.review.note || "");
  const review = useMutation({
    mutationFn: () =>
      put(
        `/evaluation/runs/${path(runID)}/results/${path(item.profile.id)}/review`,
        { revision: item.review.revision, verdict, note },
      ),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ["evaluation-run", runID] }),
  });
  const result = item.result;
  const seconds = result
    ? (Date.parse(result.completed_at) - Date.parse(result.started_at)) / 1000
    : null;
  const states: Record<string, string> = {
    queued: "В очереди",
    running: "Модель выполняет разбор",
    complete: "Разбор завершён",
    failed: "Не выполнен",
  };
  const usage = result?.usage;
  return (
    <Card title={item.profile.label}>
      <p className="break-all font-mono text-xs text-text-secondary">
        {item.profile.model}
      </p>
      <p className="mt-2 break-all text-[11px] text-text-muted">
        {item.profile.prompt_version} · до {item.profile.max_tokens} токенов
        ответа
      </p>
      <p role="status" className="mt-4 text-sm font-medium">
        {states[item.status] ?? "Состояние неизвестно"}
      </p>
      {item.error_code && (
        <p
          role="alert"
          className="mt-2 break-words text-xs text-severity-critical"
        >
          {item.error_code}. Автоматического повтора нет; принятый провайдером
          запрос мог быть оплачен.
        </p>
      )}
      {result && (
        <>
          <dl className="mt-4 grid grid-cols-3 gap-3 border-y border-border-soft py-3 text-xs">
            <div>
              <dt className="text-text-muted">Время</dt>
              <dd className="tnum mt-1">
                {seconds !== null && Number.isFinite(seconds)
                  ? `${seconds.toFixed(1)} с`
                  : "Нет данных"}
              </dd>
            </div>
            <div>
              <dt className="text-text-muted">Токены</dt>
              <dd className="tnum mt-1">
                {typeof usage?.total_tokens === "number"
                  ? usage.total_tokens
                  : "Нет данных"}
              </dd>
            </div>
            <div>
              <dt className="text-text-muted">Стоимость провайдера</dt>
              <dd className="tnum mt-1">
                {typeof usage?.cost === "number"
                  ? usage.cost.toFixed(5)
                  : "Нет данных"}
              </dd>
            </div>
          </dl>
          <p className="mt-2 text-[11px] text-text-muted">
            Стоимость сообщена провайдером; единицу расчёта проверьте в его
            настройках. Отсутствующие значения не заменяются нулём.
          </p>
          <p className="mt-4 whitespace-pre-wrap text-sm">{result.summary}</p>
          {(result.hypotheses ?? []).map((hypothesis, i) => (
            <div
              key={i}
              className="mt-4 border-t border-border-soft pt-3 text-xs"
            >
              <p className="font-medium">{hypothesis.text}</p>
              {[
                ["Поддерживает", hypothesis.supporting_evidence_ids],
                ["Противоречит", hypothesis.contradicting_evidence_ids],
              ].map(([label, ids]) => (
                <div className="mt-2" key={label as string}>
                  <p className="text-text-muted">{label as string}</p>
                  {((ids as string[] | undefined) ?? []).map((id) => {
                    const evidence = snapshot.input.evidence.find(
                      (e) => e.evidence_id === id,
                    );
                    return (
                      <details
                        className="mt-1 rounded bg-surface-tertiary p-2"
                        key={id}
                      >
                        <summary className="cursor-pointer break-all">
                          {evidence?.source_type ?? "Доказательство"} · {id}
                        </summary>
                        <pre className="mt-2 whitespace-pre-wrap break-words">
                          {evidence?.excerpt ??
                            "Доказательство отсутствует в снимке"}
                        </pre>
                      </details>
                    );
                  })}
                </div>
              ))}
            </div>
          ))}
          <p className="mt-4 text-xs text-text-secondary">
            Не хватает данных:{" "}
            {result.missing_information?.join("; ") || "модель не указала"}
          </p>
          <p className="mt-2 text-xs text-text-secondary">
            Следующие проверки:{" "}
            {result.suggested_next_checks?.join("; ") || "не указаны"}
          </p>
          <form
            className="mt-5 space-y-3 border-t border-border-default pt-4"
            onSubmit={(event) => {
              event.preventDefault();
              review.mutate();
            }}
          >
            <Select
              label={`Оценка: ${item.profile.label}`}
              value={verdict}
              onChange={(event) => setVerdict(event.target.value)}
              options={[
                { value: "unrated", label: "Не оценено" },
                { value: "accurate", label: "Верно" },
                { value: "partial", label: "Частично верно" },
                { value: "incorrect", label: "Неверно" },
              ]}
            />
            <label className="block text-xs text-text-secondary">
              Комментарий к оценке
              <textarea
                maxLength={2000}
                className={`${textareaStyle} mt-2`}
                value={note}
                onChange={(event) => setNote(event.target.value)}
              />
            </label>
            <Button
              type="submit"
              size="sm"
              variant="secondary"
              disabled={review.isPending}
            >
              {review.isPending ? "Сохраняем…" : "Сохранить оценку"}
            </Button>
            {item.review.reviewed_by && (
              <p className="text-[11px] text-text-muted">
                {item.review.reviewed_by} ·{" "}
                {formatDate(item.review.reviewed_at!)} · ревизия{" "}
                {item.review.revision}
              </p>
            )}
            {review.isError && <ErrorMessage error={review.error} />}
          </form>
        </>
      )}
    </Card>
  );
}

export function ModelEvaluation() {
  const [params, setParams] = useSearchParams();
  const caseID = params.get("case") || "";
  const runID = params.get("run") || "";
  const sourceIncident = params.get("incident_id") || "";
  const sourceAnalysis = params.get("analysis_id") || "";
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<string[]>([]);
  const [confirmed, setConfirmed] = useState(false);
  const request = useRef<{ key: string; id: string } | null>(null);
  const qc = useQueryClient();
  const config = useQuery({
    queryKey: ["evaluation-config"],
    queryFn: () =>
      get<{ enabled: boolean; profiles: Profile[] }>("/evaluation/config"),
    retry: false,
  });
  const enabled = config.data?.enabled === true && !config.isError;
  const library = useQuery({
    queryKey: ["evaluation-cases", offset],
    queryFn: () =>
      get<{ items: CaseSummary[]; has_more: boolean; next_offset: number }>(
        `/evaluation/cases?offset=${offset}&limit=25`,
      ),
    enabled,
    retry: false,
  });
  const testCase = useQuery({
    queryKey: ["evaluation-case", caseID],
    queryFn: () => get<TestCase>(`/evaluation/cases/${path(caseID)}`),
    enabled: enabled && !!caseID,
    retry: false,
  });
  const runs = useQuery({
    queryKey: ["evaluation-runs", caseID],
    queryFn: () =>
      get<{
        items: { run_id: string; created_at: string; profiles: Profile[] }[];
      }>(`/evaluation/cases/${path(caseID)}/runs`),
    enabled: enabled && !!caseID,
    retry: false,
  });
  const run = useQuery({
    queryKey: ["evaluation-run", runID],
    queryFn: () => get<Run>(`/evaluation/runs/${path(runID)}`),
    enabled: enabled && !!runID,
    retry: false,
    refetchInterval: (query) =>
      query.state.data?.results.some(
        (item) => item.status === "queued" || item.status === "running",
      )
        ? 2000
        : false,
  });
  function chooseCase(id: string) {
    setParams({ case: id });
    setSelected([]);
    setConfirmed(false);
    request.current = null;
  }
  const launch = useMutation({
    mutationFn: () => {
      const key = `${caseID}:${[...selected].sort().join(",")}`;
      if (request.current?.key !== key)
        request.current = { key, id: crypto.randomUUID() };
      return post<{ run_id: string }>(
        `/evaluation/cases/${path(caseID)}/runs`,
        {
          profile_ids: selected,
          request_id: request.current.id,
          confirm_provider_calls: confirmed,
        },
      );
    },
    onSuccess: (result) => {
      setParams({ case: caseID, run: result.run_id });
      setConfirmed(false);
      request.current = null;
      qc.invalidateQueries({ queryKey: ["evaluation-runs", caseID] });
    },
  });
  const snapshot = run.data?.run.test_case;
  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="mb-2 flex items-center gap-2 text-xs text-text-muted">
            <FlaskConical size={15} />
            Лаборатория
          </p>
          <h1 className="page-title">Тестирование моделей</h1>
          <p className="page-description">
            Сравнивайте модели и версии промпта на одинаковых сохранённых
            данных. Ожидаемую причину видит инженер, модель её не получает.
          </p>
        </div>
      </header>
      {config.isPending ? (
        <p role="status">Загружаем лабораторию…</p>
      ) : config.isError ? (
        <Card>
          <ErrorMessage error={config.error} />
          <Button
            className="mt-3"
            variant="secondary"
            onClick={() => config.refetch()}
          >
            Повторить загрузку
          </Button>
        </Card>
      ) : !enabled ? (
        <Card title="Лаборатория ещё не включена">
          <p className="text-sm text-text-secondary">
            Администратор может включить тестирование и настроить список
            разрешённых моделей. После этого здесь появятся библиотека кейсов и
            сравнение результатов.
          </p>
        </Card>
      ) : (
        <>
          <CaseForm
            incident={sourceIncident}
            analysis={sourceAnalysis}
            onCreated={(id) => {
              chooseCase(id);
              qc.invalidateQueries({ queryKey: ["evaluation-cases"] });
            }}
          />
          <div className="grid min-w-0 gap-5 lg:grid-cols-[280px_minmax(0,1fr)]">
            <Card title="Библиотека кейсов" className="self-start">
              {library.isPending ? (
                <p className="text-xs">Загружаем кейсы…</p>
              ) : library.isError ? (
                <ErrorMessage error={library.error} />
              ) : library.data?.items.length === 0 ? (
                <p className="text-sm text-text-secondary">
                  Кейсов пока нет. Создайте свой сценарий или сохраните снимок
                  из истории анализа инцидента.
                </p>
              ) : (
                <ul className="space-y-2">
                  {library.data?.items.map((item) => (
                    <li key={item.case_id}>
                      <button
                        aria-pressed={caseID === item.case_id}
                        className={`w-full rounded-md border p-3 text-left ${caseID === item.case_id ? "border-accent bg-surface-tertiary" : "border-border-soft hover:bg-surface-primary"}`}
                        onClick={() => chooseCase(item.case_id)}
                      >
                        <span className="block break-words text-sm font-medium">
                          {item.name}
                        </span>
                        <span className="mt-1 block text-[11px] text-text-muted">
                          {item.source === "incident"
                            ? item.source_incident_id
                            : "Свой сценарий"}{" "}
                          · {item.has_reference ? "С эталоном" : "Без эталона"}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
              <div className="mt-4 flex justify-between">
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label="Предыдущие кейсы"
                  disabled={offset === 0}
                  onClick={() => setOffset(Math.max(0, offset - 25))}
                >
                  <ArrowLeft size={15} />
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  aria-label="Следующие кейсы"
                  disabled={!library.data?.has_more}
                  onClick={() => setOffset(library.data!.next_offset)}
                >
                  <ArrowRight size={15} />
                </Button>
              </div>
            </Card>
            <div className="min-w-0 space-y-5">
              {!caseID ? (
                <Card title="Выберите тестовый кейс">
                  <p className="text-sm text-text-secondary">
                    У каждого кейса неизменяемый снимок данных. Запуски
                    сравнения хранятся отдельно от рабочего анализа и не
                    отправляют уведомления.
                  </p>
                </Card>
              ) : testCase.isPending ? (
                <p>Загружаем снимок…</p>
              ) : testCase.isError ? (
                <ErrorMessage error={testCase.error} />
              ) : (
                testCase.data && (
                  <>
                    <Card title={testCase.data.name}>
                      <p className="text-xs text-text-muted">
                        Создан {formatDate(testCase.data.created_at)} ·{" "}
                        {testCase.data.created_by} · наблюдений:{" "}
                        {testCase.data.input.evidence.length}
                      </p>
                      {testCase.data.source_incident_id && (
                        <Link
                          className="mt-2 inline-block text-xs text-accent underline"
                          to={`/incidents/${path(testCase.data.source_incident_id)}`}
                        >
                          Исходный инцидент: {testCase.data.source_incident_id}
                        </Link>
                      )}
                      <details className="mt-3 text-xs">
                        <summary className="cursor-pointer">
                          Данные снимка и эталон
                        </summary>
                        <p className="mt-2 break-all font-mono text-[11px] text-text-muted">
                          SHA-256: {testCase.data.snapshot_hash}
                        </p>
                        <p className="mt-3 whitespace-pre-wrap">
                          Ожидаемая причина:{" "}
                          {testCase.data.reference.cause || "Не задана"}
                        </p>
                        <p className="mt-1 text-text-muted">
                          {testCase.data.reference.kind ===
                          "confirmed_resolution"
                            ? `Подтверждённое решение · ревизия ${testCase.data.reference.resolution_revision}`
                            : "Эталон инженера"}
                          . Эталон не включается в запрос к модели.
                        </p>
                        {testCase.data.input.evidence.map((e) => (
                          <details
                            key={e.evidence_id}
                            className="mt-2 rounded bg-surface-tertiary p-3"
                          >
                            <summary className="cursor-pointer break-all">
                              {e.source_type} · {e.availability} ·{" "}
                              {e.evidence_id}
                            </summary>
                            <pre className="mt-2 whitespace-pre-wrap break-words">
                              {e.excerpt}
                            </pre>
                          </details>
                        ))}
                      </details>
                    </Card>
                    <Card title="Модели для сравнения">
                      {config.data.profiles.length === 0 && (
                        <p
                          role="status"
                          className="mb-4 text-sm text-text-secondary"
                        >
                          Конфигурации моделей недоступны. Сохранённые кейсы и
                          результаты остаются доступны.
                        </p>
                      )}
                      <fieldset>
                        <legend className="mb-3 text-xs text-text-muted">
                          Выберите от одной до четырёх конфигураций
                        </legend>
                        <div className="grid gap-3 xl:grid-cols-2">
                          {config.data.profiles.map((profile) => (
                            <label
                              key={profile.id}
                              className="flex min-w-0 cursor-pointer items-start gap-3 rounded-md border border-border-default p-3"
                            >
                              <input
                                className="mt-1 shrink-0 accent-accent"
                                type="checkbox"
                                checked={selected.includes(profile.id)}
                                disabled={
                                  launch.isPending ||
                                  (!selected.includes(profile.id) &&
                                    selected.length >= 4)
                                }
                                onChange={(event) => {
                                  setSelected((current) =>
                                    event.target.checked
                                      ? [...current, profile.id]
                                      : current.filter(
                                          (id) => id !== profile.id,
                                        ),
                                  );
                                  setConfirmed(false);
                                }}
                              />
                              <span className="min-w-0">
                                <span className="block text-sm font-medium">
                                  {profile.label}
                                </span>
                                <span className="mt-1 block break-all font-mono text-[11px] text-text-muted">
                                  {profile.model}
                                </span>
                                <span className="mt-1 block break-all text-[11px] text-text-secondary">
                                  {profile.prompt_version} · до{" "}
                                  {profile.max_tokens} токенов
                                </span>
                              </span>
                            </label>
                          ))}
                        </div>
                      </fieldset>
                      <p className="mt-4 text-xs text-text-muted">
                        Один запрос провайдеру на конфигурацию, без
                        автоматического платного повтора. Новые данные и база
                        знаний не запрашиваются. Не более одного сравнения в
                        минуту.
                      </p>
                      <label className="mt-4 flex items-start gap-2 text-xs text-text-secondary">
                        <input
                          className="mt-0.5 shrink-0 accent-accent"
                          type="checkbox"
                          checked={confirmed}
                          onChange={(event) =>
                            setConfirmed(event.target.checked)
                          }
                        />
                        Разрешаю {selected.length} запрос(а) к выбранным
                        моделям. Провайдер может списать оплату.
                      </label>
                      <Button
                        className="mt-4"
                        disabled={
                          !config.data.profiles.length ||
                          !selected.length ||
                          !confirmed ||
                          launch.isPending ||
                          testCase.isError
                        }
                        onClick={() => launch.mutate()}
                      >
                        <Play size={15} />
                        {launch.isPending
                          ? "Создаём сравнение…"
                          : `Запустить сравнение · ${selected.length}`}
                      </Button>
                      {launch.isError && <ErrorMessage error={launch.error} />}
                    </Card>
                    {runs.isError && <ErrorMessage error={runs.error} />}
                    {!!runs.data?.items.length && (
                      <Select
                        label="История сравнений"
                        value={runID}
                        onChange={(event) =>
                          setParams({
                            case: caseID,
                            ...(event.target.value
                              ? { run: event.target.value }
                              : {}),
                          })
                        }
                        options={[
                          { value: "", label: "Выберите запуск" },
                          ...runs.data.items.map((item) => ({
                            value: item.run_id,
                            label: `${formatDate(item.created_at)} · ${item.profiles.map((p) => p.label).join(", ")}`,
                          })),
                        ]}
                      />
                    )}
                  </>
                )
              )}
            </div>
          </div>
          {runID && (
            <section aria-label="Результаты сравнения" className="space-y-4">
              <h2 className="text-xl font-semibold">Результаты сравнения</h2>
              {run.isPending ? (
                <p role="status">Загружаем результаты…</p>
              ) : run.isError ? (
                <ErrorMessage error={run.error} />
              ) : (
                run.data &&
                snapshot && (
                  <>
                    <p className="text-xs text-text-muted">
                      {snapshot.name} · {formatDate(run.data.run.created_at)} ·{" "}
                      {run.data.run.created_by}. Результаты оцениваются
                      инженером; автоматического процента правильности нет.
                    </p>
                    {run.data.paused && (
                      <p
                        role="status"
                        className="text-sm text-severity-warning"
                      >
                        AI на паузе или управление недоступно. Ожидающие запуски
                        начнутся после включения.
                      </p>
                    )}
                    <Card title="Эталон этого запуска">
                      <p className="whitespace-pre-wrap text-sm">
                        {snapshot.reference.cause ||
                          "Эталон не задан; оцените результат по данным снимка."}
                      </p>
                      <p className="mt-2 break-all font-mono text-[11px] text-text-muted">
                        Снимок: {snapshot.snapshot_hash}
                      </p>
                    </Card>
                    <div className="grid min-w-0 gap-4 xl:grid-cols-2">
                      {run.data.results.map((item) => (
                        <ResultCard
                          key={`${item.job_id}:${item.review.revision}`}
                          item={item}
                          runID={runID}
                          snapshot={snapshot}
                        />
                      ))}
                    </div>
                  </>
                )
              )}
            </section>
          )}
        </>
      )}
    </div>
  );
}
