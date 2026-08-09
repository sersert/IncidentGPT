import { useEffect, useMemo, useState } from "react";
import { DocsSidebar } from "./components/DocsSidebar";
import { Footer } from "./components/Footer";
import { Header } from "./components/Header";
import { MobileNavigation } from "./components/MobileNavigation";
import { TableOfContents, type TocItem } from "./components/TableOfContents";
import { getRouteByPath, routes, type PageId } from "./data/navigation";
import { detectInitialLanguage, translations } from "./i18n";
import type { Language, ThemeMode, Translation } from "./i18n/types";
import { ArchitecturePage } from "./pages/ArchitecturePage";
import { ConfigurationPage } from "./pages/ConfigurationPage";
import { ExamplesPage } from "./pages/ExamplesPage";
import { HomePage } from "./pages/HomePage";
import { InstallationPage } from "./pages/InstallationPage";
import { LimitationsPage } from "./pages/LimitationsPage";
import { LogsPage } from "./pages/LogsPage";
import { TroubleshootingPage } from "./pages/TroubleshootingPage";
import { WebUiPage } from "./pages/WebUiPage";

type DocPageId = Exclude<PageId, "overview">;

type ParsedHash = {
  language: Language;
  path: string;
};

function parseHash(fallbackLanguage: Language): ParsedHash {
  const raw = window.location.hash.replace(/^#\/?/, "");
  const [first = "", ...rest] = raw.split("/").filter(Boolean);
  if (first === "ru" || first === "en") {
    return { language: first, path: rest.join("/") };
  }
  return { language: fallbackLanguage, path: raw };
}

function getThemeFromStorage(): ThemeMode {
  const stored = localStorage.getItem("incidentgpt-theme");
  return stored === "dark" || stored === "light" || stored === "system" ? stored : "system";
}

function resolveTheme(theme: ThemeMode): "dark" | "light" {
  if (theme === "system") {
    return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  }
  return theme;
}

function pageToc(page: PageId, t: Translation): TocItem[] {
  if (page === "overview") {
    return [
      { id: "features", label: t.home.featuresTitle },
      { id: "quick-start", label: t.home.quickStartTitle },
    ];
  }
  const map: Record<PageId, TocItem[]> = {
    overview: [],
    architecture: [
      { id: "system-map", label: t.architecture.headings.systemMap },
      { id: "alert-flow", label: t.architecture.headings.alertFlow },
      { id: "sequence", label: t.architecture.headings.sequence },
      { id: "correlation", label: t.architecture.headings.correlation },
      { id: "sanitization", label: t.architecture.headings.sanitization },
      { id: "failures", label: t.architecture.headings.failures },
    ],
    installation: [
      { id: "prerequisites", label: t.installation.headings.prerequisites },
      { id: "telegram", label: t.installation.headings.telegram },
      { id: "openrouter", label: t.installation.headings.openrouter },
      { id: "secrets", label: t.installation.headings.secrets },
      { id: "helm", label: t.installation.headings.helm },
      { id: "images", label: t.installation.headings.images },
      { id: "alertmanager", label: t.installation.headings.alertmanager },
      { id: "verify", label: t.installation.headings.verify },
    ],
    configuration: [
      { id: "upgrade-0-2-0", label: t.configuration.headings.upgrade },
      { id: "enricher-env", label: t.configuration.headings.enricherEnv },
      { id: "ai-worker-env", label: t.configuration.headings.aiWorkerEnv },
      { id: "backend-env", label: t.configuration.headings.backendEnv },
      { id: "promql", label: t.configuration.headings.promql },
      { id: "runbooks", label: t.configuration.headings.runbooks },
      { id: "generator", label: t.generator.title },
    ],
    webUi: [
      { id: "components", label: t.webUi.headings.components },
      { id: "wiring", label: t.webUi.headings.wiring },
      { id: "alerts-vs-incidents", label: t.webUi.headings.alertsVsIncidents },
      { id: "storage", label: t.webUi.headings.storage },
      { id: "access", label: t.webUi.headings.access },
      { id: "screens", label: t.webUi.headings.screens },
      { id: "data-sources", label: t.webUi.headings.dataSources },
    ],
    logs: [
      { id: "why", label: t.logs.headings.why },
      { id: "requirements", label: t.logs.headings.requirements },
      { id: "wiring", label: t.logs.headings.wiring },
      { id: "collector", label: t.logs.headings.collector },
      { id: "prompt", label: t.logs.headings.prompt },
    ],
    examples: [
      { id: "direct-enricher", label: t.examples.headings.direct },
      { id: "cascade", label: t.examples.headings.cascade },
      { id: "prometheusrule", label: t.examples.headings.prometheusRule },
      { id: "production", label: t.examples.headings.production },
      { id: "telegram-mock", label: t.examples.headings.telegramMock },
    ],
    troubleshooting: [],
    limitations: [
      { id: "correlation-limits", label: t.limitations.headings.correlation },
      { id: "llm", label: t.limitations.headings.llm },
      { id: "storage", label: t.limitations.headings.storage },
      { id: "security", label: t.limitations.headings.security },
      { id: "scaling", label: t.limitations.headings.scaling },
    ],
  };
  return map[page];
}

function renderPage(page: PageId, t: Translation, language: Language) {
  switch (page) {
    case "overview":
      return <HomePage t={t} language={language} />;
    case "architecture":
      return <ArchitecturePage t={t} />;
    case "installation":
      return <InstallationPage t={t} />;
    case "configuration":
      return <ConfigurationPage t={t} />;
    case "webUi":
      return <WebUiPage t={t} />;
    case "logs":
      return <LogsPage t={t} />;
    case "examples":
      return <ExamplesPage t={t} />;
    case "troubleshooting":
      return <TroubleshootingPage t={t} />;
    case "limitations":
      return <LimitationsPage t={t} />;
  }
}

export default function App() {
  const [language, setLanguageState] = useState<Language>(() => detectInitialLanguage());
  const [theme, setThemeState] = useState<ThemeMode>(() => getThemeFromStorage());
  const [path, setPath] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const route = getRouteByPath(path);
  const t = translations[language];

  useEffect(() => {
    function syncHash() {
      const parsed = parseHash(language);
      setLanguageState(parsed.language);
      setPath(parsed.path);
      localStorage.setItem("incidentgpt-language", parsed.language);
    }
    syncHash();
    window.addEventListener("hashchange", syncHash);
    return () => window.removeEventListener("hashchange", syncHash);
  }, [language]);

  useEffect(() => {
    document.documentElement.lang = language;
    document.title = t.meta.title;
    const description = document.querySelector<HTMLMetaElement>('meta[name="description"]');
    if (description) {
      description.content = t.meta.description;
    }
  }, [language, t]);

  useEffect(() => {
    const apply = () => {
      document.documentElement.dataset.theme = resolveTheme(theme);
    };
    apply();
    localStorage.setItem("incidentgpt-theme", theme);
    const query = window.matchMedia("(prefers-color-scheme: light)");
    query.addEventListener("change", apply);
    return () => query.removeEventListener("change", apply);
  }, [theme]);

  function setLanguage(next: Language) {
    localStorage.setItem("incidentgpt-language", next);
    const targetRoute = routes.find((item) => item.id === route.id) ?? routes[0];
    window.location.hash = `/${next}${targetRoute.path ? `/${targetRoute.path}` : ""}`;
    setLanguageState(next);
  }

  function setTheme(next: ThemeMode) {
    setThemeState(next);
  }

  const toc = useMemo(() => pageToc(route.id, t), [route.id, t]);
  const isHome = route.id === "overview";
  const docPage = isHome ? null : (route.id as DocPageId);

  return (
    <>
      <Header
        t={t}
        language={language}
        theme={theme}
        activePage={route.id}
        onLanguageChange={setLanguage}
        onThemeChange={setTheme}
        onMenuOpen={() => setMenuOpen(true)}
      />
      <MobileNavigation
        open={menuOpen}
        t={t}
        language={language}
        theme={theme}
        activePage={route.id}
        onClose={() => setMenuOpen(false)}
        onLanguageChange={setLanguage}
        onThemeChange={setTheme}
      />
      <main className={isHome ? "home-main" : "docs-shell"}>
        {!isHome ? <DocsSidebar t={t} language={language} activePage={route.id} /> : null}
        <div className="content-column">
          {!isHome ? (
            <header className="page-header">
              <p className="breadcrumbs">{t.common.breadcrumbsHome} / {t.navigation[route.id]}</p>
              <h1>{t.pages[docPage!].title}</h1>
              <p>{t.pages[docPage!].description}</p>
            </header>
          ) : null}
          {renderPage(route.id, t, language)}
        </div>
        {!isHome && toc.length ? <TableOfContents items={toc} /> : null}
      </main>
      <a className="back-to-top" href="#top">
        {t.common.top}
      </a>
      <Footer t={t} language={language} />
    </>
  );
}
