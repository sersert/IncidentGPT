import type { Language } from "../i18n/types";

export type PageId =
  | "overview"
  | "architecture"
  | "installation"
  | "configuration"
  | "webUi"
  | "logs"
  | "examples"
  | "troubleshooting"
  | "limitations";

export type Route = {
  id: PageId;
  path: string;
};

export const routes: Route[] = [
  { id: "overview", path: "" },
  { id: "architecture", path: "architecture" },
  { id: "installation", path: "installation" },
  { id: "configuration", path: "configuration" },
  { id: "webUi", path: "web-ui" },
  { id: "logs", path: "logs" },
  { id: "examples", path: "examples" },
  { id: "troubleshooting", path: "troubleshooting" },
  { id: "limitations", path: "limitations" },
];

export function buildRoute(
  language: Language,
  path: string,
  section?: string,
): string {
  return `#/${language}${path ? `/${path}` : ""}${section ? `?s=${encodeURIComponent(section)}` : ""}`;
}

export function parseRouteHash(hash: string, fallbackLanguage: Language) {
  const raw = hash.replace(/^#\/?/, "");
  const [route, query = ""] = raw.split("?", 2);
  const [first = "", ...rest] = route.split("/").filter(Boolean);
  const language: Language =
    first === "ru" || first === "en" ? first : fallbackLanguage;
  const path = first === "ru" || first === "en" ? rest.join("/") : route;
  return { language, path, section: new URLSearchParams(query).get("s") ?? "" };
}

export function getRouteByPath(path: string): Route {
  return routes.find((route) => route.path === path) ?? routes[0];
}
