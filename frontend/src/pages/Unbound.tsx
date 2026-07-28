import { useCallback, useEffect, useState, type FormEvent, type KeyboardEvent } from "react";
import { Activity, Code2, MapPinned, SlidersHorizontal } from "lucide-react";
import {
  fetchUnboundDiagnostics,
  fetchUnboundHistory,
  fetchUnboundAdvice,
  fetchUnboundActiveConfiguration,
  fetchUnboundPresets,
  fetchUnboundSettings,
  previewUnboundSettings,
  restoreUnboundVersion,
  updateUnboundSettings,
  type UnboundDiagnosticReport,
  type UnboundAdvice,
  type UnboundActiveConfiguration,
  type UnboundHistoryEntry,
  type UnboundPreset,
  type UnboundPreview,
  type UnboundSettings,
} from "../api/client";
import "../styles/unbound.css";
import "../styles/unbound-live.css";
import "../styles/unbound-polish.css";
import "../styles/unbound-structure.css";
import UnboundExpertEditor from "../components/UnboundExpertEditor";
import UnboundForwardZones from "../components/UnboundForwardZones";
import UnboundGuidedZones from "../components/UnboundGuidedZones";
import UnboundPrivateDomains from "../components/UnboundPrivateDomains";
import { useI18n } from "../i18n";

export default function Unbound() {
  const { t, formatDate } = useI18n();
  const [settings, setSettings] = useState<UnboundSettings | null>(null);
  const [history, setHistory] = useState<UnboundHistoryEntry[]>([]);
  const [preview, setPreview] = useState<UnboundPreview | null>(null);
  const [diagnostics, setDiagnostics] = useState<UnboundDiagnosticReport | null>(null);
  const [presets, setPresets] = useState<UnboundPreset[]>([]);
  const [advice, setAdvice] = useState<UnboundAdvice | null>(null);
  const [liveConfig, setLiveConfig] = useState<UnboundActiveConfiguration | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [activeSection, setActiveSection] = useState<UnboundSection>("overview");

  const reload = useCallback(async () => {
    const [loadedSettings, loadedHistory, loadedPresets, loadedConfig] = await Promise.all([
      fetchUnboundSettings(),
      fetchUnboundHistory(),
      fetchUnboundPresets(),
      fetchUnboundActiveConfiguration(),
    ]);
    setSettings({
      ...loadedSettings,
      forward_zones: loadedSettings.forward_zones ?? [],
      private_domains: loadedSettings.private_domains ?? [],
      reverse_zones: loadedSettings.reverse_zones ?? [],
    });
    setHistory(loadedHistory);
    setPresets(loadedPresets);
    setLiveConfig(loadedConfig);
  }, []);

  useEffect(() => {
    reload()
      .catch((err: unknown) => setError(errorMessage(err, t("unbound.loadError"))))
      .finally(() => setLoading(false));
  }, [reload, t]);

  useEffect(() => {
    if (!settings) return;
    const request = window.setTimeout(() => {
      fetchUnboundAdvice(settings).then(setAdvice).catch(() => setAdvice(null));
    }, 250);
    return () => window.clearTimeout(request);
  }, [settings]);

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
        <section className="glass-card compact overview-diagnostics">
          <div>
            <p className="unbound-eyebrow">{t("unbound.overviewHealth")}</p>
            <h2>{t("unbound.liveDiagnostics")}</h2>
            <p className="muted-copy">{t("unbound.diagnosticsHelp")}</p>
          </div>
          <button className="secondary-action" type="button" disabled={busy} onClick={runDiagnostics}>
            {busy ? t("unbound.wait") : t("unbound.diagnose")}
          </button>
          {diagnostics && <div className="diagnostic-results"><div className={`overall-status ${diagnostics.healthy ? "healthy" : "failed"}`}>{diagnostics.healthy ? t("unbound.allPassed") : t("unbound.someFailed")}</div>{diagnostics.checks.map((check) => <DiagnosticRow key={check.name} {...check} label={fieldLabel(check.name, t)} />)}<small className="timestamp">{t("unbound.checked", { date: formatDate(diagnostics.checked_at) })}</small></div>}
        </section>
      </section>

      <section id="unbound-panel-resolver" role="tabpanel" aria-labelledby="unbound-tab-resolver" hidden={activeSection !== "resolver"} tabIndex={0}>
        <section className="glass-card preset-panel">
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
          <form onSubmit={createPreview} className="glass-card compact settings-panel">
            <h2>{t("unbound.resolverSettings")}</h2>
            <p className="muted-copy">{t("unbound.resolverSettingsHelp")}</p>
            <Toggle directive="qname-minimisation" label={t("unbound.qname")} badge={t("unbound.qnameBadge")} description={t("unbound.qnameHelp")} checked={settings.qname_minimisation} onChange={(value) => setSettings({ ...settings, qname_minimisation: value })} />
            <Toggle directive="prefetch" label={t("unbound.prefetch")} badge={t("unbound.prefetchBadge")} description={t("unbound.prefetchHelp")} checked={settings.prefetch} onChange={(value) => setSettings({ ...settings, prefetch: value })} />
            <Toggle directive="serve-expired" label={t("unbound.expired")} badge={t("unbound.expiredBadge")} description={t("unbound.expiredHelp")} checked={settings.serve_expired} onChange={(value) => setSettings({ ...settings, serve_expired: value })} />
            <details className="advanced-settings">
              <summary><span>{t("unbound.cachePerformance")}</span><small>{t("unbound.cachePerformanceHelp")}</small></summary>
              <div className="number-grid">
                <NumberField directive="cache-min-ttl" label="Minimum TTL" description={t("unbound.minTtlHelp")} recommended={t("unbound.recommended", { value: "0–300" })} value={settings.cache_min_ttl} min={0} max={3600} onChange={(value) => setSettings({ ...settings, cache_min_ttl: value })} />
                <NumberField directive="cache-max-ttl" label="Maximum TTL" description={t("unbound.maxTtlHelp")} recommended={t("unbound.recommended", { value: "3.600–172.800" })} value={settings.cache_max_ttl} min={60} max={604800} onChange={(value) => setSettings({ ...settings, cache_max_ttl: value })} />
                <NumberField directive="num-threads" label={t("unbound.threads")} description={t("unbound.threadsHelp")} recommended={t("unbound.recommended", { value: "2–4" })} value={settings.threads} min={1} max={32} onChange={(value) => setSettings({ ...settings, threads: value })} />
              </div>
            </details>
            <button type="submit" disabled={busy}>{t("unbound.review")}</button>
          </form>
          <section className="glass-card compact side-panel advisor-panel">
            <div className="advisor-heading"><h2>RootGuard Advisor</h2>{advice && <span className={`advice-state ${advice.status}`}>{t(`unbound.advice.${advice.status}`)}</span>}</div>
            {!advice && <p className="muted-copy">{t("unbound.advisorHelp")}</p>}
            {advice?.recommendations.map((item) => <article className={`advice-item ${item.severity}`} key={item.id}><strong>{adviceText(item.id, "title", t, item.title)}</strong><p>{adviceText(item.id, "description", t, item.description)}</p><small>{adviceText(item.id, "suggestion", t, item.suggestion)}</small></article>)}
          </section>
        </div>
        {preview && (
          <section className="glass-card preview-panel" aria-live="polite">
            <div className="panel-heading"><div><p className="unbound-eyebrow">{t("unbound.preview")}</p><h2>{t("unbound.changes")}</h2></div><button className="text-action" type="button" onClick={() => setPreview(null)}>{t("common.close")}</button></div>
            {!preview.changed ? <p>{t("unbound.noChanges")}</p> : <><div className="change-list">{preview.changes.map((change) => <div key={change.field}><code>{fieldLabel(change.field, t)}</code><span>{change.before}</span><b aria-hidden="true">→</b><span>{change.after}</span></div>)}</div><details><summary>{t("unbound.showGenerated")}</summary><pre>{preview.rendered_config}</pre></details><button type="button" disabled={busy} onClick={applyPreview}>{busy ? t("unbound.activating") : t("unbound.validateActivate")}</button></>}
          </section>
        )}
      </section>

      <section id="unbound-panel-zones" role="tabpanel" aria-labelledby="unbound-tab-zones" hidden={activeSection !== "zones"} tabIndex={0}>
        <div className="section-introduction"><p className="unbound-eyebrow">{t("unbound.localDnsEyebrow")}</p><h2>{t("unbound.localDnsTitle")}</h2><p>{t("unbound.localDnsHelp")}</p></div>
        <UnboundGuidedZones version={history[0]?.id} onActivated={reload} />
        <UnboundPrivateDomains version={history[0]?.id} onActivated={reload} />
        <UnboundForwardZones version={history[0]?.id} onActivated={reload} />
      </section>

      <section id="unbound-panel-advanced" role="tabpanel" aria-labelledby="unbound-tab-advanced" hidden={activeSection !== "advanced"} tabIndex={0}>
        <div className="section-introduction"><p className="unbound-eyebrow">{t("unbound.advancedEyebrow")}</p><h2>{t("unbound.advancedTitle")}</h2><p>{t("unbound.advancedHelp")}</p></div>
        {liveConfig && (
          <section className="glass-card live-config-panel">
            <div className="panel-heading"><div><p className="unbound-eyebrow">LIVE · READ ONLY</p><h2>{t("unbound.liveTitle")}</h2><p className="muted-copy">{t("unbound.liveHelp")}</p></div><span className="live-config-state"><i /> {t("common.active")} · {formatDate(liveConfig.checked_at)}</span></div>
            <div className="config-file-label"><span>50-rootguard.conf</span><code>/etc/unbound/unbound.d/50-rootguard.conf</code></div>
            <details className="live-config-disclosure"><summary>{t("unbound.managedConfig")}</summary><pre>{liveConfig.managed_config}</pre></details>
            <div className="live-config-details"><details><summary>{t("unbound.baseConfig")}</summary><pre>{liveConfig.base_config}</pre></details><details><summary>{t("unbound.customConfig")}</summary><pre>{liveConfig.custom_config || t("unbound.noCustom")}</pre></details></div>
          </section>
        )}
        <UnboundExpertEditor version={history[0]?.id} onActivated={reload} />
        <section className="glass-card history-panel">
          <details className="history-disclosure">
            <summary><span><span className="unbound-eyebrow">ROLLBACK</span><strong>{t("unbound.history")}</strong></span><em>{t("unbound.versions", { count: history.length })}</em></summary>
            {history.length === 0 ? <p className="muted-copy">{t("unbound.noHistory")}</p> : <div className="history-list">{history.map((entry, index) => <article key={entry.id}><div><strong>{index === 0 ? t("unbound.latest") : t("unbound.saved")}</strong><span>{formatDate(entry.created_at)}</span><small>Threads {entry.settings.threads} · TTL {entry.settings.cache_min_ttl}–{entry.settings.cache_max_ttl} · {t("forward.historyCount", { count: entry.settings.forward_zones?.length ?? 0 })}{entry.custom_config ? " · Custom Config" : ""}</small></div><button className="secondary-action" type="button" disabled={busy || index === 0} onClick={() => restore(entry)}>{index === 0 ? t("common.active") : t("unbound.restore")}</button></article>)}</div>}
          </details>
        </section>
      </section>
    </Page>
  );
}

type UnboundSection = "overview" | "resolver" | "zones" | "advanced";

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
  const labels: Record<string, string> = { qname_minimisation: t("unbound.qname"), prefetch: "Prefetch", serve_expired: "Serve Expired", cache_min_ttl: "Minimum TTL", cache_max_ttl: "Maximum TTL", threads: t("unbound.threads"), forward_zones: t("forward.title"), private_domains: t("private.title"), reverse_zones: t("private.reverseTitle"), configuration: t("unbound.field.configuration"), resolution: t("unbound.field.resolution"), dnssec: "DNSSEC" };
  return labels[field] ?? field;
}

function settingsEqual(left: UnboundSettings, right: UnboundSettings) {
  return left.qname_minimisation === right.qname_minimisation
    && left.prefetch === right.prefetch
    && left.serve_expired === right.serve_expired
    && left.cache_min_ttl === right.cache_min_ttl
    && left.cache_max_ttl === right.cache_max_ttl
    && left.threads === right.threads;
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
