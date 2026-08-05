import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { I18nContext } from "./context";
import { defaultLocale, registeredLocales } from "./registry";
import type { MessageValues } from "./types";

const storageKey = "rootguard.locale";

function initialLocale() {
  const stored = window.localStorage.getItem(storageKey);
  if (stored && registeredLocales.some((locale) => locale.code === stored)) return stored;
  const browserLocale = window.navigator.language.split("-")[0];
  return registeredLocales.some((locale) => locale.code === browserLocale) ? browserLocale : defaultLocale;
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState(initialLocale);
  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);
  const setLocale = useCallback((next: string) => {
    if (!registeredLocales.some((item) => item.code === next)) return;
    window.localStorage.setItem(storageKey, next);
    document.documentElement.lang = next;
    setLocaleState(next);
  }, []);
  const t = useCallback((key: string, values: MessageValues = {}) => {
    const selected = registeredLocales.find((item) => item.code === locale);
    const fallback = registeredLocales.find((item) => item.code === defaultLocale);
    const template = selected?.messages[key] ?? fallback?.messages[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? `{${name}}`));
  }, [locale]);
  const formatDate = useCallback((value: string | Date, options?: Intl.DateTimeFormatOptions) => (
    new Intl.DateTimeFormat(locale, options ?? { dateStyle: "short", timeStyle: "short" }).format(new Date(value))
  ), [locale]);
  const value = useMemo(() => ({ locale, locales: [...registeredLocales], setLocale, t, formatDate }), [locale, setLocale, t, formatDate]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
