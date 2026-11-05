import { ArrowUpRight, Check, Layers } from "lucide-react";
import { productUrl } from "../lib/product";
import { BrandMark } from "./BrandMark";
import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";

export function Hero({ t, language }: { t: Translation; language: Language }) {
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
          {productUrl ? (
            <a
              className="button"
              href={productUrl}
              target="_blank"
              rel="noreferrer"
            >
              {t.hero.productButton}
              <ArrowUpRight size={16} />
            </a>
          ) : null}
        </div>
        <p className="hero-note">{t.home.humanNote}</p>
        <a
          className="hero-source text-link"
          href="https://github.com/sersert/IncidentGPT"
          target="_blank"
          rel="noreferrer"
        >
          GitHub <ArrowUpRight size={13} />
        </a>
      </div>
      <div className="workspace-preview" aria-label={t.hero.preview.label}>
        <div className="preview-toolbar">
          <span>
            <BrandMark size={17} />
            IncidentGPT
          </span>
          <span>{t.hero.preview.example}</span>
        </div>
        <div className="preview-content">
          <div className="preview-breadcrumb">
            {t.hero.preview.incidents}
            <span>/</span>INC-042
          </div>
          <div className="preview-heading">
            <span className="preview-severity">P2</span>
            <span>{t.hero.preview.status}</span>
          </div>
          <h2>{t.hero.preview.title}</h2>
          <p className="preview-meta">
            monitoring <span>·</span> synthetic-pod-1
          </p>
          <div className="preview-tabs">
            <span>{t.navigation.overview}</span>
            <span>{t.hero.preview.alerts}</span>
            <span>{t.hero.preview.context}</span>
          </div>
          <div className="preview-analysis">
            <span className="preview-label">
              <Layers size={14} />
              {t.hero.preview.analysis}
            </span>
            <p>{t.hero.preview.body}</p>
          </div>
          <div className="preview-evidence">
            <Check size={14} />
            Prometheus
            <Check size={14} />
            Kubernetes
            <Check size={14} />
            {t.hero.preview.logs}
          </div>
        </div>
        <div className="preview-caption">{t.hero.preview.caption}</div>
      </div>
    </section>
  );
}
