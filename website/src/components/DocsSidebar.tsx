import { buildRoute, routes, type PageId } from "../data/navigation";
import type { Language, Translation } from "../i18n/types";

type DocsSidebarProps = {
  t: Translation;
  language: Language;
  activePage: PageId;
};

export function DocsSidebar({ t, language, activePage }: DocsSidebarProps) {
  return (
    <aside className="docs-sidebar" aria-label="Documentation sections">
      <nav>
        {routes.map((route) => (
          <a key={route.id} href={buildRoute(language, route.path)} className={activePage === route.id ? "active" : ""}>
            {t.navigation[route.id]}
          </a>
        ))}
      </nav>
    </aside>
  );
}
