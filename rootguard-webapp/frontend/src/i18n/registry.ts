import { de } from "./de";
import { en } from "./en";
import type { LocaleDefinition } from "./types";

export const defaultLocale = "de";
export const registeredLocales: LocaleDefinition[] = [de, en];

// Public extension point for future locale packages.
export function registerLocale(locale: LocaleDefinition) {
  const existing = registeredLocales.findIndex((item) => item.code === locale.code);
  if (existing >= 0) registeredLocales[existing] = locale;
  else registeredLocales.push(locale);
}
