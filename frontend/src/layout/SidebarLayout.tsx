// =====================================================
// File: frontend/src/layout/SidebarLayout.tsx
// Purpose: Layout with Sidebar Navigation
// =====================================================

import { useEffect, useState, type FocusEvent, type MouseEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { NavLink } from "react-router";
import {
  Boxes,
  Filter,
  LayoutDashboard,
  PanelLeftClose,
  PanelLeftOpen,
  ServerCog,
  ShieldCheck,
} from "lucide-react";
import Header from "./Header";
import { useI18n } from "../i18n";

interface Props {
  children: ReactNode;
}

export default function SidebarLayout({ children }: Props) {
  const { t } = useI18n();
  const [collapsed, setCollapsed] = useState(() => {
    const stored = window.localStorage.getItem("rootguard.sidebar.collapsed");
    if (stored !== null) return stored === "true";
    // No explicit preference yet: default new desktop sessions to the
    // collapsed icon view. Mobile ignores this value (the toggle is
    // hidden and labels always show below the layout breakpoint).
    return window.innerWidth >= 760;
  });
  const [tooltip, setTooltip] = useState<{ label: string; top: number; left: number } | null>(null);

  useEffect(() => {
    if (!collapsed) setTooltip(null);
  }, [collapsed]);

  function showTooltip(event: MouseEvent<HTMLElement> | FocusEvent<HTMLElement>, label: string) {
    if (!collapsed || window.innerWidth < 760) return;
    const rect = event.currentTarget.getBoundingClientRect();
    setTooltip({ label, top: rect.top + rect.height / 2, left: rect.right + 12 });
  }

  function hideTooltip() {
    setTooltip(null);
  }

  const navigation = [
    { to: "/dashboard", label: t("nav.overview"), icon: <LayoutDashboard aria-hidden="true" /> },
    { to: "/setup", label: t("nav.setup"), icon: <ServerCog aria-hidden="true" /> },
    { to: "/stack", label: t("nav.stack"), icon: <Boxes aria-hidden="true" /> },
    { to: "/unbound", label: t("nav.unbound"), icon: <ShieldCheck aria-hidden="true" /> },
    { to: "/adguard", label: t("nav.adguard"), icon: <Filter aria-hidden="true" /> },
  ];

  function toggleSidebar() {
    setCollapsed((current) => {
      const next = !current;
      window.localStorage.setItem("rootguard.sidebar.collapsed", String(next));
      return next;
    });
  }

  return (
    <div className="app-shell">
      <a className="rg-skip-link" href="#main-content">{t("accessibility.skipToContent")}</a>
      <Header />

      <div className={`layout${collapsed ? " sidebar-collapsed" : ""}`}>

        {/* ================= Sidebar ================= */}
        {/* Fixed height (see .app-shell/.layout): only .sidebar-nav
            scrolls internally if the item list ever outgrows the
            viewport, so the collapse control at the bottom always
            stays reachable without scrolling the page. */}
        <nav className="sidebar" aria-label={t("accessibility.mainNavigation")}>
          <div className="sidebar-nav">
            {navigation.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) => isActive ? "nav-item active" : "nav-item"}
                aria-label={collapsed ? item.label : undefined}
                onMouseEnter={(event) => showTooltip(event, item.label)}
                onMouseLeave={hideTooltip}
                onFocus={(event) => showTooltip(event, item.label)}
                onBlur={hideTooltip}
              >
                <span className="nav-item-icon">{item.icon}</span>
                <span className="nav-item-label">{item.label}</span>
              </NavLink>
            ))}
          </div>

          <button
            className="sidebar-toggle"
            type="button"
            onClick={toggleSidebar}
            aria-expanded={!collapsed}
            aria-label={collapsed ? t("nav.expandSidebar") : t("nav.collapseSidebar")}
            title={collapsed ? t("nav.expandSidebar") : t("nav.collapseSidebar")}
          >
            {collapsed ? <PanelLeftOpen aria-hidden="true" /> : <PanelLeftClose aria-hidden="true" />}
            <span>{t("nav.collapseSidebar")}</span>
          </button>
        </nav>

        {tooltip && createPortal(
          <div className="rg-nav-tooltip" role="presentation" style={{ top: tooltip.top, left: tooltip.left }}>
            {tooltip.label}
          </div>,
          document.body,
        )}

        {/* ================= Main ================= */}
        <main className="main" id="main-content" tabIndex={-1}>
          {children}
        </main>

      </div>
    </div>
  );
}
