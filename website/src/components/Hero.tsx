import { ArrowDown, Github, Rocket } from "lucide-react";
import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";

type HeroProps = {
  t: Translation;
  language: Language;
};

export function Hero({ t, language }: HeroProps) {
  return (
    <section className="hero-section">
      <div className="hero-copy">
        <p className="eyebrow">{t.hero.eyebrow}</p>
        <h1>{t.hero.title}</h1>
        <p className="hero-description">{t.hero.description}</p>
        <div className="hero-actions">
          <a className="button primary" href={buildRoute(language, "installation")}>
            <Rocket size={18} />
            {t.hero.installButton}
          </a>
          <a className="button" href="https://github.com/sersert/IncidentGPT" target="_blank" rel="noreferrer">
            <Github size={18} />
            {t.hero.githubButton}
          </a>
          <a className="text-link" href={buildRoute(language, "architecture")}>
            {t.hero.architectureLink}
          </a>
        </div>
      </div>
      <div className="flow-card" aria-label="Alert flow visualization">
        {t.hero.steps.map((step, index) => (
          <div key={step} className="flow-row">
            <span>{step}</span>
            {index < t.hero.steps.length - 1 ? <ArrowDown size={18} aria-hidden="true" /> : null}
          </div>
        ))}
      </div>
    </section>
  );
}
