import type { Language } from "../i18n/types";

export type PageId =
  | "overview"
  | "architecture"
  | "installation"
  | "configuration"
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
  { id: "examples", path: "examples" },
  { id: "troubleshooting", path: "troubleshooting" },
  { id: "limitations", path: "limitations" },
];

export function buildRoute(language: Language, path: string): string {
  return `#/${language}${path ? `/${path}` : ""}`;
}

export function getRouteByPath(path: string): Route {
  return routes.find((route) => route.path === path) ?? routes[0];
}
