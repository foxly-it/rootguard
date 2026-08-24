import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { Link } from "react-router";
import {
  Activity,
  ArrowRight,
  Ban,
  Check,
  Cpu,
  Filter,
  Globe2,
  MemoryStick,
  Network,
  PanelsTopLeft,
  RefreshCw,
  Server,
  ServerCog,
  ShieldCheck,
} from "lucide-react";
import {
  fetchAdGuardStatus,
  fetchDashboard,
  fetchInstallationStatus,
  fetchServices,
  serviceAction,
  type AdGuardStatus,
  type DashboardResponse,
  type InstallationStatus,
  type ServiceInfo,
} from "../api/client";
import "../styles/dashboard.css";
import { useI18n } from "../i18n";
import { healthLabel, runtimeTone } from "../utils/serviceHealth";
import { blockRatePercent, pushHistory, type HistoryPoint } from "../utils/metrics";

const serviceIcons: Record<ServiceInfo["name"], typeof Cpu> = {
  core: Cpu,
  webapp: PanelsTopLeft,
  updater: ServerCog,
  adguard: Filter,
  unbound: ShieldCheck,
};

// How many samples the resource sparklines keep in memory - purely
// client-side, resets on page load (RootGuard has no metrics time-series
// store). At the 1s poll interval below, 60 samples covers a full minute
// of live history - short enough to stay dense/readable at this refresh
// rate instead of flattening into a long, mostly-empty-looking line.
const HISTORY_LENGTH = 60;

export default function Overview() {
  const { locale, t } = useI18n();
  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null);
  const [installation, setInstallation] = useState<InstallationStatus | null>(null);
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [adGuard, setAdGuard] = useState<AdGuardStatus | null>(null);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [busyService, setBusyService] = useState("");
  const [error, setError] = useState("");
  const [cpuHistory, setCpuHistory] = useState<HistoryPoint[]>([]);
  const [memoryHistory, setMemoryHistory] = useState<HistoryPoint[]>([]);
  const [queriesHistory, setQueriesHistory] = useState<HistoryPoint[]>([]);
  const [blockedHistory, setBlockedHistory] = useState<HistoryPoint[]>([]);
  const [blockRateHistory, setBlockRateHistory] = useState<HistoryPoint[]>([]);

  // Tracks the latest known installation state outside React state so
  // loadMetrics (see below) can decide whether to also fetch AdGuard stats
  // without re-fetching installation status itself every time - that's
  // loadStatus's job, on its own slower cadence.
  const installationRef = useRef<InstallationStatus | null>(null);

  // Every poll cycle can have several requests in flight at once (the
  // startup burst, the steady interval, a manual refresh, a post-restart
  // reload) with no guarantee they resolve in the order they were sent. A
  // sequence counter per loader ensures only the most recently *started*
  // request's response is ever applied - a slow, stale response finishing
  // after a newer one can no longer clobber fresher data, reset
  // lastChecked to the wrong time, or resurrect an error that already
  // cleared.
  const metricsSeq = useRef(0);
  const statusSeq = useRef(0);
  // Core's own /api/dashboard handler shells out to `docker stats
  // --no-stream` on every call, which the Docker daemon itself takes
  // ~1-2s to answer (it needs two internal CPU-accounting samples to
  // compute a delta) - measured live, consistently ~2s. At a 1s poll
  // interval that means a request is essentially always still in flight
  // when the next tick fires. The sequence guard above only decides which
  // *result* wins; without this separate in-flight check, every tick's
  // request gets superseded by the next one before it can ever complete,
  // and the metrics never populate at all (reproduced live: "Nicht
  // verfügbar" indefinitely). Skipping a tick while the previous request
  // is still outstanding lets it actually finish instead.
  const metricsInFlight = useRef(false);

  const loadMetrics = useCallback(async () => {
    if (metricsInFlight.current) return;
    metricsInFlight.current = true;
    const seq = ++metricsSeq.current;
    try {
      // These two were previously sequential (dashboard, then AdGuard) for
      // no real reason - they don't depend on each other, so awaiting them
      // one after another just adds their latencies together instead of
      // taking the slower of the two. Running them in parallel roughly
      // halves how long every single poll tick takes to land.
      const wantsAdGuard = installationRef.current?.state === "installed";
      const [nextDashboard, nextAdGuard] = await Promise.all([
        fetchDashboard(),
        wantsAdGuard ? fetchAdGuardStatus().catch(() => null) : Promise.resolve<AdGuardStatus | null>(null),
      ]);
      if (seq !== metricsSeq.current) return;

      setDashboard(nextDashboard);
      setCpuHistory((prev) => pushHistory(prev, nextDashboard.docker.metrics_available ? nextDashboard.docker.cpu : null, HISTORY_LENGTH));
      setMemoryHistory((prev) => pushHistory(prev, nextDashboard.docker.metrics_available ? nextDashboard.docker.memory : null, HISTORY_LENGTH));
      setAdGuard(nextAdGuard);
      setQueriesHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.queries : null, HISTORY_LENGTH));
      setBlockedHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.blocked : null, HISTORY_LENGTH));
      setBlockRateHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? blockRatePercent(nextAdGuard.blocked, nextAdGuard.queries) : null, HISTORY_LENGTH));
      setLastChecked(new Date());
      setError("");
    } catch (cause) {
      if (seq !== metricsSeq.current) return;
      setError(cause instanceof Error ? cause.message : t("overview.loadError"));
    } finally {
      metricsInFlight.current = false;
    }
  }, [t]);

  const loadStatus = useCallback(async () => {
    const seq = ++statusSeq.current;
    try {
      const [nextInstallation, nextServices] = await Promise.all([
        fetchInstallationStatus(),
        fetchServices(),
      ]);
      if (seq !== statusSeq.current) return;
      installationRef.current = nextInstallation;
      setInstallation(nextInstallation);
      setServices(nextServices);
      setError("");
    } catch (cause) {
      if (seq !== statusSeq.current) return;
      setError(cause instanceof Error ? cause.message : t("overview.loadError"));
    }
  }, [t]);

  const refreshAll = useCallback(async () => {
    await Promise.all([loadStatus(), loadMetrics()]);
  }, [loadStatus, loadMetrics]);

  useEffect(() => {
    // At a 1s cadence the interval itself already delivers a fresh sample
    // about as fast as a burst ever could, so the earlier startup-burst
    // scheme (extra timeouts at 2/4/7s to front-load a few samples before
    // the old, much slower 10s interval caught up) is redundant now and
    // was removed - one immediate call plus the interval is already fast.
    let metricsInterval: number | null = null;
    let statusInterval: number | null = null;

    function start() {
      loadStatus();
      loadMetrics();
      metricsInterval = window.setInterval(loadMetrics, 1_000);
      // Service/installation status changes far less often than CPU/memory/
      // query counts - a slower cadence is plenty fresh for it and avoids
      // hitting Core for a full service/Docker inspect every single second.
      statusInterval = window.setInterval(loadStatus, 20_000);
    }

    function stop() {
      if (metricsInterval !== null) {
        window.clearInterval(metricsInterval);
        metricsInterval = null;
      }
      if (statusInterval !== null) {
        window.clearInterval(statusInterval);
        statusInterval = null;
      }
    }

    // Backgrounded/hidden tabs have no reason to keep polling Core, Docker,
    // and AdGuard every second - nobody's watching the charts. Pausing
    // there and firing an immediate load on return keeps the same "feels
    // current" behavior without the wasted requests in between.
    function handleVisibilityChange() {
      if (document.hidden) stop();
      else start();
    }

    if (!document.hidden) start();
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      stop();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [loadMetrics, loadStatus]);

  async function restart(service: ServiceInfo["name"]) {
    setBusyService(service);
    try {
      await serviceAction(service, "restart");
      await refreshAll();
    } finally {
      setBusyService("");
    }
  }

  const runningServices = services.filter((service) => service.status === "running").length;
  const protectedState = installation?.state === "installed"
    && dashboard?.dns.status === "healthy"
    && adGuard?.upstream_ready === true;
  const bindAddress = installation?.config
    ? `${installation.config.dns_bind_address}:${installation.config.dns_port}`
    : t("overview.notConfigured");

  const headline = useMemo(() => {
    if (installation?.state === "deploying") return t("overview.headline.deploying");
    if (protectedState) return t("overview.headline.protected");
    if (installation?.state === "installed") return t("overview.headline.attention");
    return t("overview.headline.waiting");
  }, [installation?.state, protectedState, t]);

  return (
    <div className="overview-page">
      <section className={`overview-hero ${protectedState ? "healthy" : "attention"}`}>
        <div className="hero-copy">
          <span className="overview-eyebrow">ROOTGUARD CONTROL PANEL</span>
          <h1>{headline}</h1>
          <p>
            {protectedState
              ? t("overview.protectedText")
              : t("overview.waitingText")}
          </p>
          <div className="hero-actions">
            <Link className="rg-button rg-button-primary overview-button primary" to={installation?.state === "installed" ? "/unbound" : "/setup"}>
              {installation?.state === "installed" ? t("overview.configure") : t("overview.openSetup")}
              <ArrowRight size={16} />
            </Link>
            <button className="rg-button rg-button-secondary overview-button ghost" type="button" onClick={refreshAll}>
              <RefreshCw size={15} />
              {t("overview.refresh")}
            </button>
          </div>
        </div>
        <div className="protection-orbit" aria-label={protectedState ? t("overview.protected") : t("overview.unprotected")}>
          <span className="orbit-ring outer" />
          <span className="orbit-ring inner" />
          <div className="orbit-core"><ShieldCheck size={44} /></div>
          <strong>{protectedState ? "PROTECTED" : "CHECK"}</strong>
          <small>{bindAddress}</small>
        </div>

        <div className="hero-stats">
          <HeroStat icon={<Network />} label={t("overview.endpoint")} value={bindAddress} good={protectedState} />
          <HeroStat icon={<Server />} label={t("overview.services")} value={t("overview.servicesValue", { count: runningServices, total: services.length })} good={services.length > 0 && runningServices === services.length} />
          <HeroStat icon={<ShieldCheck />} label="DNSSEC" value={dashboard?.dns.dnssec ? t("overview.validationActive") : t("overview.unavailable")} good={dashboard?.dns.dnssec === true} />
          <HeroStat icon={<Filter />} label={t("overview.filterChain")} value={adGuard?.upstream_ready ? t("overview.connected") : t("overview.checkRequired")} good={adGuard?.upstream_ready === true} />
        </div>
      </section>

      {error && <div className="overview-error" role="alert">{error}</div>}

      <section className="overview-resources" aria-label={t("overview.resources")}>
        <SparkMetric
          icon={<Cpu />}
          label={t("overview.cpu")}
          display={dashboard?.docker.metrics_available ? formatCPU(dashboard.docker.cpu) : t("overview.metricUnavailable")}
          history={cpuHistory}
          detail={t("overview.cpuHelp")}
          available={dashboard?.docker.metrics_available === true}
          formatValue={formatCPU}
          t={t}
        />
        <SparkMetric
          icon={<MemoryStick />}
          label={t("overview.memory")}
          display={dashboard?.docker.metrics_available ? formatBytes(dashboard.docker.memory) : t("overview.metricUnavailable")}
          history={memoryHistory}
          detail={t("overview.memoryHelp")}
          available={dashboard?.docker.metrics_available === true}
          formatValue={formatBytes}
          t={t}
        />
        <SparkMetric
          icon={<Activity />}
          label={t("overview.queries")}
          display={adGuard?.stats_available ? formatInteger(adGuard.queries) : t("overview.metricUnavailable")}
          history={queriesHistory}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
          formatValue={formatInteger}
          t={t}
        />
        <SparkMetric
          icon={<Ban />}
          label={t("overview.blocked")}
          display={adGuard?.stats_available ? formatInteger(adGuard.blocked) : t("overview.metricUnavailable")}
          history={blockedHistory}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
          tone="danger"
          formatValue={formatInteger}
          t={t}
        />
        <SparkMetric
          icon={<Filter />}
          label={t("overview.blockRate")}
          display={adGuard?.stats_available ? formatBlockRate(adGuard.blocked, adGuard.queries) : t("overview.metricUnavailable")}
          history={blockRateHistory}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
          tone="accent"
          formatValue={formatCPU}
          t={t}
        />
      </section>

      <section className="overview-panel system-panel">
        <PanelHeading eyebrow={t("overview.dataFlow")} title={t("overview.flowTitle")} link="/adguard" linkLabel={t("overview.details")} />
        <div className="dns-flow">
          <FlowNode icon={<Globe2 />} title={t("overview.clients")} detail={bindAddress} state="neutral" />
          <FlowArrow index={0} />
          <FlowNode icon={<Filter />} title="AdGuard Home" detail={t("overview.filters")} state={serviceState(services, "adguard")} />
          <FlowArrow index={1} />
          <FlowNode icon={<ShieldCheck />} title="Unbound" detail={t("overview.recursive")} state={serviceState(services, "unbound")} />
          <FlowArrow index={2} />
          <FlowNode icon={<Network />} title={t("overview.hierarchy")} detail={t("overview.noExternal")} state={protectedState ? "running" : "neutral"} />
        </div>
        <div className="flow-footnote">
          <Check size={15} />
          {t("overview.privateAdmin")}
        </div>

        <div className="system-panel-divider" />

        <PanelHeading eyebrow={t("overview.runtime")} title={t("overview.dnsServices")} />
        <div className="dashboard-services">
          {services.map((service) => {
            const Icon = serviceIcons[service.name];
            return (
              <article className={`service-card ${runtimeTone(service)}`} key={service.name}>
                <button
                  className="service-card-restart"
                  type="button"
                  disabled={service.status !== "running" || busyService === service.name}
                  onClick={() => restart(service.name)}
                  aria-label={t("overview.restartService", { name: service.displayName })}
                  title={t("overview.restartService", { name: service.displayName })}
                >
                  <RefreshCw size={13} className={busyService === service.name ? "spinning" : ""} />
                </button>
                <span className="service-card-icon"><Icon size={19} /></span>
                <strong>{service.displayName}</strong>
                <span className="service-card-status">{healthLabel(service, t)}</span>
              </article>
            );
          })}
        </div>

        <div className="panel-footer">
          <span>{t("overview.lastChecked", { time: lastChecked ? lastChecked.toLocaleTimeString(locale) : "–" })}</span>
          <Link to="/setup">{t("overview.manageStack")} <ArrowRight size={14} /></Link>
        </div>
      </section>
    </div>
  );
}

function HeroStat({ icon, label, value, good }: {
  icon: React.ReactNode;
  label: string;
  value: string;
  good?: boolean;
}) {
  return (
    <article className="hero-stat">
      <span className={`hero-stat-icon ${good ? "good" : ""}`}>{icon}</span>
      <div><small>{label}</small><strong>{value}</strong></div>
    </article>
  );
}

// Baseline sits at y=29 (not the viewBox edge) so the axis line itself
// stays fully visible with a stroke instead of clipping against the SVG
// bounds - this is the chart's x-axis; the Min/Max caption under it is the
// y-axis reference, since cramming that into this tiny an SVG reads worse
// than plain text. Hovering shows the exact value at the nearest sample
// instead, following the cursor along the time axis. Each point carries its
// own real sampledAt timestamp rather than an assumed uniform interval -
// the startup burst, manual refreshes, and post-restart reloads all sample
// at irregular gaps, which a fixed "N * 10s" calculation would render as a
// confidently wrong age.
function Sparkline({ values, tone = "info", formatValue, t }: {
  values: HistoryPoint[];
  tone?: string;
  formatValue: (value: number) => string;
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  // `now` is captured alongside the index at the moment of the pointer
  // event (not read via Date.now() during render, which the render-purity
  // lint rule correctly rejects as producing a value that silently drifts
  // between renders without any actual input changing).
  const [hover, setHover] = useState<{ index: number; now: number } | null>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  // Coalesces rapid mousemove events (the browser can fire far more of
  // these per second than it ever repaints) down to at most one state
  // update per animation frame, instead of one React re-render per raw
  // event.
  const rafRef = useRef<number | null>(null);

  useEffect(() => () => {
    if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
  }, []);

  // Only depends on `values` (one new sample every ~1s), not on hover
  // state - previously this whole path/area computation re-ran on
  // every single hover-driven re-render even though only the crosshair
  // position actually needs to change while the mouse moves.
  const chart = useMemo(() => {
    if (values.length < 2) return null;
    const raw = values.map((point) => point.value);
    const max = Math.max(...raw);
    const min = Math.min(...raw);
    const range = max - min || 1;
    const points = values.map((point, index) => ({
      x: (index / (values.length - 1)) * 100,
      y: 27 - ((point.value - min) / range) * 23,
    }));
    const linePath = `M${points.map(({ x, y }) => `${x},${y}`).join(" L")}`;
    const areaPath = `${linePath} L100,29 L0,29 Z`;
    return { points, linePath, areaPath };
  }, [values]);

  function handleMove(event: React.MouseEvent<HTMLDivElement>) {
    if (!chart || !wrapRef.current || rafRef.current !== null) return;
    const rect = wrapRef.current.getBoundingClientRect();
    const clientX = event.clientX;
    rafRef.current = requestAnimationFrame(() => {
      rafRef.current = null;
      const ratio = Math.min(1, Math.max(0, (clientX - rect.left) / rect.width));
      setHover({ index: Math.round(ratio * (values.length - 1)), now: Date.now() });
    });
  }

  function handleLeave() {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    setHover(null);
  }

  if (!chart) {
    return (
      <div className="sparkline-wrap">
        <svg className={`sparkline ${tone}`} viewBox="0 0 100 32" aria-hidden="true">
          <line className="sparkline-axis" x1="0" y1="29" x2="100" y2="29" />
        </svg>
      </div>
    );
  }

  const hoveredPoint = hover ? values[hover.index] : null;
  const hovered = hover ? chart.points[hover.index] : null;
  const secondsAgo = hover && hoveredPoint ? Math.max(0, Math.round((hover.now - hoveredPoint.sampledAt) / 1000)) : 0;

  return (
    <div
      className="sparkline-wrap"
      ref={wrapRef}
      onMouseMove={handleMove}
      onMouseLeave={handleLeave}
    >
      <svg className={`sparkline ${tone}`} viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden="true">
        <path className="sparkline-area" d={chart.areaPath} />
        <line className="sparkline-axis" x1="0" y1="29" x2="100" y2="29" vectorEffect="non-scaling-stroke" />
        <path className="sparkline-line" d={chart.linePath} fill="none" vectorEffect="non-scaling-stroke" />
        {hovered && (
          <>
            <line className="sparkline-crosshair" x1={hovered.x} x2={hovered.x} y1="0" y2="29" vectorEffect="non-scaling-stroke" />
            <circle className="sparkline-dot" cx={hovered.x} cy={hovered.y} r="2.4" vectorEffect="non-scaling-stroke" />
          </>
        )}
      </svg>
      {hovered && hoveredPoint && (
        <div className="sparkline-tooltip" style={{ left: `${hovered.x}%` }}>
          <strong>{formatValue(hoveredPoint.value)}</strong>
          <small>{secondsAgo === 0 ? t("overview.chartNow") : t("overview.chartSecondsAgo", { seconds: secondsAgo })}</small>
        </div>
      )}
    </div>
  );
}

function SparkMetric({ icon, label, display, history, detail, available, tone = "info", formatValue, t }: {
  icon: React.ReactNode;
  label: string;
  display: string;
  history: HistoryPoint[];
  detail: string;
  available: boolean;
  tone?: string;
  formatValue: (value: number) => string;
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  const hasRange = history.length >= 2;
  const detailId = useId();
  return (
    <article className={`metric-card ${available ? "available" : ""}`}>
      {/* tabIndex makes this reachable/focusable for keyboard users (the
          existing [data-tooltip]:focus-visible CSS rule could never fire on
          a plain, non-interactive div); aria-describedby ties it to a
          real (if visually hidden) text node instead of relying on
          screen readers picking up CSS-generated ::after content from
          data-tooltip, which they don't reliably do. */}
      <div className="metric-card-head" data-tooltip={detail} tabIndex={0} aria-describedby={detailId}>
        <span className={`metric-icon ${tone}`}>{icon}</span>
        <div className="metric-card-labels"><small>{label}</small><strong>{display}</strong></div>
        <span id={detailId} className="sr-only">{detail}</span>
      </div>
      <div className="metric-visual">
        <Sparkline values={history} tone={tone} formatValue={formatValue} t={t} />
      </div>
      <div className="metric-chart-range">
        {hasRange && (
          <>
            <span>{t("overview.chartMin", { value: formatValue(Math.min(...history.map((point) => point.value))) })}</span>
            <span>{t("overview.chartMax", { value: formatValue(Math.max(...history.map((point) => point.value))) })}</span>
          </>
        )}
      </div>
    </article>
  );
}

function PanelHeading({ eyebrow, title, link, linkLabel }: { eyebrow: string; title: string; link?: string; linkLabel?: string }) {
  return (
    <div className="overview-panel-heading">
      <div><span>{eyebrow}</span><h2>{title}</h2></div>
      {link && <Link to={link}>{linkLabel} <ArrowRight size={14} /></Link>}
    </div>
  );
}

function FlowNode({ icon, title, detail, state }: {
  icon: React.ReactNode;
  title: string;
  detail: string;
  state: "running" | "stopped" | "neutral";
}) {
  return <div className={`flow-node ${state}`}><span>{icon}</span><strong>{title}</strong><small>{detail}</small></div>;
}

function FlowArrow({ index }: { index: number }) {
  return (
    <div className="flow-arrow" style={{ "--flow-delay": `${index * 0.5}s` } as React.CSSProperties}>
      <span />
      <ArrowRight size={15} />
    </div>
  );
}

function serviceState(services: ServiceInfo[], name: ServiceInfo["name"]) {
  return services.find((service) => service.name === name)?.status ?? "stopped";
}

function formatCPU(value: number) {
  return `${value.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} %`;
}

function formatBytes(value: number) {
  const units = ["B", "KiB", "MiB", "GiB"];
  let amount = value;
  let unit = 0;
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024;
    unit += 1;
  }
  return `${amount.toLocaleString(undefined, { minimumFractionDigits: unit > 1 ? 1 : 0, maximumFractionDigits: 1 })} ${units[unit]}`;
}

function formatInteger(value: number) {
  return value.toLocaleString();
}

function formatBlockRate(blocked: number, queries: number) {
  return `${blockRatePercent(blocked, queries).toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} %`;
}
