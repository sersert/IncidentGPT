import { Github, Menu, Moon, Sun, Monitor } from "lucide-react";
import type { Language, ThemeMode, Translation } from "../i18n/types";
import { buildRoute, routes, type PageId } from "../data/navigation";
import { Search } from "./Search";

type HeaderProps = {
  t: Translation;
  language: Language;
  theme: ThemeMode;
  activePage: PageId;
  onLanguageChange: (language: Language) => void;
  onThemeChange: (theme: ThemeMode) => void;
  onMenuOpen: () => void;
};

export function Header({ t, language, theme, activePage, onLanguageChange, onThemeChange, onMenuOpen }: HeaderProps) {
  const ThemeIcon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;

  return (
    <header className="site-header">
      <a className="brand" href={buildRoute(language, "")} aria-label="IncidentGPT overview">
        <span className="brand-mark">IG</span>
        <span>IncidentGPT</span>
      </a>
      <nav className="desktop-nav" aria-label="Primary">
        {routes.slice(0, 6).map((route) => (
          <a key={route.id} href={buildRoute(language, route.path)} className={activePage === route.id ? "active" : ""}>
            {t.navigation[route.id]}
          </a>
        ))}
        <a href="https://github.com/sersert/IncidentGPT" target="_blank" rel="noreferrer">
          {t.common.github}
        </a>
      </nav>
      <div className="header-tools">
        <Search t={t} language={language} />
        <div className="segmented" aria-label={t.common.language}>
          <button type="button" className={language === "ru" ? "selected" : ""} onClick={() => onLanguageChange("ru")}>
            RU
          </button>
          <button type="button" className={language === "en" ? "selected" : ""} onClick={() => onLanguageChange("en")}>
            EN
          </button>
        </div>
        <button
          className="icon-button"
          type="button"
          onClick={() => onThemeChange(theme === "dark" ? "light" : theme === "light" ? "system" : "dark")}
          aria-label={t.common.theme}
          title={`${t.common.theme}: ${theme}`}
        >
          <ThemeIcon size={18} />
        </button>
        <a className="icon-button" href="https://github.com/sersert/IncidentGPT" target="_blank" rel="noreferrer" aria-label="GitHub">
          <Github size={18} />
        </a>
        <button className="mobile-menu-button" type="button" onClick={onMenuOpen} aria-label="Open menu">
          <Menu size={22} />
        </button>
      </div>
    </header>
  );
}
