import { useEffect, useRef, type RefObject } from "react";
import { Search } from "./Search";
import { BrandMark } from "./BrandMark";
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
  triggerRef: RefObject<HTMLButtonElement | null>;
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
  triggerRef,
  onLanguageChange,
  onThemeChange,
}: MobileNavigationProps) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (!open) return;
    const element = dialog.current;
    const trigger = triggerRef.current;
    if (element && !element.open) element.showModal();
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      if (element?.open) element.close();
      document.body.style.overflow = previous;
      trigger?.focus();
    };
  }, [open, triggerRef]);
  if (!open) {
    return null;
  }

  return (
    <dialog
      ref={dialog}
      id="documentation-menu"
      className="mobile-panel"
      aria-label={t.common.mobileNavigation}
      onCancel={onClose}
    >
      <div className="mobile-panel-head">
        <span className="brand">
          <BrandMark size={26} />
          IncidentGPT
        </span>
        <button
          className="icon-button"
          type="button"
          onClick={onClose}
          autoFocus
          aria-label={t.common.closeMenu}
        >
          <X size={20} />
        </button>
      </div>
      <nav aria-label="Mobile">
        {routes.map((route) => (
          <a
            key={route.id}
            href={buildRoute(language, route.path)}
            className={activePage === route.id ? "active" : ""}
            onClick={onClose}
          >
            {t.navigation[route.id]}
          </a>
        ))}
        <a
          href="https://github.com/sersert/IncidentGPT"
          target="_blank"
          rel="noreferrer"
          onClick={onClose}
        >
          {t.common.github}
        </a>
      </nav>
      <div className="mobile-controls">
        <div className="mobile-search">
          <Search t={t} language={language} onNavigate={onClose} />
        </div>
        <div className="segmented">
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
        <select
          value={theme}
          onChange={(event) => onThemeChange(event.target.value as ThemeMode)}
          aria-label={t.common.theme}
        >
          <option value="system">{t.common.system}</option>
          <option value="dark">{t.common.dark}</option>
          <option value="light">{t.common.light}</option>
        </select>
      </div>
    </dialog>
  );
}
