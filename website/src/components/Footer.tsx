import { BrandMark } from "./BrandMark";
import type { Language, Translation } from "../i18n/types";
import { buildRoute, routes } from "../data/navigation";

type FooterProps = {
  t: Translation;
  language: Language;
};

export function Footer({ t, language }: FooterProps) {
  return (
    <footer className="site-footer">
      <div>
        <a className="brand" href={buildRoute(language, "")}>
          <BrandMark />
          <span>IncidentGPT</span>
        </a>
        <p>{t.hero.caption}</p>
      </div>
      <nav aria-label={t.common.navigation}>
        {routes.map((route) => (
          <a key={route.id} href={buildRoute(language, route.path)}>
            {t.navigation[route.id]}
          </a>
        ))}
      </nav>
    </footer>
  );
}
