import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router";
import { ArrowRight, Check, CheckCircle2, CircleAlert, ExternalLink, Filter, KeyRound, LockKeyhole, Network, RefreshCw, ShieldCheck } from "lucide-react";
import {
  bootstrapAdGuard,
  fetchAdGuardFilterReport,
  fetchAdGuardStatus,
  fetchInstallationStatus,
  type AdGuardFilterCheck,
  type AdGuardFilterReport,
  type AdGuardStatus,
  type InstallationStatus,
} from "../api/client";
import ContentModal from "../components/ContentModal";
import "../styles/adguard.css";
import { useI18n } from "../i18n";

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

  const load = useCallback(async () => {
    setError("");
    try {
      const currentInstallation = await fetchInstallationStatus();
      setInstallation(currentInstallation);
      if (currentInstallation.state === "installed") {
        setStatus(await fetchAdGuardStatus());
      } else {
        setStatus(null);
      }
    } catch (cause) {
      setError(errorMessage(cause, "AdGuard-Status konnte nicht geladen werden."));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const initialLoad = window.setTimeout(load, 0);
    return () => window.clearTimeout(initialLoad);
  }, [load]);

  async function initialize() {
    if (bootstrapping) return;
    setBootstrapping(true);
    setMessage("");
    setError("");
    try {
      const updated = await bootstrapAdGuard();
      setStatus(updated);
      setMessage(t("adguard.bootstrapComplete"));
    } catch (cause) {
      setError(errorMessage(cause, "AdGuard Home konnte nicht eingerichtet werden."));
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

  const ready = status?.configured && status.healthy && status.upstream_ready;

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
          {ready && (
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
      {error && <div className="adguard-feedback error">{error}</div>}

      <div className="adguard-grid">
        <section className="adguard-panel">
          <div className="adguard-panel-heading">
            <div><span className="adguard-eyebrow">STATUS</span><h2>{t("adguard.secureSetup")}</h2></div>
            <span className={`adguard-state ${ready ? "healthy" : ""}`}>{loading ? t("common.checking") : ready ? t("adguard.ready") : t("adguard.incomplete")}</span>
          </div>
          <div className="adguard-status-list">
            <StatusRow label={t("adguard.config")} active={Boolean(status?.configured)} activeText={t("adguard.managed")} inactiveText={t("adguard.notSetup")} adguardHash="#settings" adguardHashLabel={t("adguard.openSettings")} />
            <StatusRow label={t("adguard.service")} active={Boolean(status?.healthy)} activeText={t("adguard.reachable")} inactiveText={t("adguard.unreachable")} />
            <StatusRow label={t("adguard.upstream")} active={Boolean(status?.upstream_ready)} activeText={t("adguard.validated")} inactiveText={t("adguard.pending")} />
            <StatusRow label={t("adguard.bestPractices")} active={Boolean(status?.best_practices_ready)} activeText={t("adguard.bestPracticesActive")} inactiveText={t("adguard.bestPracticesPending")} adguardHash="#dns" adguardHashLabel={t("adguard.openDnsSettings")} />
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

        {ready ? (
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

      <section className="adguard-panel adguard-aio-panel">
          <div className="adguard-panel-heading">
            <div><span className="adguard-eyebrow">ROOTGUARD AIO</span><h2>{t("adguard.automatic")}</h2></div>
          </div>
          <div className="managed-steps">
            <ManagedStep icon={<Network />} number="01" title={t("adguard.privateNetwork")} text={t("adguard.privateNetworkText")} />
            <ManagedStep icon={<KeyRound />} number="02" title={t("adguard.credentials")} text={t("adguard.credentialsText")} />
            <ManagedStep icon={<Filter />} number="03" title={t("adguard.secureUpstream")} text={t("adguard.secureUpstreamText")} />
          </div>
      </section>

      <ContentModal open={filterModalOpen} size="medium" title={t("adguard.filterTestTitle")} eyebrow={t("adguard.filterTestEyebrow")} closeLabel={t("common.close")} onClose={() => setFilterModalOpen(false)}>
        <div className="adguard-filter-modal">
          <div className="adguard-filter-modal-intro">
            <p>{t("adguard.filterTestHelp")}</p>
            <button className="rg-button rg-button-secondary" type="button" disabled={testingFilters} onClick={() => void testFilters()}>
              <RefreshCw size={16} className={testingFilters ? "spin" : ""} />
              {testingFilters ? t("adguard.filterTesting") : t("adguard.filterTestRun")}
            </button>
          </div>
          {filterError && <div className="adguard-feedback error">{filterError}</div>}
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

      <section className="adguard-security-note">
        <LockKeyhole size={20} />
        <div>
          <strong>{t("adguard.why")}</strong>
          <p>{t("adguard.whyText")}</p>
        </div>
      </section>
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

function StatusRow({ label, active, activeText, inactiveText, adguardHash, adguardHashLabel }: {
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
}) {
  return (
    <div>
      <span className={active ? "status-check active" : "status-check"}>{active ? <Check size={14} /> : "!"}</span>
      <strong>
        {label}
        {adguardHash && (
          <a className="adguard-contextual-link" href={`/adguard-ui/${adguardHash}`} target="_blank" rel="noreferrer" title={adguardHashLabel}>
            <ExternalLink size={13} aria-hidden="true" />
            <span className="sr-only">{adguardHashLabel}</span>
          </a>
        )}
      </strong>
      <small>{active ? activeText : inactiveText}</small>
    </div>
  );
}

function ManagedStep({ icon, number, title, text }: {
  icon: React.ReactNode;
  number: string;
  title: string;
  text: string;
}) {
  return (
    <article>
      <span className="managed-step-icon">{icon}</span>
      <div><small>{number}</small><strong>{title}</strong><p>{text}</p></div>
    </article>
  );
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
