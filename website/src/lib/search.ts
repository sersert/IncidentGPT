import type { PageId } from "../data/navigation";

export type SearchEntry = {
  id: PageId;
  title: string;
  description: string;
  anchors: string[];
};

export function searchDocs(entries: SearchEntry[], query: string): SearchEntry[] {
  const normalized = query.trim().toLowerCase();
  if (!normalized) {
    return [];
  }
  return entries.filter((entry) => {
    const haystack = [entry.title, entry.description, ...entry.anchors].join(" ").toLowerCase();
    return haystack.includes(normalized);
  });
}
