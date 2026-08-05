import { en } from "./en";
import { ru } from "./ru";
import type { Language, Translation } from "./types";

export const translations: Record<Language, Translation> = { ru, en };

export function detectInitialLanguage(): Language {
  const stored = localStorage.getItem("incidentgpt-language");
  if (stored === "ru" || stored === "en") {
    return stored;
  }
  return navigator.language.toLowerCase().startsWith("ru") ? "ru" : "en";
}
