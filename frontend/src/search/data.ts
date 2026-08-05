// =====================================================
// File: frontend/src/search/data.ts
// Purpose: Static, local-only search index. No network
// request is made for search - every entry below is
// matched against the current locale's translated label
// plus its (language-independent) technical keywords.
//
// Extend this array as new pages, tabs, guided settings,
// or actions are added; there is no other registration
// step.
// =====================================================

export interface SearchEntry {
  id: string;
  labelKey: string;
  categoryKey: string;
  route: string;
  keywords?: string[];
}

export const searchIndex: SearchEntry[] = [
  // ---------------- Pages ----------------
  { id: "page-dashboard", labelKey: "nav.overview", categoryKey: "search.category.pages", route: "/dashboard" },
  { id: "page-setup", labelKey: "nav.setup", categoryKey: "search.category.pages", route: "/setup" },
  { id: "page-stack", labelKey: "nav.stack", categoryKey: "search.category.pages", route: "/stack" },
  { id: "page-unbound", labelKey: "nav.unbound", categoryKey: "search.category.pages", route: "/unbound" },
  { id: "page-adguard", labelKey: "nav.adguard", categoryKey: "search.category.pages", route: "/adguard" },

  // ---------------- Unbound tabs ----------------
  { id: "unbound-tab-overview", labelKey: "unbound.tab.overview", categoryKey: "search.category.unbound", route: "/unbound" },
  { id: "unbound-tab-resolver", labelKey: "unbound.tab.resolver", categoryKey: "search.category.unbound", route: "/unbound" },
  { id: "unbound-tab-zones", labelKey: "unbound.tab.zones", categoryKey: "search.category.unbound", route: "/unbound" },
  { id: "unbound-tab-advanced", labelKey: "unbound.tab.advanced", categoryKey: "search.category.unbound", route: "/unbound" },

  // ---------------- Unbound: guided resolver settings ----------------
  { id: "unbound-qname", labelKey: "unbound.qname", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["qname-minimisation", "privacy"] },
  { id: "unbound-prefetch", labelKey: "unbound.prefetch", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["prefetch"] },
  { id: "unbound-prefetch-key", labelKey: "unbound.prefetchKey", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["prefetch-key", "dnssec"] },
  { id: "unbound-aggressive-nsec", labelKey: "unbound.aggressiveNsec", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["aggressive-nsec", "nsec", "dnssec"] },
  { id: "unbound-edns-buffer", labelKey: "unbound.ednsBufferSize", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["edns-buffer-size", "1232"] },
  { id: "unbound-logging", labelKey: "unbound.operationalLogging", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["verbosity", "logging"] },
  { id: "unbound-serve-expired", labelKey: "unbound.expired", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["serve-expired", "availability"] },
  { id: "unbound-serve-expired-ttl", labelKey: "unbound.expiredTtl", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["serve-expired-ttl"] },
  { id: "unbound-serve-expired-timeout", labelKey: "unbound.expiredTimeout", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["serve-expired-client-timeout"] },
  { id: "unbound-threads", labelKey: "unbound.threads", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["num-threads"] },
  { id: "unbound-resource-profile", labelKey: "unbound.resourceProfile", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["msg-cache-size", "rrset-cache-size", "cache"] },
  { id: "unbound-network-mode", labelKey: "network.title", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["do-ip4", "do-ip6", "ipv6"] },
  { id: "unbound-presets", labelKey: "unbound.chooseProfile", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["balanced", "privacy", "performance", "high availability", "presets"] },

  // ---------------- Unbound: local DNS & forwarding ----------------
  { id: "unbound-local-zones", labelKey: "zones.title", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["local-zone", "local-data", "a record", "aaaa record", "cname"] },
  { id: "unbound-forwarding", labelKey: "forward.title", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["forward-zone", "stub-zone"] },
  { id: "unbound-private-domains", labelKey: "private.title", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["private-domain", "private-address", "rfc1918"] },
  { id: "unbound-reverse-zones", labelKey: "private.reverseTitle", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["in-addr.arpa", "ip6.arpa", "ptr"] },

  // ---------------- Unbound: advanced ----------------
  { id: "unbound-expert", labelKey: "expert.title", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["90-rootguard-custom.conf", "expert editor", "custom config"] },
  { id: "unbound-live-config", labelKey: "unbound.liveTitle", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["live configuration"] },
  { id: "unbound-history", labelKey: "unbound.history", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["rollback", "versions"] },

  // ---------------- Unbound: diagnostics ----------------
  { id: "unbound-diagnostics", labelKey: "unbound.liveDiagnostics", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["diagnostics", "dnssec check"] },
  { id: "unbound-diagnostic-logging", labelKey: "unbound.diagnosticLogging", categoryKey: "search.category.unbound", route: "/unbound", keywords: ["verbosity 2"] },

  // ---------------- Setup ----------------
  { id: "setup-network", labelKey: "setup.network", categoryKey: "search.category.setup", route: "/setup", keywords: ["bind address", "dns port", "interface"] },
  { id: "setup-channel", labelKey: "setup.channel.title", categoryKey: "search.category.setup", route: "/setup", keywords: ["stable", "beta", "release channel"] },
  { id: "setup-preflight", labelKey: "setup.runPreflight", categoryKey: "search.category.setup", route: "/setup", keywords: ["preflight"] },
  { id: "setup-deployment", labelKey: "setup.deployment", categoryKey: "search.category.setup", route: "/setup", keywords: ["install", "deploy"] },

  // ---------------- Stack & Updates ----------------
  { id: "stack-check-updates", labelKey: "stack.check", categoryKey: "search.category.stack", route: "/stack", keywords: ["update check"] },
  { id: "stack-control-plane", labelKey: "stack.controlPlaneTitle", categoryKey: "search.category.stack", route: "/stack", keywords: ["core", "webapp", "updater"] },
  { id: "stack-protection", labelKey: "stack.protection", categoryKey: "search.category.stack", route: "/stack", keywords: ["backup", "rollback"] },

  // ---------------- AdGuard ----------------
  { id: "adguard-open", labelKey: "adguard.open", categoryKey: "search.category.adguard", route: "/adguard", keywords: ["native interface"] },
  { id: "adguard-filter-check", labelKey: "adguard.filterTestTitle", categoryKey: "search.category.adguard", route: "/adguard", keywords: ["filter check", "ads", "tracking"] },
  { id: "adguard-baseline", labelKey: "adguard.bestPractices", categoryKey: "search.category.adguard", route: "/adguard", keywords: ["dns baseline"] },
];
