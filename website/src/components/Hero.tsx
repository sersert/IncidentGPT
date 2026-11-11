import { ArrowRight, Github } from "lucide-react";
import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";
import { productUrl } from "../lib/product";
import { IncidentPreview } from "./IncidentPreview";

type HeroProps = { t: Translation; language: Language };

export function Hero({ t, language }: HeroProps) {
  return (
    <section className="hero-section">
      <div className="hero-copy">
        <p className="eyebrow">
          <span className="status-dot" />
          {t.hero.eyebrow}
        </p>
        <h1>{t.hero.title}</h1>
        <p className="hero-description">{t.hero.description}</p>
        <div className="hero-actions">
          {productUrl ? (
            <a
              className="button primary"
              href={productUrl}
              target="_blank"
              rel="noreferrer"
            >
              {t.common.signIn}
              <ArrowRight size={16} />
            </a>
          ) : null}
          <a
            className={productUrl ? "button" : "button primary"}
            href={buildRoute(language, "installation")}
          >
            {t.hero.installButton}
            <ArrowRight size={16} />
          </a>
          <a
            className="button"
            href="https://github.com/sersert/IncidentGPT"
            target="_blank"
            rel="noreferrer"
          >
            <Github size={17} />
            {t.hero.githubButton}
          </a>
        </div>
        <p className="hero-caption">{t.home.humanNote}</p>
      </div>
      <IncidentPreview t={t} />
    </section>
  );
}
