import { ArrowUpRight, Menu, Moon, Sun, Monitor } from "lucide-react";
import type { Language, ThemeMode, Translation } from "../i18n/types";
import { buildRoute, routes, type PageId } from "../data/navigation";
import type { RefObject } from "react";
import { productUrl } from "../lib/product";
import { Search } from "./Search";
import { BrandMark } from "./BrandMark";

type HeaderProps = {
  t: Translation;
  language: Language;
  theme: ThemeMode;
  activePage: PageId;
  onLanguageChange: (language: Language) => void;
  onThemeChange: (theme: ThemeMode) => void;
  onMenuOpen: () => void;
  menuButtonRef: RefObject<HTMLButtonElement | null>;
  menuOpen: boolean;
};

export function Header({
  t,
  language,
  theme,
  activePage,
  onLanguageChange,
  onThemeChange,
  onMenuOpen,
  menuButtonRef,
  menuOpen,
}: HeaderProps) {
  const ThemeIcon = theme === "dark" ? Moon : theme === "light" ? Sun : Monitor;

  return (
    <header className="site-header">
      <a
        className="brand"
        href={buildRoute(language, "")}
        aria-label="IncidentGPT overview"
      >
        <BrandMark size={26} />
        <span>IncidentGPT</span>
      </a>
      <nav className="desktop-nav" aria-label="Primary">
        {routes
          .filter((route) =>
            ["installation", "architecture", "configuration"].includes(
              route.id,
            ),
          )
          .map((route) => (
            <a
              key={route.id}
              href={buildRoute(language, route.path)}
              className={activePage === route.id ? "active" : ""}
            >
              {t.navigation[route.id]}
            </a>
          ))}
      </nav>
      <div className="header-tools">
        <Search t={t} language={language} />
        <div className="segmented" aria-label={t.common.language}>
          <button
            type="button"
            className={language === "ru" ? "selected" : ""}
            onClick={() => onLanguageChange("ru")}
          >
            RU
          </button>
          <button
            type="button"
            className={language === "en" ? "selected" : ""}
            onClick={() => onLanguageChange("en")}
          >
            EN
          </button>
        </div>
        <button
          className="icon-button"
          type="button"
          onClick={() =>
            onThemeChange(
              theme === "dark"
                ? "light"
                : theme === "light"
                  ? "system"
                  : "dark",
            )
          }
          aria-label={t.common.theme}
          title={`${t.common.theme}: ${theme}`}
        >
          <ThemeIcon size={18} />
        </button>
        {productUrl ? (
          <a
            className="button product-link"
            href={productUrl}
            target="_blank"
            rel="noreferrer"
          >
            {t.common.signIn}
            <ArrowUpRight size={15} />
          </a>
        ) : null}
        <button
          className="mobile-menu-button"
          type="button"
          onClick={onMenuOpen}
          aria-label={t.common.openMenu}
          ref={menuButtonRef}
          aria-controls="documentation-menu"
          aria-expanded={menuOpen}
        >
          <Menu size={22} />
        </button>
      </div>
    </header>
  );
}
