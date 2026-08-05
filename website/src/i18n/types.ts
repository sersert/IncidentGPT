export type Language = "ru" | "en";
export type ThemeMode = "system" | "dark" | "light";

export type Translation = {
  meta: {
    title: string;
    description: string;
  };
  navigation: Record<"overview" | "architecture" | "installation" | "configuration" | "examples" | "troubleshooting" | "limitations", string>;
  common: {
    copy: string;
    copied: string;
    download: string;
    reset: string;
    top: string;
    github: string;
    search: string;
    noResults: string;
    breadcrumbsHome: string;
    language: string;
    theme: string;
    dark: string;
    light: string;
    system: string;
    recommended: string;
    currentBehavior: string;
    potentialRisk: string;
  };
  hero: {
    title: string;
    description: string;
    installButton: string;
    githubButton: string;
    architectureLink: string;
    demoNote: string;
  };
  home: {
    terminalTitle: string;
    resultTitle: string;
    featuresTitle: string;
    compareTitle: string;
    regularApproach: string;
    incidentgptApproach: string;
    compareConclusion: string;
    reliabilityTitle: string;
    reliabilityBody: string;
    quickStartTitle: string;
    humanNote: string;
  };
  pages: Record<"architecture" | "installation" | "configuration" | "examples" | "troubleshooting" | "limitations", {
    title: string;
    description: string;
  }>;
  generator: {
    title: string;
    description: string;
    aiWorker: string;
    enricher: string;
    alertmanager: string;
    fields: Record<keyof import("../lib/generator").GeneratorValues, string>;
  };
};
