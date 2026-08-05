// =====================================================
// File: frontend/src/layout/Header.tsx
// Project: RootGuard WebApp
// Purpose: Global application header
//
// Behaviour:
// - Version wird immer angezeigt (auch "dev")
// - Commit wird nur angezeigt, wenn != "unknown"
// - Kein "vdev · unknown"
// - GitHub + Docs als Icon + Text
// - Icons folgen currentColor (Theme-ready)
// =====================================================

import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router";
import { getVersion } from "../services/api";

// RootGuard shield icon (SVG, icon-only)
import logo from "../assets/rootguard-icon.svg";

// Inline SVG icon components
import GithubIcon from "../components/icons/GithubIcon";
import DocsIcon from "../components/icons/DocsIcon";
import { useI18n } from "../i18n";
import UserMenu from "./UserMenu";
import SearchModal from "../search/SearchModal";

interface VersionData {
  version: string;
  commit: string;
}

export default function Header() {
  const [version, setVersion] = useState<VersionData | null>(null);
  const { t } = useI18n();
  const location = useLocation();
  const pageName = pageTitle(location.pathname, t);

  // -----------------------------------------------------
  // Fetch backend version metadata on mount
  // -----------------------------------------------------
  useEffect(() => {
    getVersion()
      .then(setVersion)
      .catch(() => setVersion(null));
  }, []);

  // -----------------------------------------------------
  // Format version string cleanly
  // - Always show version
  // - Only append commit if it is valid
  // -----------------------------------------------------
  function formatVersion(v: VersionData | null): string | null {
    if (!v) return null;

    if (v.commit && v.commit !== "unknown") {
      return `v${v.version} · ${v.commit.substring(0, 7)}`;
    }

    return `v${v.version}`;
  }

  return (
    <header className="rg-header">
      {/* =====================================================
          Left: Branding
         ===================================================== */}
      <div className="rg-header-left">
        <Link className="rg-brand" to="/dashboard" aria-label="RootGuard Dashboard">
        <img
          src={logo}
          alt=""
          className="rg-logo"
        />
          <span className="rg-title">Root<span>Guard</span></span>
        </Link>
        <span className="rg-title-divider" aria-hidden="true" />
        <span className="rg-page-name">{pageName}</span>
      </div>

      {/* =====================================================
          Right: Version + Utility Links
         ===================================================== */}
      <div className="rg-header-right">

        <SearchModal />

        {version && (
          <span className="rg-version">
            {formatVersion(version)}
          </span>
        )}

        <a
          href="https://github.com/foxly-it/rootguard-webapp"
          target="_blank"
          rel="noreferrer"
          className="rg-link rg-link-desktop-only"
          aria-label={t("header.github")}
        >
          <GithubIcon />
          <span>GitHub</span>
        </a>

        <a
          href="https://rootguard.foxly.de/docs.html"
          target="_blank"
          rel="noreferrer"
          className="rg-link rg-link-desktop-only"
          aria-label={t("header.docs")}
        >
          <DocsIcon />
          <span>Docs</span>
        </a>

        <UserMenu />
      </div>
    </header>
  );
}

function pageTitle(pathname: string, t: (key: string) => string) {
  if (pathname.startsWith("/setup")) return t("nav.setup");
  if (pathname.startsWith("/stack")) return t("nav.stack");
  if (pathname.startsWith("/unbound")) return t("nav.unbound");
  if (pathname.startsWith("/adguard")) return t("nav.adguard");
  return t("nav.overview");
}
