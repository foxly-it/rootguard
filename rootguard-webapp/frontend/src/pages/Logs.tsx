import { useCallback, useEffect, useMemo, useState } from "react";
import { Download, FileSearch, RefreshCw, Search, ShieldCheck, Stethoscope } from "lucide-react";
import { Link, useSearchParams } from "react-router";
import { fetchServiceLogs, fetchServices, type ServiceInfo, type ServiceLogs } from "../api/client";
import { useI18n } from "../i18n";
import "../styles/logs.css";

type Level = "all" | "error" | "warning" | "info";
const serviceOrder: ServiceInfo["name"][] = ["core", "webapp", "updater", "adguard", "unbound"];

export default function Logs() {
  const { t } = useI18n();
  const [params, setParams] = useSearchParams();
  const requested = params.get("service") as ServiceInfo["name"] | null;
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [selected, setSelected] = useState<ServiceInfo["name"]>(serviceOrder.includes(requested as ServiceInfo["name"]) ? requested! : "core");
  const [logs, setLogs] = useState<ServiceLogs | null>(null);
  const [query, setQuery] = useState("");
  const [level, setLevel] = useState<Level>("all");
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async (service = selected) => {
    setLoading(true);
    try {
      setLogs(await fetchServiceLogs(service));
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t("logs.loadError"));
    } finally {
      setLoading(false);
    }
  }, [selected, t]);

  useEffect(() => {
    const initial = window.setTimeout(async () => {
      try {
        const available = await fetchServices();
        setServices(available.sort((left, right) => serviceOrder.indexOf(left.name) - serviceOrder.indexOf(right.name)));
        if (!available.some((service) => service.name === selected) && available[0]) setSelected(available[0].name);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : t("logs.loadError"));
      }
    }, 0);
    return () => window.clearTimeout(initial);
  }, [selected, t]);

  useEffect(() => {
    const initial = window.setTimeout(() => load(), 0);
    if (!autoRefresh) return () => window.clearTimeout(initial);
    const interval = window.setInterval(() => load(), 10_000);
    return () => { window.clearTimeout(initial); window.clearInterval(interval); };
  }, [autoRefresh, load]);

  const visibleLines = useMemo(() => (logs?.lines ?? []).filter((line) => {
    const normalized = line.toLowerCase();
    if (query && !normalized.includes(query.toLowerCase())) return false;
    if (level === "error") return /\b(error|fatal|panic|failed|failure)\b/i.test(line);
    if (level === "warning") return /\b(warn|warning)\b/i.test(line);
    if (level === "info") return !/\b(error|fatal|panic|failed|failure|warn|warning)\b/i.test(line);
    return true;
  }), [level, logs, query]);

  function selectService(name: ServiceInfo["name"]) {
    setSelected(name);
    setParams({ service: name }, { replace: true });
    setQuery("");
    setLevel("all");
  }

  function downloadReport() {
    if (!logs) return;
    const report = [
      "RootGuard privacy-safe diagnostic report",
      `Service: ${logs.service}`,
      `Generated: ${new Date().toISOString()}`,
      `Window: ${logs.since}; tail: ${logs.tail}; truncated: ${logs.truncated}; redacted: ${logs.redacted}`,
      "",
      ...visibleLines,
    ].join("\n");
    const href = URL.createObjectURL(new Blob([report], { type: "text/plain;charset=utf-8" }));
    const anchor = document.createElement("a");
    anchor.href = href;
    anchor.download = `rootguard-${logs.service}-diagnostics.txt`;
    anchor.click();
    URL.revokeObjectURL(href);
  }

  return (
    <div className="logs-page">
      <section className="logs-hero">
        <div><span className="stack-eyebrow">{t("logs.eyebrow")}</span><h1>{t("logs.title")}</h1><p>{t("logs.intro")}</p></div>
        <button className="rg-button rg-button-secondary" type="button" disabled={loading} onClick={() => load()}><RefreshCw className={loading ? "spin" : ""} size={16} /> {t("common.refresh")}</button>
      </section>

      {error && <div className="stack-feedback error" role="alert">{error}</div>}

      <section className="logs-service-picker" aria-label={t("logs.services")}>
        {services.map((service) => <button key={service.name} type="button" className={selected === service.name ? "active" : ""} onClick={() => selectService(service.name)}>
          <span className={`logs-health ${service.status === "running" && service.health !== "unhealthy" ? "healthy" : ""}`} />
          <span><strong>{service.displayName}</strong><small>{service.status === "running" ? t("stack.running") : t("stack.stopped")}</small></span>
        </button>)}
      </section>

      <section className="logs-console">
        <div className="logs-toolbar">
          <label className="logs-search"><Search size={15} /><span className="sr-only">{t("logs.search")}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("logs.searchPlaceholder")} /></label>
          <label><span>{t("logs.level")}</span><select value={level} onChange={(event) => setLevel(event.target.value as Level)}><option value="all">{t("logs.levelAll")}</option><option value="error">{t("logs.levelError")}</option><option value="warning">{t("logs.levelWarning")}</option><option value="info">{t("logs.levelInfo")}</option></select></label>
          <label className="logs-auto-refresh"><input type="checkbox" checked={autoRefresh} onChange={(event) => setAutoRefresh(event.target.checked)} /> <span>{t("logs.autoRefresh")}</span></label>
          <button className="rg-button rg-button-secondary" type="button" disabled={!logs} onClick={downloadReport}><Download size={15} /> {t("logs.download")}</button>
        </div>
        <div className="logs-meta"><span><ShieldCheck size={14} /> {t("logs.privacy")}</span><span>{t("logs.visible", { count: visibleLines.length, total: logs?.lines.length ?? 0 })}</span></div>
        <pre tabIndex={0} aria-label={t("logs.console")}>{visibleLines.length ? visibleLines.join("\n") : t("logs.empty")}</pre>
        {logs?.truncated && <small className="logs-truncated">{t("logs.truncated")}</small>}
      </section>

      <section className="logs-diagnostics">
        <Stethoscope size={22} /><div><strong>{t("logs.diagnosticsTitle")}</strong><p>{t("logs.diagnosticsIntro")}</p></div>
        <Link className="rg-button rg-button-secondary" to="/unbound#unbound-section-overview-diagnostics"><FileSearch size={15} /> {t("logs.openDiagnostics")}</Link>
      </section>
    </div>
  );
}
