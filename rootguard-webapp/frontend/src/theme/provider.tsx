import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { ThemeContext, type ThemeMode, type ResolvedTheme } from "./context";

const storageKey = "rootguard.theme";
const media = () => window.matchMedia("(prefers-color-scheme: light)");

function initialMode(): ThemeMode {
  const stored = window.localStorage.getItem(storageKey);
  return stored === "light" || stored === "dark" ? stored : "system";
}

function resolve(mode: ThemeMode): ResolvedTheme {
  if (mode === "system") return media().matches ? "light" : "dark";
  return mode;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(initialMode);
  const [resolvedTheme, setResolvedTheme] = useState<ResolvedTheme>(() => resolve(mode));

  useEffect(() => {
    const next = resolve(mode);
    setResolvedTheme(next);
    if (next === "light") document.documentElement.dataset.theme = "light";
    else delete document.documentElement.dataset.theme;

    if (mode !== "system") return;
    const query = media();
    const onChange = () => {
      const nextResolved = query.matches ? "light" : "dark";
      setResolvedTheme(nextResolved);
      if (nextResolved === "light") document.documentElement.dataset.theme = "light";
      else delete document.documentElement.dataset.theme;
    };
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, [mode]);

  const setMode = useCallback((next: ThemeMode) => {
    if (next === "system") window.localStorage.removeItem(storageKey);
    else window.localStorage.setItem(storageKey, next);
    setModeState(next);
  }, []);

  const value = useMemo(() => ({ mode, resolvedTheme, setMode }), [mode, resolvedTheme, setMode]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
