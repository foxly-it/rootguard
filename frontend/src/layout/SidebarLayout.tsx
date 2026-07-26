// =====================================================
// File: frontend/src/layout/SidebarLayout.tsx
// Purpose: Layout with Sidebar Navigation
// =====================================================

import type { ReactNode } from "react";
import { NavLink } from "react-router";
import Header from "./Header";
import { useI18n } from "../i18n";

interface Props {
  children: ReactNode;
}

export default function SidebarLayout({ children }: Props) {
  const { t } = useI18n();

  return (
    <>
      <a className="rg-skip-link" href="#main-content">{t("accessibility.skipToContent")}</a>
      <Header />

      <div className="layout">

        {/* ================= Sidebar ================= */}
        <nav className="sidebar" aria-label={t("accessibility.mainNavigation")}>

          <NavLink
            to="/dashboard"
            className={({ isActive }) =>
              isActive ? "nav-item active" : "nav-item"
            }
          >
            {t("nav.overview")}
          </NavLink>

          <NavLink
            to="/setup"
            className={({ isActive }) =>
              isActive ? "nav-item active" : "nav-item"
            }
          >
            {t("nav.setup")}
          </NavLink>

          <NavLink
            to="/stack"
            className={({ isActive }) =>
              isActive ? "nav-item active" : "nav-item"
            }
          >
            {t("nav.stack")}
          </NavLink>

          <NavLink
            to="/unbound"
            className={({ isActive }) =>
              isActive ? "nav-item active" : "nav-item"
            }
          >
            {t("nav.unbound")}
          </NavLink>

          <NavLink
            to="/adguard"
            className={({ isActive }) =>
              isActive ? "nav-item active" : "nav-item"
            }
          >
            {t("nav.adguard")}
          </NavLink>

        </nav>

        {/* ================= Main ================= */}
        <main className="main" id="main-content" tabIndex={-1}>
          {children}
        </main>

      </div>
    </>
  );
}
