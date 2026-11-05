import { ArrowUpRight, BookOpen, Check, Layers } from "lucide-react";
import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";

export function Hero({ t, language }: { t: Translation; language: Language }) {
  const ru = language === "ru";
  return (
    <section className="hero-section">
      <div className="hero-copy">
        <h1>{t.hero.title}</h1>
        <p className="hero-description">{t.hero.description}</p>
        <div className="hero-actions">
          <a
            className="button primary"
            href={buildRoute(language, "installation")}
          >
            {t.hero.installButton}
          </a>
          <a
            className="button"
            href="https://incidentgpt.sersert.io/"
            target="_blank"
            rel="noreferrer"
          >
            {ru ? "Открыть продукт" : "Open workspace"}
            <ArrowUpRight size={16} />
          </a>
        </div>
        <a
          className="hero-source text-link"
          href="https://github.com/sersert/IncidentGPT"
          target="_blank"
          rel="noreferrer"
        >
          GitHub <ArrowUpRight size={13} />
        </a>
      </div>
      <div
        className="workspace-preview"
        aria-label={
          ru
            ? "Демонстрационный пример инцидента"
            : "Illustrative incident example"
        }
      >
        <div className="preview-toolbar">
          <span>
            <BookOpen size={15} />
            IncidentGPT
          </span>
          <span>{ru ? "Пример интерфейса" : "UI example"}</span>
        </div>
        <div className="preview-content">
          <div className="preview-breadcrumb">
            {ru ? "Инциденты" : "Incidents"}
            <span>/</span>INC-042
          </div>
          <div className="preview-heading">
            <span className="preview-severity">P2</span>
            <span>{ru ? "Требует внимания" : "Needs attention"}</span>
          </div>
          <h2>{ru ? "Высокая нагрузка на CPU" : "High CPU utilization"}</h2>
          <p className="preview-meta">
            monitoring <span>·</span> synthetic-pod-1
          </p>
          <div className="preview-tabs">
            <span>{ru ? "Обзор" : "Overview"}</span>
            <span>{ru ? "Алерты · 3" : "Alerts · 3"}</span>
            <span>{ru ? "Контекст" : "Context"}</span>
          </div>
          <div className="preview-analysis">
            <span className="preview-label">
              <Layers size={14} />
              {ru ? "Черновик AI-разбора" : "AI analysis draft"}
            </span>
            <p>
              {ru
                ? "Насыщение CPU совпало с ростом задержек. Проверьте лимиты ресурсов и последние изменения workload."
                : "CPU saturation coincides with rising latency. Check resource limits and recent workload changes."}
            </p>
          </div>
          <div className="preview-evidence">
            <Check size={14} />
            Prometheus
            <Check size={14} />
            Kubernetes
            <Check size={14} />
            {ru ? "Логи" : "Logs"}
          </div>
        </div>
        <div className="preview-caption">
          {ru
            ? "Демонстрационные данные · решение принимает инженер"
            : "Illustrative data · the engineer makes the decision"}
        </div>
      </div>
    </section>
  );
}
