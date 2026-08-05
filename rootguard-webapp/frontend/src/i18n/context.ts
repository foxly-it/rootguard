import { createContext, useContext } from "react";
import type { LocaleDefinition, MessageValues } from "./types";

export interface I18nValue {
  locale: string;
  locales: LocaleDefinition[];
  setLocale: (locale: string) => void;
  t: (key: string, values?: MessageValues) => string;
  formatDate: (value: string | Date, options?: Intl.DateTimeFormatOptions) => string;
}

export const I18nContext = createContext<I18nValue | null>(null);

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) throw new Error("useI18n must be used inside I18nProvider");
  return context;
}
