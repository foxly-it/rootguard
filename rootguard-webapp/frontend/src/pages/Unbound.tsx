import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { useLocation, useNavigate, useParams } from "react-router";
import {
  Activity, Code2, Expand, MapPinned, SlidersHorizontal,
  Sparkles, Settings2, Lightbulb, Home, Lock, Route, FileText, SquarePen, History as HistoryIcon,
  Router as RouterIcon,
} from "lucide-react";
import {
  fetchUnboundDiagnostics,
  fetchUnboundDiagnosticLoggingStatus,
  fetchUnboundHistory,
  fetchUnboundAdvice,
  fetchUnboundActiveConfiguration,
  fetchUnboundPresets,
  fetchUnboundSettings,
  fetchUnboundNetworkCapabilities,
  previewUnboundSettings,
  startUnboundDiagnosticLogging,
  stopUnboundDiagnosticLogging,
  restoreUnboundVersion,
  updateUnboundSettings,
  type UnboundDiagnosticReport,
  type UnboundDiagnosticLoggingStatus,
  type UnboundAdvice,
  type UnboundActiveConfiguration,
  type UnboundHistoryEntry,
  type UnboundPreset,
  type UnboundPreview,
  type UnboundSettings,
  type UnboundNetworkCapabilities,
} from "../api/client";
import "../styles/unbound.css";
import "../styles/unbound-live.css";
import "../styles/unbound-polish.css";
import "../styles/unbound-structure.css";
import "../styles/unbound-actions.css";
import UnboundExpertEditor from "../components/UnboundExpertEditor";
import UnboundForwardZones from "../components/UnboundForwardZones";
import UnboundRouterImport from "../components/UnboundRouterImport";
import UnboundGuidedZones from "../components/UnboundGuidedZones";
import UnboundPrivateDomains from "../components/UnboundPrivateDomains";
import ContentModal from "../components/ContentModal";
import { useI18n } from "../i18n";

export default function Unbound() {
  const { t, formatDate } = useI18n();
  const [settings, setSettings] = useState<UnboundSettings | null>(null);
  const [history, setHistory] = useState<UnboundHistoryEntry[]>([]);
  const [preview, setPreview] = useState<UnboundPreview | null>(null);
  const [diagnostics, setDiagnostics] = useState<UnboundDiagnosticReport | null>(null);
  const [diagnosticLogging, setDiagnosticLogging] = useState<UnboundDiagnosticLoggingStatus | null>(null);
  const [presets, setPresets] = useState<UnboundPreset[]>([]);
  const [advice, setAdvice] = useState<UnboundAdvice | null>(null);
  const [liveConfig, setLiveConfig] = useState<UnboundActiveConfiguration | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const navigate = useNavigate();
  const location = useLocation();
  const { section: sectionParam } = useParams<{ section?: string }>();
  const activeSection: UnboundSection = isUnboundSection(sectionParam) ? sectionParam : "overview";
  const setActiveSection = useCallback(
    (section: UnboundSection) => navigate(section === "overview" ? "/unbound" : `/unbound/${section}`, { replace: true }),
    [navigate],
  );
  const [networkCapabilities, setNetworkCapabilities] = useState<UnboundNetworkCapabilities | null>(null);
  const [configModal, setConfigModal] = useState<"base" | "custom" | null>(null);

  const reload = useCallback(async () => {
    const [loadedSettings, loadedHistory, loadedPresets, loadedConfig, loadedDiagnosticLogging] = await Promise.all([
      fetchUnboundSettings(),
      fetchUnboundHistory(),
      fetchUnboundPresets(),
      fetchUnboundActiveConfiguration(),
      fetchUnboundDiagnosticLoggingStatus(),
    ]);
    setSettings({
      ...loadedSettings,
      forward_zones: loadedSettings.forward_zones ?? [],
      private_domains: loadedSettings.private_domains ?? [],
      reverse_zones: loadedSettings.reverse_zones ?? [],
      local_zones: loadedSettings.local_zones ?? [],
      network_mode: loadedSettings.network_mode ?? "ipv4",
      resource_profile: loadedSettings.resource_profile ?? "medium",
      prefetch_key: loadedSettings.prefetch_key ?? true,
      aggressive_nsec: loadedSettings.aggressive_nsec ?? true,
      edns_buffer_size: loadedSettings.edns_buffer_size ?? 1232,
      log_verbosity: loadedSettings.log_verbosity ?? 1,
      serve_expired_ttl: loadedSettings.serve_expired_ttl ?? 86400,
      serve_expired_client_timeout: loadedSettings.serve_expired_client_timeout ?? 1800,
    });
    setHistory(loadedHistory);
    setPresets(loadedPresets);
    setLiveConfig(loadedConfig);
    setDiagnosticLogging(loadedDiagnosticLogging);
  }, []);

  async function checkNetworkCapabilities() {
    if (busy) return;
    setBusy(true);
    clearFeedback();
    try {
      setNetworkCapabilities(await fetchUnboundNetworkCapabilities());
      setMessage(t("network.checked"));
    } catch (err) {
      setError(errorMessage(err, t("network.checkError")));
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    reload()
      .catch((err: unknown) => setError(errorMessage(err, t("unbound.loadError"))))
      .finally(() => setLoading(false));
  }, [reload, t]);

  useEffect(() => {
    if (!settings) return;
    let current = true;
    const request = window.setTimeout(() => {
      fetchUnboundAdvice(settings)
        .then((nextAdvice) => { if (current) setAdvice(nextAdvice); })
        .catch(() => { if (current) setAdvice(null); });
    }, 250);
    return () => {
      current = false;
      window.clearTimeout(request);
    };
  }, [settings]);

  useEffect(() => {
    if (!diagnosticLogging?.active) return;
    const refresh = window.setInterval(() => {
      fetchUnboundDiagnosticLoggingStatus()
        .then(setDiagnosticLogging)
        .catch(() => undefined);
    }, 10_000);
    return () => window.clearInterval(refresh);
  }, [diagnosticLogging?.active]);

  // Deep-links and search results can point at a specific section within a
  // tab (e.g. "/unbound/advanced#unbound-section-advanced-expert"), not just
  // the tab itself. Both panels and their sections stay in the DOM (toggled
  // via `hidden`), so once loading finishes the target is guaranteed to
  // exist for every section id this page actually renders.
  useEffect(() => {
    if (loading || !location.hash) return;
    jumpToSection(location.hash.slice(1));
  }, [loading, location.hash]);

  async function selectPreset(preset: UnboundPreset) {
    if (busy || !settings) return;
    setBusy(true);
    clearFeedback();
    try {
      const proposed = {
        ...preset.settings,
        forward_zones: settings.forward_zones,
        private_domains: settings.private_domains,
        reverse_zones: settings.reverse_zones,
        network_mode: settings.network_mode,
      };
      setSettings(proposed);
      setPreview(await previewUnboundSettings(proposed));
      setMessage(t("unbound.presetLoaded", { name: presetText(preset.id, "name", t, preset.name) }));
    } catch (err) {
      setError(errorMessage(err, t("unbound.presetError")));
    } finally {
      setBusy(false);
    }
  }

  async function createPreview(event: FormEvent) {
    event.preventDefault();
    if (!settings || busy) return;
    setBusy(true);
    clearFeedback();
    try {
      setPreview(await previewUnboundSettings(settings));
    } catch (err) {
      setError(errorMessage(err, t("unbound.previewError")));
    } finally {
      setBusy(false);
    }
  }

  async function applyPreview() {
    if (!settings || busy || !preview?.changed) return;
    setBusy(true);
    clearFeedback();
    try {
      const updated = await updateUnboundSettings(settings);
      setSettings(updated);
      setPreview(null);
      await reload();
      setMessage(t("unbound.activated"));
    } catch (err) {
      setError(errorMessage(err, t("unbound.activateError")));
    } finally {
      setBusy(false);
    }
  }

  async function restore(entry: UnboundHistoryEntry) {
    if (busy || !window.confirm(`Version vom ${formatDate(entry.created_at)} wirklich wiederherstellen?`)) return;
    setBusy(true);
    clearFeedback();
    try {
      setSettings(await restoreUnboundVersion(entry.id));
      setPreview(null);
      await reload();
      setMessage(t("unbound.restored"));
    } catch (err) {
      setError(errorMessage(err, t("unbound.restoreError")));
    } finally {
      setBusy(false);
    }
  }

  async function runDiagnostics() {
    if (busy) return;
    setBusy(true);
    clearFeedback();
    try {
      setDiagnostics(await fetchUnboundDiagnostics());
    } catch (err) {
      setError(errorMessage(err, t("unbound.diagnosticError")));
    } finally {
      setBusy(false);
    }
  }

  async function toggleDiagnosticLogging() {
    if (busy) return;
    setBusy(true);
    clearFeedback();
    try {
      const status = diagnosticLogging?.active
        ? await stopUnboundDiagnosticLogging()
        : await startUnboundDiagnosticLogging();
      setDiagnosticLogging(status);
      setMessage(t(status.active ? "unbound.diagnosticLoggingStarted" : "unbound.diagnosticLoggingStopped"));
    } catch (err) {
      setError(errorMessage(err, t("unbound.diagnosticLoggingError")));
    } finally {
      setBusy(false);
    }
  }

  function clearFeedback() {
    setMessage("");
    setError("");
  }

  if (loading) return <Page><p>{t("unbound.loading")}</p></Page>;
  if (!settings) return <Page><p className="error-message">{error}</p></Page>;
  const activePreset = presets.find((preset) => settingsEqual(settings, preset.settings));

  return (
    <Page>
      <div className="unbound-heading">
        <div>
          <p className="unbound-eyebrow">{t("unbound.pageEyebrow")}</p>
          <h1>{t("unbound.title")}</h1>
          <p>{t("unbound.intro")}</p>
        </div>
        <div className="unbound-heading-status" aria-label={t("unbound.resolverStatus")}>
          <i aria-hidden="true" />
          <span><small>{t("unbound.resolverStatus")}</small><strong>{liveConfig ? t("unbound.statusActive") : t("unbound.statusUnknown")}</strong></span>
        </div>
      </div>

      <UnboundTabs active={activeSection} onChange={setActiveSection} t={t} />
      <UnboundSectionNav section={activeSection} hash={location.hash} t={t} />

      <div className="unbound-feedback" aria-live="polite" aria-atomic="true">
        {message && <div className="feedback success">{message}</div>}
        {error && <div className="feedback error" role="alert">{error}</div>}
      </div>

      <section id="unbound-panel-overview" role="tabpanel" aria-labelledby="unbound-tab-overview" hidden={activeSection !== "overview"} tabIndex={0}>
        <div className="unbound-summary-grid">
          <SummaryCard label={t("unbound.summary.status")} value={liveConfig ? t("unbound.statusActive") : t("unbound.statusUnknown")} detail={t("unbound.summary.statusHelp")} state={liveConfig ? "healthy" : "neutral"} />
          <SummaryCard label={t("unbound.summary.profile")} value={activePreset ? presetText(activePreset.id, "name", t, activePreset.name) : t("unbound.summary.custom")} detail={t("unbound.summary.profileHelp")} />
          <SummaryCard label={t("unbound.summary.history")} value={t("unbound.summary.versionCount", { count: history.length })} detail={history[0] ? formatDate(history[0].created_at) : t("unbound.noHistoryShort")} />
          <SummaryCard label={t("unbound.summary.customConfig")} value={liveConfig?.custom_config ? t("common.active") : t("unbound.summary.none")} detail={t("unbound.summary.customHelp")} />
        </div>
        <section id="unbound-section-overview-diagnostics" className="glass-card compact overview-diagnostics" tabIndex={-1}>
          <div>
            <p className="unbound-eyebrow">{t("unbound.overviewHealth")}</p>
            <h2>{t("unbound.liveDiagnostics")}</h2>
            <p className="muted-copy">{t("unbound.diagnosticsHelp")}</p>
          </div>
          <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy} onClick={runDiagnostics}>
            {busy ? t("unbound.wait") : t("unbound.diagnose")}
          </button>
          <div className="diagnostic-logging-control">
            <div>
              <strong>{t("unbound.diagnosticLogging")}</strong>
              <small>{diagnosticLogging?.active && diagnosticLogging.expires_at
                ? t("unbound.diagnosticLoggingActive", { date: formatDate(diagnosticLogging.expires_at) })
                : t("unbound.diagnosticLoggingHelp")}</small>
            </div>
            <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy} onClick={toggleDiagnosticLogging}>
              {t(diagnosticLogging?.active ? "unbound.diagnosticLoggingStop" : "unbound.diagnosticLoggingStart")}
            </button>
          </div>
          {diagnostics && <div className="diagnostic-results"><div className={`overall-status ${diagnostics.healthy ? "healthy" : "failed"}`}>{diagnostics.healthy ? t("unbound.allPassed") : t("unbound.someFailed")}</div>{diagnostics.checks.map((check) => <DiagnosticRow key={check.name} {...check} label={fieldLabel(check.name, t)} />)}<small className="timestamp">{t("unbound.checked", { date: formatDate(diagnostics.checked_at) })}</small></div>}
        </section>
      </section>

      <section id="unbound-panel-resolver" role="tabpanel" aria-labelledby="unbound-tab-resolver" hidden={activeSection !== "resolver"} tabIndex={0}>
        <section id="unbound-section-resolver-presets" className="glass-card preset-panel" tabIndex={-1}>
          <div className="panel-heading"><div><p className="unbound-eyebrow">{t("unbound.presets")}</p><h2>{t("unbound.chooseProfile")}</h2></div><span>{t("unbound.draftOnly")}</span></div>
          <div className="preset-grid">{presets.map((preset) => (
            <button key={preset.id} className={`preset-card ${settingsEqual(settings, preset.settings) ? "selected" : ""}`} type="button" aria-pressed={settingsEqual(settings, preset.settings)} disabled={busy} onClick={() => selectPreset(preset)}>
              <span className="preset-name">{presetText(preset.id, "name", t, preset.name)}</span>
              <small>{presetText(preset.id, "description", t, preset.description)}</small>
              <em>{presetText(preset.id, "bestFor", t, preset.best_for)}</em>
            </button>
          ))}</div>
        </section>

        <div className="unbound-grid">
          <form id="unbound-section-resolver-settings" onSubmit={createPreview} className="glass-card compact settings-panel" tabIndex={-1}>
            <h2>{t("unbound.resolverSettings")}</h2>
            <p className="muted-copy">{t("unbound.resolverSettingsHelp")}</p>
            <Toggle directive="qname-minimisation" label={t("unbound.qname")} badge={t("unbound.qnameBadge")} description={t("unbound.qnameHelp")} checked={settings.qname_minimisation} onChange={(value) => setSettings({ ...settings, qname_minimisation: value })} />
            <Toggle directive="prefetch" label={t("unbound.prefetch")} badge={t("unbound.prefetchBadge")} description={t("unbound.prefetchHelp")} checked={settings.prefetch} onChange={(value) => setSettings({ ...settings, prefetch: value })} />
            <Toggle directive="serve-expired" label={t("unbound.expired")} badge={t("unbound.expiredBadge")} description={t("unbound.expiredHelp")} checked={settings.serve_expired} onChange={(value) => setSettings({ ...settings, serve_expired: value })} />
            <div className="network-mode-setting">
              <div className="network-mode-heading"><div><strong>{t("network.title")}</strong><small>{t("network.help")}</small></div><button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy} onClick={checkNetworkCapabilities}>{t("network.check")}</button></div>
              {networkCapabilities && <div className="network-capabilities">
                <span className={networkCapabilities.ipv4_available ? "available" : "unavailable"}><b>IPv4</b><small>{networkCapabilities.ipv4_available ? t("network.available") : networkCapabilities.ipv4_detail}</small></span>
                <span className={networkCapabilities.ipv6_available ? "available" : "unavailable"}><b>IPv6</b><small>{networkCapabilities.ipv6_available ? t("network.available") : networkCapabilities.ipv6_detail}</small></span>
              </div>}
              <div className="network-mode-options" role="radiogroup" aria-label={t("network.title")}>
                {(["ipv4", "dual", "ipv6"] as const).map((mode) => {
                  const requiresIPv6 = mode !== "ipv4";
                  const unavailable = requiresIPv6 && !networkCapabilities?.ipv6_available;
                  return <label className={unavailable ? "unavailable" : ""} key={mode}><input type="radio" name="network-mode" checked={settings.network_mode === mode} disabled={unavailable} onChange={() => setSettings({ ...settings, network_mode: mode })} /><span><b>{t(`network.${mode}`)}</b><small>{t(`network.${mode}Help`)}</small></span></label>;
                })}
              </div>
              {settings.network_mode !== "ipv4" && !networkCapabilities?.ipv6_available && <p className="network-warning">{t("network.activationBlocked")}</p>}
              <code className="setting-directive">do-ip4 / do-ip6 / prefer-ip6</code>
            </div>
            <details className="advanced-settings">
              <summary><span>{t("unbound.cachePerformance")}</span><small>{t("unbound.cachePerformanceHelp")}</small></summary>
              <label className="resource-profile-field">
                <strong>{t("unbound.resourceProfile")}</strong>
                <select value={settings.resource_profile} onChange={(event) => setSettings({ ...settings, resource_profile: event.target.value as UnboundSettings["resource_profile"] })}>
                  {(["small", "medium", "large"] as const).map((profile) => <option key={profile} value={profile}>{t(`unbound.resourceProfile.${profile}`)}</option>)}
                </select>
                <small>{t(`unbound.resourceProfile.${settings.resource_profile}Help`)}</small>
                <code className="setting-directive">{resourceProfileDirectives(settings.resource_profile)}</code>
              </label>
              <Toggle directive="prefetch-key" label={t("unbound.prefetchKey")} badge={t("unbound.prefetchKeyBadge")} description={t("unbound.prefetchKeyHelp")} checked={settings.prefetch_key} onChange={(value) => setSettings({ ...settings, prefetch_key: value })} />
              <Toggle directive="aggressive-nsec" label={t("unbound.aggressiveNsec")} badge={t("unbound.aggressiveNsecBadge")} description={t("unbound.aggressiveNsecHelp")} checked={settings.aggressive_nsec} onChange={(value) => setSettings({ ...settings, aggressive_nsec: value })} />
              <Toggle directive="verbosity" label={t("unbound.operationalLogging")} badge={t("unbound.operationalLoggingBadge")} description={t("unbound.operationalLoggingHelp")} checked={settings.log_verbosity === 1} onChange={(value) => setSettings({ ...settings, log_verbosity: value ? 1 : 0 })} />
              <div className="number-grid">
                <NumberField directive="edns-buffer-size" label={t("unbound.ednsBufferSize")} description={t("unbound.ednsBufferSizeHelp")} recommended={t("unbound.recommended", { value: "1.232 Byte" })} value={settings.edns_buffer_size} min={512} max={4096} onChange={(value) => setSettings({ ...settings, edns_buffer_size: value })} />
                <NumberField directive="serve-expired-ttl" label={t("unbound.expiredTtl")} description={t("unbound.expiredTtlHelp")} recommended={t("unbound.recommended", { value: "86.400" })} value={settings.serve_expired_ttl} min={3600} max={604800} onChange={(value) => setSettings({ ...settings, serve_expired_ttl: value })} />
                <NumberField directive="serve-expired-client-timeout" label={t("unbound.expiredTimeout")} description={t("unbound.expiredTimeoutHelp")} recommended={t("unbound.recommended", { value: "1.800 ms" })} value={settings.serve_expired_client_timeout} min={0} max={5000} onChange={(value) => setSettings({ ...settings, serve_expired_client_timeout: value })} />
                <NumberField directive="cache-min-ttl" label="Minimum TTL" description={t("unbound.minTtlHelp")} recommended={t("unbound.recommended", { value: "0–300" })} value={settings.cache_min_ttl} min={0} max={3600} onChange={(value) => setSettings({ ...settings, cache_min_ttl: value })} />
                <NumberField directive="cache-max-ttl" label="Maximum TTL" description={t("unbound.maxTtlHelp")} recommended={t("unbound.recommended", { value: "3.600–172.800" })} value={settings.cache_max_ttl} min={60} max={604800} onChange={(value) => setSettings({ ...settings, cache_max_ttl: value })} />
                <NumberField directive="num-threads" label={t("unbound.threads")} description={t("unbound.threadsHelp")} recommended={t("unbound.recommended", { value: "2–4" })} value={settings.threads} min={1} max={32} onChange={(value) => setSettings({ ...settings, threads: value })} />
              </div>
            </details>
            <button className="rg-button rg-button-primary" type="submit" disabled={busy}>{t("unbound.review")}</button>
          </form>
          <section id="unbound-section-resolver-advisor" className="glass-card compact side-panel advisor-panel" tabIndex={-1}>
            <div className="advisor-heading"><h2>RootGuard Advisor</h2>{advice && <span className={`advice-state ${advice.status}`}>{t(`unbound.advice.${advice.status}`)}</span>}</div>
            {!advice && <p className="muted-copy">{t("unbound.advisorHelp")}</p>}
            {advice?.recommendations.map((item) => <article className={`advice-item ${item.severity}`} key={item.id}><strong>{adviceText(item.id, "title", t, item.title)}</strong><p>{adviceText(item.id, "description", t, item.description)}</p><small>{adviceText(item.id, "suggestion", t, item.suggestion)}</small></article>)}
          </section>
        </div>
        {preview && (
          <section className="glass-card preview-panel" aria-live="polite">
            <div className="panel-heading"><div><p className="unbound-eyebrow">{t("unbound.preview")}</p><h2>{t("unbound.changes")}</h2></div><button className="text-action" type="button" onClick={() => setPreview(null)}>{t("common.close")}</button></div>
            {!preview.changed ? <p>{t("unbound.noChanges")}</p> : <><div className="change-list">{preview.changes.map((change) => <div key={change.field}><code>{fieldLabel(change.field, t)}</code><span>{change.before}</span><b aria-hidden="true">→</b><span>{change.after}</span></div>)}</div><details><summary>{t("unbound.showGenerated")}</summary><pre tabIndex={0} aria-label={t("unbound.showGenerated")}>{preview.rendered_config}</pre></details><button className="rg-button rg-button-primary" type="button" disabled={busy} onClick={applyPreview}>{busy ? t("unbound.activating") : t("unbound.validateActivate")}</button></>}
          </section>
        )}
      </section>

      <section id="unbound-panel-zones" role="tabpanel" aria-labelledby="unbound-tab-zones" hidden={activeSection !== "zones"} tabIndex={0}>
        <div className="section-introduction"><p className="unbound-eyebrow">{t("unbound.localDnsEyebrow")}</p><h2>{t("unbound.localDnsTitle")}</h2><p>{t("unbound.localDnsHelp")}</p></div>
        <UnboundGuidedZones id="unbound-section-zones-local" version={history[0]?.id} onActivated={reload} />
        <UnboundPrivateDomains id="unbound-section-zones-private" version={history[0]?.id} onActivated={reload} />
        <UnboundForwardZones id="unbound-section-zones-forwarding" version={history[0]?.id} onActivated={reload} />
        <UnboundRouterImport id="unbound-section-zones-router-import" version={history[0]?.id} onActivated={reload} />
      </section>

      <section id="unbound-panel-advanced" role="tabpanel" aria-labelledby="unbound-tab-advanced" hidden={activeSection !== "advanced"} tabIndex={0}>
        <div className="section-introduction"><p className="unbound-eyebrow">{t("unbound.advancedEyebrow")}</p><h2>{t("unbound.advancedTitle")}</h2><p>{t("unbound.advancedHelp")}</p></div>
        {liveConfig && (
          <section id="unbound-section-advanced-live" className="glass-card live-config-panel" tabIndex={-1}>
            <div className="panel-heading"><div><p className="unbound-eyebrow">LIVE · READ ONLY</p><h2>{t("unbound.liveTitle")}</h2><p className="muted-copy">{t("unbound.liveHelp")}</p></div><span className="live-config-state"><i /> {t("common.active")} · {formatDate(liveConfig.checked_at)}</span></div>
            <div className="config-file-label"><span>50-rootguard.conf</span><code>/etc/unbound/unbound.d/50-rootguard.conf</code></div>
            <details className="live-config-disclosure"><summary>{t("unbound.managedConfig")}</summary><pre tabIndex={0} aria-label={t("unbound.managedConfig")}>{liveConfig.managed_config}</pre></details>
            <div className="live-config-details">
              <button type="button" onClick={() => setConfigModal("base")}><span>{t("unbound.baseConfig")}</span><Expand size={16} /></button>
              <button type="button" onClick={() => setConfigModal("custom")}><span>{t("unbound.customConfig")}</span><Expand size={16} /></button>
            </div>
            <ContentModal open={configModal !== null} eyebrow="LIVE · READ ONLY" title={configModal === "base" ? t("unbound.baseConfig") : t("unbound.customConfig")} closeLabel={t("common.close")} onClose={() => setConfigModal(null)}>
              <pre tabIndex={0} aria-label={configModal === "base" ? t("unbound.baseConfig") : t("unbound.customConfig")}>{configModal === "base" ? liveConfig.base_config : liveConfig.custom_config || t("unbound.noCustom")}</pre>
            </ContentModal>
          </section>
        )}
        <UnboundExpertEditor id="unbound-section-advanced-expert" version={history[0]?.id} baseConfig={liveConfig?.base_config} onActivated={reload} />
        <section id="unbound-section-advanced-history" className="glass-card history-panel" tabIndex={-1}>
          <details className="history-disclosure">
            <summary><span><span className="unbound-eyebrow">ROLLBACK</span><strong>{t("unbound.history")}</strong></span><em>{t("unbound.versions", { count: history.length })}</em></summary>
            {history.length === 0 ? <p className="muted-copy">{t("unbound.noHistory")}</p> : <div className="history-list">{history.map((entry, index) => <article key={entry.id}><div><strong>{index === 0 ? t("unbound.latest") : t("unbound.saved")}</strong><span>{formatDate(entry.created_at)}</span><small>Threads {entry.settings.threads} · TTL {entry.settings.cache_min_ttl}–{entry.settings.cache_max_ttl} · {t("forward.historyCount", { count: entry.settings.forward_zones?.length ?? 0 })}{entry.custom_config ? " · Custom Config" : ""}</small></div><button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy || index === 0} onClick={() => restore(entry)}>{index === 0 ? t("common.active") : t("unbound.restore")}</button></article>)}</div>}
          </details>
        </section>
      </section>
    </Page>
  );
}

type UnboundSection = "overview" | "resolver" | "zones" | "advanced";

const UNBOUND_SECTIONS: readonly UnboundSection[] = ["overview", "resolver", "zones", "advanced"];

function isUnboundSection(value: string | undefined): value is UnboundSection {
  return !!value && (UNBOUND_SECTIONS as readonly string[]).includes(value);
}

function UnboundTabs({ active, onChange, t }: { active: UnboundSection; onChange: (section: UnboundSection) => void; t: (key: string) => string }) {
  const tabs: Array<{ id: UnboundSection; icon: React.ReactNode }> = [
    { id: "overview", icon: <Activity aria-hidden="true" /> },
    { id: "resolver", icon: <SlidersHorizontal aria-hidden="true" /> },
    { id: "zones", icon: <MapPinned aria-hidden="true" /> },
    { id: "advanced", icon: <Code2 aria-hidden="true" /> },
  ];
  function handleKeys(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const next = event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : (index + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) % tabs.length;
    onChange(tabs[next].id);
    event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus();
  }
  return <div className="unbound-tabs" role="tablist" aria-label={t("unbound.navigation")}>{tabs.map((tab, index) => <button id={`unbound-tab-${tab.id}`} role="tab" type="button" key={tab.id} aria-selected={active === tab.id} aria-controls={`unbound-panel-${tab.id}`} tabIndex={active === tab.id ? 0 : -1} onKeyDown={(event) => handleKeys(event, index)} onClick={() => onChange(tab.id)}>{tab.icon}<span>{t(`unbound.tab.${tab.id}`)}</span></button>)}</div>;
}

interface UnboundSubSection { id: string; label: string; icon: React.ReactNode }

function sectionsFor(section: UnboundSection, t: (key: string) => string): UnboundSubSection[] {
  switch (section) {
    case "resolver":
      return [
        { id: "unbound-section-resolver-presets", label: t("unbound.section.profiles"), icon: <Sparkles aria-hidden="true" /> },
        { id: "unbound-section-resolver-settings", label: t("unbound.resolverSettings"), icon: <Settings2 aria-hidden="true" /> },
        { id: "unbound-section-resolver-advisor", label: "RootGuard Advisor", icon: <Lightbulb aria-hidden="true" /> },
      ];
    case "zones":
      return [
        { id: "unbound-section-zones-local", label: t("zones.title"), icon: <Home aria-hidden="true" /> },
        { id: "unbound-section-zones-private", label: t("private.title"), icon: <Lock aria-hidden="true" /> },
        { id: "unbound-section-zones-forwarding", label: t("forward.title"), icon: <Route aria-hidden="true" /> },
        { id: "unbound-section-zones-router-import", label: t("routerImport.title"), icon: <RouterIcon aria-hidden="true" /> },
      ];
    case "advanced":
      return [
        { id: "unbound-section-advanced-live", label: t("unbound.liveTitle"), icon: <FileText aria-hidden="true" /> },
        { id: "unbound-section-advanced-expert", label: t("expert.title"), icon: <SquarePen aria-hidden="true" /> },
        { id: "unbound-section-advanced-history", label: t("unbound.history"), icon: <HistoryIcon aria-hidden="true" /> },
      ];
    default:
      return [];
  }
}

function jumpToSection(id: string) {
  const target = document.getElementById(id);
  if (!target) return;
  // A collapsed <details> (e.g. the history panel) would otherwise scroll
  // into view showing only its closed summary - open it so "reveal the
  // section" actually reveals its content, not just its header.
  target.querySelector<HTMLDetailsElement>("details")?.setAttribute("open", "");
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  target.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth", block: "start" });
  target.focus({ preventScroll: true });
  window.history.replaceState(null, "", `#${id}`);
}

function UnboundSectionNav({ section, hash, t }: { section: UnboundSection; hash: string; t: (key: string) => string }) {
  const sections = useMemo(() => sectionsFor(section, t), [section, t]);
  const requestedId = hash.slice(1);
  const [activeId, setActiveId] = useState("");
  const [isRailMode, setIsRailMode] = useState(false);
  const navRef = useRef<HTMLElement>(null);

  // Tracks the rail breakpoint (see the media query in unbound-structure.css).
  useEffect(() => {
    const rail = window.matchMedia("(min-width: 1440px)");
    const update = () => setIsRailMode(rail.matches);
    update();
    rail.addEventListener("change", update);
    return () => rail.removeEventListener("change", update);
  }, []);

  // In rail mode the nav is a fixed-position element portaled straight to
  // <body> (see the render below) - .unbound-page permanently carries a
  // non-"none" transform from its own entrance animation (fill-mode: both
  // leaves transform: translateY(0) applied forever, and any transform
  // other than the literal keyword "none" makes an element the containing
  // block for its position: fixed descendants, per spec). Left in place,
  // that silently repositions the rail relative to the content box instead
  // of the viewport - miles off on a wide monitor where the centered,
  // max-width content sits nowhere near the screen's actual edge. Portaling
  // to <body> (which has no such transform) sidesteps the problem entirely,
  // the same way the sidebar's own tooltip already does (see rg-nav-tooltip
  // in SidebarLayout.tsx) - so this positioning effect only needs to track
  // .unbound-page's real edge, never fight a containing-block surprise.
  useEffect(() => {
    const nav = navRef.current;
    if (!nav) return;
    if (!isRailMode) {
      nav.style.left = "";
      return;
    }
    const page = document.querySelector<HTMLElement>(".unbound-page");
    if (!page) return;

    function position() {
      if (!nav) return;
      nav.style.left = `${page!.getBoundingClientRect().right + 24}px`;
    }

    position();
    window.addEventListener("resize", position);
    const observer = new ResizeObserver(position);
    observer.observe(page);
    return () => {
      window.removeEventListener("resize", position);
      observer.disconnect();
    };
  }, [isRailMode]);

  useEffect(() => {
    if (sections.length === 0) { setActiveId(""); return; }
    // Seed from the requested hash (a deep link or search result) so the
    // nav highlights the actual destination right away, instead of always
    // defaulting to the first section until the next manual scroll makes
    // the observer below correct it.
    setActiveId(sections.some((entry) => entry.id === requestedId) ? requestedId : sections[0].id);
    const elements = sections
      .map((entry) => document.getElementById(entry.id))
      .filter((element): element is HTMLElement => element !== null);
    if (elements.length === 0) return;
    // The seeded value above can be permanently un-reachable by the
    // observer below: the last section on a short tab can't be scrolled
    // any further up once the page hits its max scroll position, so it may
    // never count as "intersecting" the top band even at rest - that's not
    // transient race noise, it's the genuine final geometry. So the
    // observer stays inert until the user actually tries to scroll
    // themselves; only then does real scroll-spy behaviour take over.
    let userScrolled = false;
    const markScrolled = () => { userScrolled = true; };
    const scrollHost = document.getElementById("main-content");
    scrollHost?.addEventListener("wheel", markScrolled, { passive: true, once: true });
    scrollHost?.addEventListener("touchmove", markScrolled, { passive: true, once: true });
    scrollHost?.addEventListener("keydown", markScrolled, { once: true });
    // Each callback only reports entries whose intersection state actually
    // changed since the last check, not a full snapshot of every observed
    // element - so the latest state per id has to be tracked across calls
    // rather than treating each callback's entries as the complete picture.
    const latestById = new Map<string, IntersectionObserverEntry>();
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => latestById.set(entry.target.id, entry));
        if (!userScrolled) return;
        const visible = Array.from(latestById.values())
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
        if (visible) setActiveId(visible.target.id);
      },
      { rootMargin: "-96px 0px -60% 0px", threshold: 0 },
    );
    elements.forEach((element) => observer.observe(element));
    return () => {
      observer.disconnect();
      scrollHost?.removeEventListener("wheel", markScrolled);
      scrollHost?.removeEventListener("touchmove", markScrolled);
      scrollHost?.removeEventListener("keydown", markScrolled);
    };
  }, [sections, requestedId]);

  if (sections.length < 2) return null;

  const navElement = (
    <nav ref={navRef} className="unbound-section-nav" aria-label={t("unbound.sectionNav")}>
      {sections.map((entry) => (
        <a
          key={entry.id}
          href={`#${entry.id}`}
          className={activeId === entry.id ? "active" : ""}
          data-tooltip={entry.label}
          onClick={(event) => { event.preventDefault(); jumpToSection(entry.id); setActiveId(entry.id); }}
        >
          {entry.icon}
          <span className="unbound-section-nav-label">{entry.label}</span>
        </a>
      ))}
    </nav>
  );

  return isRailMode ? createPortal(navElement, document.body) : navElement;
}

function SummaryCard({ label, value, detail, state = "neutral" }: { label: string; value: string; detail: string; state?: "healthy" | "neutral" }) {
  return <article className="unbound-summary-card"><span>{label}</span><strong><i className={state} aria-hidden="true" />{value}</strong><small>{detail}</small></article>;
}

function Page({ children }: { children: React.ReactNode }) {
  return <div className="unbound-page">{children}</div>;
}

function Toggle({ directive, label, badge, description, checked, onChange }: { directive: string; label: string; badge?: string; description: string; checked: boolean; onChange: (value: boolean) => void }) {
  return <label className="toggle-row"><span><span className="setting-title"><strong>{label}</strong>{badge && <em className="setting-badge">{badge}</em>}</span><code className="setting-directive">{directive}: {checked ? "yes" : "no"}</code><small>{description}</small></span><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} /></label>;
}

function NumberField({ directive, label, description, recommended, value, min, max, onChange }: { directive: string; label: string; description: string; recommended: string; value: number; min: number; max: number; onChange: (value: number) => void }) {
  return <label className="number-field"><strong>{label}</strong><code className="setting-directive">{directive}: {value}</code><input type="number" value={value} min={min} max={max} onChange={(event) => onChange(Number(event.target.value))} /><small>{description}</small><em>{recommended}</em></label>;
}

function DiagnosticRow({ passed, detail, label }: { name: string; passed: boolean; detail: string; label: string }) {
  return <div className="diagnostic-row"><span className={passed ? "check-pass" : "check-fail"}>{passed ? "✓" : "!"}</span><div><strong>{label}</strong><small>{detail}</small></div></div>;
}

function fieldLabel(field: string, t: (key: string) => string) {
  const labels: Record<string, string> = { qname_minimisation: t("unbound.qname"), prefetch: "Prefetch", prefetch_key: t("unbound.prefetchKey"), aggressive_nsec: t("unbound.aggressiveNsec"), edns_buffer_size: t("unbound.ednsBufferSize"), log_verbosity: t("unbound.operationalLogging"), serve_expired: "Serve Expired", serve_expired_ttl: t("unbound.expiredTtl"), serve_expired_client_timeout: t("unbound.expiredTimeout"), cache_min_ttl: "Minimum TTL", cache_max_ttl: "Maximum TTL", threads: t("unbound.threads"), resource_profile: t("unbound.resourceProfile"), network_mode: t("network.title"), forward_zones: t("forward.title"), private_domains: t("private.title"), reverse_zones: t("private.reverseTitle"), configuration: t("unbound.field.configuration"), resolution: t("unbound.field.resolution"), dnssec: "DNSSEC" };
  return labels[field] ?? field;
}

function settingsEqual(left: UnboundSettings, right: UnboundSettings) {
  return left.qname_minimisation === right.qname_minimisation
    && left.prefetch === right.prefetch
    && left.prefetch_key === right.prefetch_key
    && left.aggressive_nsec === right.aggressive_nsec
    && left.edns_buffer_size === right.edns_buffer_size
    && left.log_verbosity === right.log_verbosity
    && left.serve_expired === right.serve_expired
    && left.serve_expired_ttl === right.serve_expired_ttl
    && left.serve_expired_client_timeout === right.serve_expired_client_timeout
    && left.cache_min_ttl === right.cache_min_ttl
    && left.cache_max_ttl === right.cache_max_ttl
    && left.threads === right.threads
    && left.resource_profile === right.resource_profile;
}

function resourceProfileDirectives(profile: UnboundSettings["resource_profile"]) {
  const sizes = { small: ["32m", "16m"], medium: ["64m", "32m"], large: ["128m", "64m"] }[profile];
  return `rrset-cache-size: ${sizes[0]} · msg-cache-size: ${sizes[1]}`;
}

function presetText(id: string, field: "name" | "description" | "bestFor", t: (key: string) => string, fallback: string) {
  const key = `unbound.preset.${id}.${field}`;
  const translated = t(key);
  return translated === key ? fallback : translated;
}

function adviceText(id: string, field: "title" | "description" | "suggestion", t: (key: string) => string, fallback: string) {
  const key = `unbound.recommendation.${id}.${field}`;
  const translated = t(key);
  return translated === key ? fallback : translated;
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
