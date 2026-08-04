import { Download, RotateCcw } from "lucide-react";
import { useMemo, useState } from "react";
import type { Translation } from "../i18n/types";
import {
  defaultGeneratorValues,
  generateAiWorkerValues,
  generateAlertmanagerSnippet,
  generateEnricherValues,
  type GeneratorValues,
} from "../lib/generator";
import { CodeBlock } from "./CodeBlock";

type ConfigGeneratorProps = {
  t: Translation;
};

type OutputKind = "ai" | "enricher" | "alertmanager";

export function ConfigGenerator({ t }: ConfigGeneratorProps) {
  const [values, setValues] = useState<GeneratorValues>(defaultGeneratorValues);
  const [kind, setKind] = useState<OutputKind>("ai");
  const output = useMemo(() => {
    if (kind === "ai") {
      return generateAiWorkerValues(values);
    }
    if (kind === "enricher") {
      return generateEnricherValues(values);
    }
    return generateAlertmanagerSnippet();
  }, [kind, values]);

  function updateField(key: keyof GeneratorValues, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
  }

  function download() {
    const filename = kind === "ai" ? "values-ai-worker.yaml" : kind === "enricher" ? "values-enricher.yaml" : "alertmanager-incidentgpt.yaml";
    const blob = new Blob([output], { type: "text/yaml;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }

  return (
    <section className="generator" id="generator">
      <div className="section-heading">
        <h2>{t.generator.title}</h2>
        <p>{t.generator.description}</p>
      </div>
      <div className="generator-grid">
        <form className="generator-form">
          {(Object.keys(defaultGeneratorValues) as (keyof GeneratorValues)[]).map((key) => (
            <label key={key}>
              <span>{t.generator.fields[key]}</span>
              <input value={values[key]} onChange={(event) => updateField(key, event.target.value)} />
            </label>
          ))}
        </form>
        <div>
          <div className="tabs">
            <button type="button" className={kind === "ai" ? "selected" : ""} onClick={() => setKind("ai")}>
              {t.generator.aiWorker}
            </button>
            <button type="button" className={kind === "enricher" ? "selected" : ""} onClick={() => setKind("enricher")}>
              {t.generator.enricher}
            </button>
            <button type="button" className={kind === "alertmanager" ? "selected" : ""} onClick={() => setKind("alertmanager")}>
              {t.generator.alertmanager}
            </button>
          </div>
          <div className="generator-actions">
            <button className="icon-text" type="button" onClick={download}>
              <Download size={16} />
              {t.common.download}
            </button>
            <button className="icon-text" type="button" onClick={() => setValues(defaultGeneratorValues)}>
              <RotateCcw size={16} />
              {t.common.reset}
            </button>
          </div>
          <CodeBlock code={output} language="yaml" copyLabel={t.common.copy} copiedLabel={t.common.copied} />
        </div>
      </div>
    </section>
  );
}
