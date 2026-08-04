import { X } from "lucide-react";
import { buildRoute, routes, type PageId } from "../data/navigation";
import type { Language, ThemeMode, Translation } from "../i18n/types";

type MobileNavigationProps = {
  open: boolean;
  t: Translation;
  language: Language;
  theme: ThemeMode;
  activePage: PageId;
  onClose: () => void;
  onLanguageChange: (language: Language) => void;
  onThemeChange: (theme: ThemeMode) => void;
};

export function MobileNavigation({
  open,
  t,
  language,
  theme,
  activePage,
  onClose,
  onLanguageChange,
  onThemeChange,
}: MobileNavigationProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="mobile-panel" role="dialog" aria-modal="true" aria-label="Navigation">
      <div className="mobile-panel-head">
        <span>IncidentGPT</span>
        <button className="icon-button" type="button" onClick={onClose} aria-label="Close menu">
          <X size={20} />
        </button>
      </div>
      <nav aria-label="Mobile">
        {routes.map((route) => (
          <a key={route.id} href={buildRoute(language, route.path)} className={activePage === route.id ? "active" : ""} onClick={onClose}>
            {t.navigation[route.id]}
          </a>
        ))}
        <a href="https://github.com/sersert/IncidentGPT" target="_blank" rel="noreferrer" onClick={onClose}>
          {t.common.github}
        </a>
      </nav>
      <div className="mobile-controls">
        <div className="segmented">
          <button type="button" className={language === "ru" ? "selected" : ""} onClick={() => onLanguageChange("ru")}>
            RU
          </button>
          <button type="button" className={language === "en" ? "selected" : ""} onClick={() => onLanguageChange("en")}>
            EN
          </button>
        </div>
        <select value={theme} onChange={(event) => onThemeChange(event.target.value as ThemeMode)} aria-label={t.common.theme}>
          <option value="system">{t.common.system}</option>
          <option value="dark">{t.common.dark}</option>
          <option value="light">{t.common.light}</option>
        </select>
      </div>
    </div>
  );
}
