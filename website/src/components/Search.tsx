import { SearchIcon } from "lucide-react";
import { useMemo, useState } from "react";
import { routes } from "../data/navigation";
import type { Language, Translation } from "../i18n/types";
import { buildRoute } from "../data/navigation";
import { searchDocs, type SearchEntry } from "../lib/search";

type SearchProps = {
  t: Translation;
  language: Language;
};

export function Search({ t, language }: SearchProps) {
  const [query, setQuery] = useState("");
  const entries = useMemo<SearchEntry[]>(
    () =>
      routes.map((route) => ({
        id: route.id,
        title: t.navigation[route.id],
        description: route.id === "overview" ? t.hero.description : t.pages[route.id].description,
        anchors: ["Prometheus", "Kubernetes", "Telegram", "OpenRouter", "Redis", "Alertmanager"],
      })),
    [t],
  );
  const results = searchDocs(entries, query);

  return (
    <div className="search">
      <label>
        <SearchIcon size={15} aria-hidden="true" />
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t.common.search} />
      </label>
      {query.trim() ? (
        <div className="search-results">
          {results.length ? (
            results.map((entry) => {
              const route = routes.find((item) => item.id === entry.id) ?? routes[0];
              return (
                <a key={entry.id} href={buildRoute(language, route.path)} onClick={() => setQuery("")}>
                  <strong>{entry.title}</strong>
                  <span>{entry.description}</span>
                </a>
              );
            })
          ) : (
            <span className="empty">{t.common.noResults}</span>
          )}
        </div>
      ) : null}
    </div>
  );
}
