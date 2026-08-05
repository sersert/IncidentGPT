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
        <strong>IncidentGPT</strong>
        <p>{t.home.humanNote}</p>
      </div>
      <nav aria-label="Footer">
        {routes.map((route) => (
          <a key={route.id} href={buildRoute(language, route.path)}>
            {t.navigation[route.id]}
          </a>
        ))}
      </nav>
    </footer>
  );
}
