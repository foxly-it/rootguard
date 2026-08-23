import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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

const serviceIcons: Record<ServiceInfo["name"], typeof Cpu> = {
  core: Cpu,
  webapp: PanelsTopLeft,
  updater: ServerCog,
  adguard: Filter,
  unbound: ShieldCheck,
};

// How many samples the resource sparklines/gauges keep in memory - purely
// client-side, resets on page load (RootGuard has no metrics time-series
// store). At the 10s poll interval below, 24 samples covers 4 minutes,
// enough to show a meaningful trend without the chart going stale-looking
// on a rarely-refreshed tab.
const HISTORY_LENGTH = 24;

function pushHistory(previous: number[], value: number | null): number[] {
  if (value === null) return previous;
  return [...previous, value].slice(-HISTORY_LENGTH);
}

export default function Overview() {
  const { locale, t } = useI18n();
  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null);
  const [installation, setInstallation] = useState<InstallationStatus | null>(null);
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [adGuard, setAdGuard] = useState<AdGuardStatus | null>(null);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [busyService, setBusyService] = useState("");
  const [error, setError] = useState("");
  const [cpuHistory, setCpuHistory] = useState<number[]>([]);
  const [memoryHistory, setMemoryHistory] = useState<number[]>([]);
  const [queriesHistory, setQueriesHistory] = useState<number[]>([]);
  const [blockedHistory, setBlockedHistory] = useState<number[]>([]);
  const [blockRateHistory, setBlockRateHistory] = useState<number[]>([]);

  const loadDashboard = useCallback(async () => {
    try {
      const [nextDashboard, nextInstallation, nextServices] = await Promise.all([
        fetchDashboard(),
        fetchInstallationStatus(),
        fetchServices(),
      ]);
      setDashboard(nextDashboard);
      setInstallation(nextInstallation);
      setServices(nextServices);
      setCpuHistory((prev) => pushHistory(prev, nextDashboard.docker.metrics_available ? nextDashboard.docker.cpu : null));
      setMemoryHistory((prev) => pushHistory(prev, nextDashboard.docker.metrics_available ? nextDashboard.docker.memory : null));

      const nextAdGuard = nextInstallation.state === "installed" ? await fetchAdGuardStatus().catch(() => null) : null;
      setAdGuard(nextAdGuard);
      setQueriesHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.queries : null));
      setBlockedHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.blocked : null));
      setBlockRateHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? blockRatePercent(nextAdGuard.blocked, nextAdGuard.queries) : null));

      setLastChecked(new Date());
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t("overview.loadError"));
    }
  }, [t]);

  useEffect(() => {
    // A single sample can't draw a chart at all, and a real trend only
    // starts looking like one after a handful of points - at a flat 10s
    // cadence that's a genuinely long, boring wait after opening the page.
    // Front-load a quick burst of extra samples right after mount (and
    // again whenever the tab becomes visible, see below) so the metric
    // charts have a few real points within seconds instead of one; the
    // steady-state poll cadence itself is unchanged.
    const burstDelays = [2_000, 4_000, 7_000];
    let timeouts: number[] = [];
    let interval: number | null = null;

    function start() {
      loadDashboard();
      timeouts = burstDelays.map((delay) => window.setTimeout(loadDashboard, delay));
      interval = window.setInterval(loadDashboard, 10_000);
    }

    function stop() {
      timeouts.forEach(window.clearTimeout);
      timeouts = [];
      if (interval !== null) {
        window.clearInterval(interval);
        interval = null;
      }
    }

    // Backgrounded/hidden tabs have no reason to keep polling Core, Docker,
    // and AdGuard every 10s - nobody's watching the charts. Pausing there
    // and catching back up with a fresh burst on return keeps the same
    // "feels current" behavior without the wasted requests in between.
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
  }, [loadDashboard]);

  async function restart(service: ServiceInfo["name"]) {
    setBusyService(service);
    try {
      await serviceAction(service, "restart");
      await loadDashboard();
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
            <button className="rg-button rg-button-secondary overview-button ghost" type="button" onClick={loadDashboard}>
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
                  aria-label={t("common.restart")}
                  title={t("common.restart")}
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
// instead, following the cursor along the time axis (each sample is one
// 10s poll tick, oldest to newest left-to-right).
function Sparkline({ values, tone = "info", formatValue, t }: {
  values: number[];
  tone?: string;
  formatValue: (value: number) => string;
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);
  const wrapRef = useRef<HTMLDivElement>(null);
  // Coalesces rapid mousemove events (the browser can fire far more of
  // these per second than it ever repaints) down to at most one state
  // update per animation frame, instead of one React re-render per raw
  // event.
  const rafRef = useRef<number | null>(null);

  useEffect(() => () => {
    if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
  }, []);

  // Only depends on `values` (one new sample every ~10s), not on
  // hoverIndex - previously this whole path/area computation re-ran on
  // every single hover-driven re-render even though only the crosshair
  // position actually needs to change while the mouse moves.
  const chart = useMemo(() => {
    if (values.length < 2) return null;
    const max = Math.max(...values);
    const min = Math.min(...values);
    const range = max - min || 1;
    const points = values.map((value, index) => ({
      x: (index / (values.length - 1)) * 100,
      y: 27 - ((value - min) / range) * 23,
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
      setHoverIndex(Math.round(ratio * (values.length - 1)));
    });
  }

  function handleLeave() {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    setHoverIndex(null);
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

  const hovered = hoverIndex !== null ? chart.points[hoverIndex] : null;
  const secondsAgo = hoverIndex !== null ? (values.length - 1 - hoverIndex) * 10 : 0;

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
      {hovered && hoverIndex !== null && (
        <div className="sparkline-tooltip" style={{ left: `${hovered.x}%` }}>
          <strong>{formatValue(values[hoverIndex])}</strong>
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
  history: number[];
  detail: string;
  available: boolean;
  tone?: string;
  formatValue: (value: number) => string;
  t: (key: string, values?: Record<string, string | number>) => string;
}) {
  const hasRange = history.length >= 2;
  return (
    <article className={`metric-card ${available ? "available" : ""}`}>
      <div className="metric-card-head" data-tooltip={detail}>
        <span className={`metric-icon ${tone}`}>{icon}</span>
        <div className="metric-card-labels"><small>{label}</small><strong>{display}</strong></div>
      </div>
      <div className="metric-visual">
        <Sparkline values={history} tone={tone} formatValue={formatValue} t={t} />
      </div>
      <div className="metric-chart-range">
        {hasRange && (
          <>
            <span>{t("overview.chartMin", { value: formatValue(Math.min(...history)) })}</span>
            <span>{t("overview.chartMax", { value: formatValue(Math.max(...history)) })}</span>
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

function blockRatePercent(blocked: number, queries: number) {
  return queries === 0 ? 0 : (blocked / queries) * 100;
}
