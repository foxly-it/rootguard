import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router";
import { ArrowRight, Check, CheckCircle2, CircleAlert, ExternalLink, Filter, RefreshCw, ShieldCheck } from "lucide-react";
import {
  bootstrapAdGuard,
  fetchAdGuardFilterReport,
  fetchAdGuardStatus,
  fetchInstallationStatus,
  setAdGuardFiltering,
  setAdGuardProtection,
  type AdGuardFilterCheck,
  type AdGuardFilterReport,
  type AdGuardStatus,
  type InstallationStatus,
} from "../api/client";
import ContentModal from "../components/ContentModal";
import "../styles/adguard.css";
import { useI18n } from "../i18n";
import { errorMessage } from "../utils/errors";
import { formatCountdown } from "../utils/countdown";

export default function AdGuard() {
  const { t } = useI18n();
  const [status, setStatus] = useState<AdGuardStatus | null>(null);
  const [installation, setInstallation] = useState<InstallationStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [bootstrapping, setBootstrapping] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [filterReport, setFilterReport] = useState<AdGuardFilterReport | null>(null);
  const [testingFilters, setTestingFilters] = useState(false);
  const [filterModalOpen, setFilterModalOpen] = useState(false);
  const [filterError, setFilterError] = useState("");
  const [filteringBusy, setFilteringBusy] = useState(false);
  const [protectionBusy, setProtectionBusy] = useState(false);
  const [protectionChoice, setProtectionChoice] = useState("");
  // When `status` was last actually fetched/updated - paired with
  // status.protection_disabled_duration_ms (AdGuard's own remaining-pause
  // figure at that moment) to compute a live countdown without re-fetching
  // every second. `now` is ticked from a 1s interval rather than read via
  // Date.now() directly in render, matching the Sparkline hover-age pattern
  // elsewhere in this codebase (react-hooks/purity rejects impure reads
  // during render).
  const [statusFetchedAt, setStatusFetchedAt] = useState<number | null>(null);
  const [now, setNow] = useState(0);

  const applyStatus = useCallback((next: AdGuardStatus | null) => {
    setStatus(next);
    setStatusFetchedAt(Date.now());
  }, []);

  const load = useCallback(async () => {
    setError("");
    try {
      const currentInstallation = await fetchInstallationStatus();
      setInstallation(currentInstallation);
      if (currentInstallation.state === "installed") {
        applyStatus(await fetchAdGuardStatus());
      } else {
        applyStatus(null);
      }
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.statusLoadError")));
    } finally {
      setLoading(false);
    }
  }, [t, applyStatus]);

  useEffect(() => {
    const initialLoad = window.setTimeout(load, 0);
    return () => window.clearTimeout(initialLoad);
  }, [load]);

  // AdGuard re-enables protection itself once a timed pause elapses (see
  // changeProtection) - without polling here, RootGuard would keep showing
  // "paused" until the page was manually reloaded. Only runs while actually
  // paused, so it doesn't add load the rest of the time.
  useEffect(() => {
    if (!status || status.protection_enabled) return;
    const interval = window.setInterval(() => {
      fetchAdGuardStatus().then(applyStatus).catch(() => {});
    }, 5000);
    return () => window.clearInterval(interval);
  }, [status, applyStatus]);

  // Drives the visible countdown between polls above - only ticks while a
  // *timed* pause (not an indefinite one) is showing.
  useEffect(() => {
    if (!status || status.protection_enabled || status.protection_disabled_duration_ms <= 0) return;
    setNow(Date.now());
    const interval = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(interval);
  }, [status]);

  const remainingMs = status && !status.protection_enabled && status.protection_disabled_duration_ms > 0 && statusFetchedAt !== null && now > 0
    ? Math.max(0, status.protection_disabled_duration_ms - (now - statusFetchedAt))
    : null;

  async function initialize() {
    if (bootstrapping) return;
    setBootstrapping(true);
    setMessage("");
    setError("");
    try {
      const updated = await bootstrapAdGuard();
      applyStatus(updated);
      setMessage(t("adguard.bootstrapComplete"));
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.bootstrapError")));
    } finally {
      setBootstrapping(false);
    }
  }

  async function testFilters() {
    if (testingFilters) return;
    setTestingFilters(true);
    setFilterError("");
    try {
      setFilterReport(await fetchAdGuardFilterReport());
    } catch (cause) {
      setFilterError(errorMessage(cause, t("adguard.filterTestError")));
    } finally {
      setTestingFilters(false);
    }
  }

  function openFilterTest() {
    setFilterModalOpen(true);
    void testFilters();
  }

  async function toggleFiltering() {
    if (filteringBusy || !status) return;
    setFilteringBusy(true);
    setError("");
    try {
      applyStatus(await setAdGuardFiltering(!status.filtering_enabled));
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.filteringToggleError")));
    } finally {
      setFilteringBusy(false);
    }
  }

  // Mirrors AdGuard Home's own "Protection" dropdown (Off/10 minutes/1
  // hour) - unlike filtering above, AdGuard itself re-enables protection
  // after the chosen duration, no RootGuard-side scheduling needed. The
  // select is an action trigger, not a state display (see protectionChoice
  // reset below) - found via code review: binding its value straight to
  // protection_enabled made a 10-minute pause look identical to "off
  // indefinitely" the instant it was chosen, since both just set
  // protection_enabled to false. The actual state is shown separately
  // (protectionStatusLabel below).
  async function changeProtection(choice: "on" | "off" | "10m" | "1h") {
    setProtectionChoice("");
    if (protectionBusy || !status) return;
    setProtectionBusy(true);
    setError("");
    try {
      const durations: Record<typeof choice, number> = { on: 0, off: 0, "10m": 600, "1h": 3600 };
      applyStatus(await setAdGuardProtection(choice === "on", durations[choice]));
    } catch (cause) {
      setError(errorMessage(cause, t("adguard.protectionToggleError")));
    } finally {
      setProtectionBusy(false);
    }
  }

  // reachable: AdGuard is configured and answering - independent of whether
  // protection/filtering happen to be paused right now. Gates things that
  // stay usable during a pause (opening the native UI, running a filter
  // test, the pause control itself - it would be self-defeating if pausing
  // protection also hid the control needed to un-pause it).
  const reachable = status?.configured && status.healthy && status.upstream_ready;
  // ready: reachable AND actually filtering traffic right now. Found via
  // code review: "PROTECTED"/the STATUS badge previously ignored
  // protection_enabled entirely, so pausing protection for a client still
  // showed fully green everywhere. Matches Overview.tsx's protectedState.
  const ready = reachable && status?.protection_enabled === true && status?.filtering_enabled === true;

  const protectionStatusLabel = !status
    ? ""
    : status.protection_enabled
      ? t("adguard.protectionStatusActive")
      : remainingMs !== null
        ? t("adguard.protectionStatusPausedFor", { time: formatCountdown(remainingMs) })
        : t("adguard.protectionStatusPausedIndefinite");

  return (
    <div className="adguard-page">
      <section className={`adguard-hero ${ready ? "ready" : ""}`}>
        <div>
          <span className="adguard-eyebrow">MANAGED DNS FILTER</span>
          <h1>AdGuard Home</h1>
          <p>{t("adguard.intro")}</p>
          {installation?.state !== "installed" && (
            <Link className="rg-button rg-button-primary adguard-primary-action" to="/setup">
              {t("adguard.setup")} <ArrowRight size={16} />
            </Link>
          )}
          {reachable && (
            <a className="rg-button rg-button-primary adguard-primary-action" href="/adguard-ui/" target="_blank" rel="noreferrer">
              {t("adguard.open")} <ExternalLink size={16} />
            </a>
          )}
        </div>
        <div className="adguard-shield">
          <ShieldCheck size={46} />
          <strong>{ready ? t("adguard.protected") : loading ? t("adguard.check") : t("adguard.setupState")}</strong>
          <small>{ready ? t("adguard.readyText") : t("adguard.managedText")}</small>
        </div>
      </section>

      {message && <div className="adguard-feedback success">{message}</div>}
      {error && <div className="adguard-feedback error" role="alert">{error}</div>}

      <div className="adguard-grid">
        <section className="adguard-panel">
          <div className="adguard-panel-heading">
            <div><span className="adguard-eyebrow">STATUS</span><h2>{t("adguard.secureSetup")}</h2></div>
            <span className={`adguard-state ${ready ? "healthy" : ""}`}>{loading ? t("common.checking") : ready ? t("adguard.ready") : t("adguard.incomplete")}</span>
          </div>
          <div className="adguard-status-list">
            <StatusRow label={t("adguard.config")} active={Boolean(status?.configured)} activeText={t("adguard.managed")} inactiveText={t("adguard.notSetup")} adguardHash="#settings" adguardHashLabel={t("adguard.openSettingsShort")} adguardHashAriaLabel={t("adguard.openSettings")} />
            <StatusRow label={t("adguard.bestPractices")} active={Boolean(status?.best_practices_ready)} activeText={t("adguard.bestPracticesActive")} inactiveText={t("adguard.bestPracticesPending")} adguardHash="#dns" adguardHashLabel={t("adguard.openDnsSettingsShort")} adguardHashAriaLabel={t("adguard.openDnsSettings")} />
            <StatusRow
              label={t("adguard.filterLists")}
              active={Boolean(status?.filtering_enabled)}
              activeText={t("adguard.filterListsActive", { active: status?.active_filter_lists ?? 0, total: status?.total_filter_lists ?? 0 })}
              inactiveText={t("adguard.filterListsInactive")}
              adguardHash="#filters"
              adguardHashLabel={t("adguard.openFilterListsShort")}
              adguardHashAriaLabel={t("adguard.openFilterLists")}
            />
          </div>
          <div className="adguard-upstream">
            <span>{t("adguard.activeUpstream")}</span>
            <code>{status?.upstream || "172.29.53.2:5335"}</code>
          </div>
          {status?.version && (
            <div className="adguard-upstream">
              <span>{t("adguard.version")}</span>
              <code>{status.version}</code>
            </div>
          )}
          {!loading && installation?.state === "installed" && (!status?.configured || !status?.best_practices_ready) && (
            <button className="rg-button rg-button-primary adguard-primary-action" type="button" disabled={bootstrapping} onClick={initialize}>
              {bootstrapping
                ? t("adguard.settingUp")
                : status?.configured
                  ? t("adguard.applyBestPractices")
                  : t("adguard.finish")}
            </button>
          )}
        </section>

        {reachable ? (
          <section className="adguard-panel adguard-filter-launcher">
            <div className="adguard-filter-launcher-icon"><Filter size={22} /></div>
            <div>
              <span className="adguard-eyebrow">{t("adguard.filterTestEyebrow")}</span>
              <h2>{t("adguard.filterTestTitle")}</h2>
              <p>{t("adguard.filterTestHelp")}</p>
            </div>
            <button className="rg-button rg-button-primary" type="button" disabled={testingFilters} onClick={openFilterTest}>
              <RefreshCw size={16} /> {t("adguard.filterTestRun")}
            </button>
            {status && (
              <>
                <label className="adguard-filtering-toggle">
                  <div>
                    <strong>{t("adguard.filteringToggleLabel")}</strong>
                    <small>{t("adguard.filteringToggleHelp")}</small>
                  </div>
                  <input type="checkbox" checked={status.filtering_enabled} disabled={filteringBusy} onChange={() => void toggleFiltering()} aria-label={t("adguard.filteringToggleLabel")} />
                </label>
                <div className="adguard-protection-control">
                  <div>
                    <strong>{t("adguard.protectionLabel")}</strong>
                    <small className={status.protection_enabled ? "" : "paused"}>{protectionStatusLabel}</small>
                  </div>
                  <select
                    value={protectionChoice}
                    disabled={protectionBusy}
                    onChange={(event) => void changeProtection(event.target.value as "on" | "off" | "10m" | "1h")}
                    aria-label={t("adguard.protectionLabel")}
                  >
                    <option value="" disabled>{t("adguard.protectionChooseAction")}</option>
                    {!status.protection_enabled && <option value="on">{t("adguard.protectionOn")}</option>}
                    <option value="off">{t("adguard.protectionOffIndefinite")}</option>
                    <option value="10m">{t("adguard.protectionOff10m")}</option>
                    <option value="1h">{t("adguard.protectionOff1h")}</option>
                  </select>
                </div>
              </>
            )}
          </section>
        ) : (
          <section className="adguard-panel adguard-filter-launcher unavailable">
            <div className="adguard-filter-launcher-icon"><Filter size={22} /></div>
            <div>
              <span className="adguard-eyebrow">{t("adguard.filterTestEyebrow")}</span>
              <h2>{t("adguard.filterTestTitle")}</h2>
              <p>{t("adguard.filterTestHelp")}</p>
            </div>
          </section>
        )}
      </div>

      <ContentModal open={filterModalOpen} size="medium" title={t("adguard.filterTestTitle")} eyebrow={t("adguard.filterTestEyebrow")} closeLabel={t("common.close")} onClose={() => setFilterModalOpen(false)}>
        <div className="adguard-filter-modal">
          <div className="adguard-filter-modal-intro">
            <p>{t("adguard.filterTestHelp")}</p>
            <button className="rg-button rg-button-secondary" type="button" disabled={testingFilters} onClick={() => void testFilters()}>
              <RefreshCw size={16} className={testingFilters ? "spin" : ""} />
              {testingFilters ? t("adguard.filterTesting") : t("adguard.filterTestRun")}
            </button>
          </div>
          {filterError && <div className="adguard-feedback error" role="alert">{filterError}</div>}
          {testingFilters && !filterReport && <div className="adguard-filter-loading"><RefreshCw size={22} className="spin" /><span>{t("adguard.filterTesting")}</span></div>}
          {filterReport && (
            <>
              <div className={`adguard-filter-summary ${filterReport.passed === filterReport.expected ? "healthy" : "warning"}`}>
                <strong>{t("adguard.filterSummary", { passed: filterReport.passed, expected: filterReport.expected })}</strong>
                <span>{t("adguard.filterSummaryHelp", { blocked: filterReport.blocked })}</span>
              </div>
              <div className="adguard-filter-grid">
                {filterReport.checks.map((check) => <FilterCheck key={check.host} check={check} />)}
              </div>
              <small className="adguard-filter-note">{t("adguard.filterTestNote")}</small>
              <a className="adguard-manage-filters-link" href="/adguard-ui/#filters" target="_blank" rel="noreferrer">
                {t("adguard.manageFilters")} <ExternalLink size={14} aria-hidden="true" />
              </a>
            </>
          )}
        </div>
      </ContentModal>
    </div>
  );
}

function FilterCheck({ check }: { check: AdGuardFilterCheck }) {
  const { t } = useI18n();
  const passed = !check.expected_blocked || check.blocked;
  return (
    <article className={passed ? "passed" : "missed"}>
      <span>{passed ? <CheckCircle2 size={18} /> : <CircleAlert size={18} />}</span>
      <div>
        <strong>{check.host}</strong>
        <small>{categoryLabel(t, check.category)}</small>
        <p>
          {check.blocked
            ? t("adguard.filterBlocked")
            : check.expected_blocked
              ? t("adguard.filterNotBlocked")
              : t("adguard.filterInformational")}
        </p>
        {check.matched_rule && <code>{check.matched_rule}</code>}
      </div>
    </article>
  );
}

function categoryLabel(t: (key: string) => string, category: AdGuardFilterCheck["category"]) {
  const keys = {
    advertising: "adguard.categoryAdvertising",
    tracking: "adguard.categoryTracking",
    service: "adguard.categoryService",
    telemetry: "adguard.categoryTelemetry",
    "security-test": "adguard.categorySecurityTest",
  };
  return t(keys[category]);
}

function StatusRow({ label, active, activeText, inactiveText, adguardHash, adguardHashLabel, adguardHashAriaLabel }: {
  label: string;
  active: boolean;
  activeText: string;
  inactiveText: string;
  // Deep-links straight into the relevant native AdGuard Home page through
  // the existing protected proxy - never a raw admin-port URL - so an
  // operator who spots something to fix doesn't have to hunt for it inside
  // AdGuard's own navigation first.
  adguardHash?: string;
  adguardHashLabel?: string;
  adguardHashAriaLabel?: string;
}) {
  return (
    <div>
      <span className={active ? "status-check active" : "status-check"}>{active ? <Check size={14} /> : "!"}</span>
      <strong>{label}</strong>
      <small>{active ? activeText : inactiveText}</small>
      {adguardHash && (
        <a className="adguard-contextual-button" href={`/adguard-ui/${adguardHash}`} target="_blank" rel="noreferrer" aria-label={adguardHashAriaLabel} title={adguardHashAriaLabel}>
          <span>{adguardHashLabel}</span> <ExternalLink size={12} aria-hidden="true" />
        </a>
      )}
    </div>
  );
}

