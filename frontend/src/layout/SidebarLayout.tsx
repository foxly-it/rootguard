// =====================================================
// File: frontend/src/layout/SidebarLayout.tsx
// Purpose: Layout with Sidebar Navigation
// =====================================================

import { useState, type ReactNode } from "react";
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
    <>
      <a className="rg-skip-link" href="#main-content">{t("accessibility.skipToContent")}</a>
      <Header />

      <div className={`layout${collapsed ? " sidebar-collapsed" : ""}`}>

        {/* ================= Sidebar ================= */}
        <nav className="sidebar" aria-label={t("accessibility.mainNavigation")}>
          {navigation.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => isActive ? "nav-item active" : "nav-item"}
              aria-label={collapsed ? item.label : undefined}
              data-tooltip={collapsed ? item.label : undefined}
            >
              <span className="nav-item-icon">{item.icon}</span>
              <span className="nav-item-label">{item.label}</span>
            </NavLink>
          ))}

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

        {/* ================= Main ================= */}
        <main className="main" id="main-content" tabIndex={-1}>
          {children}
        </main>

      </div>
    </>
  );
}
