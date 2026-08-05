// =====================================================
// File: frontend/src/layout/UserMenu.tsx
// Project: RootGuard WebApp
// Purpose: Consolidated account menu (language, appearance,
// sign-out); on narrow viewports also carries Docs/GitHub.
// =====================================================

import { useEffect, useRef, useState } from "react";
import { ChevronDown, Monitor, Sun, Moon, LogOut, User } from "lucide-react";
import { useI18n } from "../i18n";
import { useAuth } from "../auth";
import { useTheme, type ThemeMode } from "../theme";
import GithubIcon from "../components/icons/GithubIcon";
import DocsIcon from "../components/icons/DocsIcon";

const themeOrder: ThemeMode[] = ["system", "light", "dark"];
const themeIcon: Record<ThemeMode, typeof Monitor> = { system: Monitor, light: Sun, dark: Moon };

export default function UserMenu() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const firstItemRef = useRef<HTMLButtonElement>(null);

  const { locale, locales, setLocale, t } = useI18n();
  const { mode, setMode } = useTheme();
  const { username, logout } = useAuth();

  useEffect(() => {
    if (!open) return;
    firstItemRef.current?.focus();

    function onPointerDown(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    }
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div className="rg-user-menu" ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className="rg-user-trigger"
        aria-haspopup="true"
        aria-expanded={open}
        aria-label={t("header.accountMenu", { username })}
        onClick={() => setOpen((value) => !value)}
      >
        <User aria-hidden="true" />
        <span>{username}</span>
        <ChevronDown aria-hidden="true" className="rg-user-chevron" />
      </button>

      {open && (
        <div className="rg-user-panel">
          <div className="rg-user-panel-section" role="group" aria-label={t("header.appearance")}>
            <span className="rg-user-panel-label">{t("header.appearance")}</span>
            <div className="rg-user-theme-options">
              {themeOrder.map((option, index) => {
                const Icon = themeIcon[option];
                return (
                  <button
                    key={option}
                    ref={index === 0 ? firstItemRef : undefined}
                    type="button"
                    className="rg-user-theme-option"
                    aria-pressed={mode === option}
                    onClick={() => setMode(option)}
                  >
                    <Icon aria-hidden="true" />
                    {t(`theme.${option}`)}
                  </button>
                );
              })}
            </div>
          </div>

          <label className="rg-user-panel-section rg-user-language">
            <span className="rg-user-panel-label">{t("language.label")}</span>
            <select value={locale} onChange={(event) => setLocale(event.target.value)} aria-label={t("language.label")}>
              {locales.map((item) => <option value={item.code} key={item.code}>{item.label}</option>)}
            </select>
          </label>

          <div className="rg-user-panel-section rg-user-panel-mobile-links">
            <a href="https://github.com/foxly-it/rootguard-webapp" target="_blank" rel="noreferrer" className="rg-user-panel-link">
              <GithubIcon />
              <span>GitHub</span>
            </a>
            <a href="https://rootguard.foxly.de/docs.html" target="_blank" rel="noreferrer" className="rg-user-panel-link">
              <DocsIcon />
              <span>Docs</span>
            </a>
          </div>

          <button type="button" className="rg-user-panel-signout" onClick={() => void logout()}>
            <LogOut aria-hidden="true" />
            {t("login.signOut")}
          </button>
        </div>
      )}
    </div>
  );
}
