import { buildRoute } from "../data/navigation";
import type { Language } from "../i18n/types";
export type TocItem = { id: string; label: string };
export function TableOfContents({
  items,
  language,
  path,
  label,
}: {
  items: TocItem[];
  language: Language;
  path: string;
  label: string;
}) {
  return (
    <nav className="toc" aria-label={label}>
      {items.map((item) => (
        <a key={item.id} href={buildRoute(language, path, item.id)}>
          {item.label}
        </a>
      ))}
    </nav>
  );
}
