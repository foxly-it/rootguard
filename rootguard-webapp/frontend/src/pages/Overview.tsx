import { useCallback, useEffect, useMemo, useState } from "react";
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
  const [memoryHistory, setMemoryHistory] = useState<number[]>([]);
  const [queriesHistory, setQueriesHistory] = useState<number[]>([]);
  const [blockedHistory, setBlockedHistory] = useState<number[]>([]);

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
      setMemoryHistory((prev) => pushHistory(prev, nextDashboard.docker.metrics_available ? nextDashboard.docker.memory : null));

      const nextAdGuard = nextInstallation.state === "installed" ? await fetchAdGuardStatus().catch(() => null) : null;
      setAdGuard(nextAdGuard);
      setQueriesHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.queries : null));
      setBlockedHistory((prev) => pushHistory(prev, nextAdGuard?.stats_available ? nextAdGuard.blocked : null));

      setLastChecked(new Date());
      setError("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t("overview.loadError"));
    }
  }, [t]);

  useEffect(() => {
    const initialLoad = window.setTimeout(loadDashboard, 0);
    const interval = window.setInterval(loadDashboard, 10_000);
    return () => {
      window.clearTimeout(initialLoad);
      window.clearInterval(interval);
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
        <GaugeMetric
          icon={<Cpu />}
          label={t("overview.cpu")}
          display={dashboard?.docker.metrics_available ? formatCPU(dashboard.docker.cpu) : t("overview.metricUnavailable")}
          percent={dashboard?.docker.metrics_available ? dashboard.docker.cpu : 0}
          detail={t("overview.cpuHelp")}
          available={dashboard?.docker.metrics_available === true}
        />
        <SparkMetric
          icon={<MemoryStick />}
          label={t("overview.memory")}
          display={dashboard?.docker.metrics_available ? formatBytes(dashboard.docker.memory) : t("overview.metricUnavailable")}
          history={memoryHistory}
          detail={t("overview.memoryHelp")}
          available={dashboard?.docker.metrics_available === true}
        />
        <SparkMetric
          icon={<Activity />}
          label={t("overview.queries")}
          display={adGuard?.stats_available ? formatInteger(adGuard.queries) : t("overview.metricUnavailable")}
          history={queriesHistory}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
        />
        <SparkMetric
          icon={<Ban />}
          label={t("overview.blocked")}
          display={adGuard?.stats_available ? formatInteger(adGuard.blocked) : t("overview.metricUnavailable")}
          history={blockedHistory}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
          tone="danger"
        />
        <GaugeMetric
          icon={<Filter />}
          label={t("overview.blockRate")}
          display={adGuard?.stats_available ? formatBlockRate(adGuard.blocked, adGuard.queries) : t("overview.metricUnavailable")}
          percent={adGuard?.stats_available ? blockRatePercent(adGuard.blocked, adGuard.queries) : 0}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
          tone="accent"
        />
      </section>

      <section className="overview-panel system-panel">
        <PanelHeading eyebrow={t("overview.dataFlow")} title={t("overview.flowTitle")} link="/adguard" linkLabel={t("overview.details")} />
        <div className="dns-flow">
          <FlowNode icon={<Globe2 />} title={t("overview.clients")} detail={bindAddress} state="neutral" />
          <FlowArrow />
          <FlowNode icon={<Filter />} title="AdGuard Home" detail={t("overview.filters")} state={serviceState(services, "adguard")} />
          <FlowArrow />
          <FlowNode icon={<ShieldCheck />} title="Unbound" detail={t("overview.recursive")} state={serviceState(services, "unbound")} />
          <FlowArrow />
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
              <article className={`service-row ${runtimeTone(service)}`} key={service.name}>
                <span className="service-row-icon"><Icon size={16} /></span>
                <div className="service-row-detail">
                  <strong>{service.displayName}</strong>
                  <span className="service-row-status">{healthLabel(service, t)}</span>
                </div>
                <button
                  className="service-row-restart"
                  type="button"
                  disabled={service.status !== "running" || busyService === service.name}
                  onClick={() => restart(service.name)}
                  aria-label={t("common.restart")}
                  title={t("common.restart")}
                >
                  <RefreshCw size={14} className={busyService === service.name ? "spinning" : ""} />
                </button>
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

// RadialGauge/Sparkline render a bounded percentage and an unbounded
// rolling count respectively - the same visual split OPNsense/Proxmox use
// (a ring for "how full is this out of 100") vs. what AdGuard Home's own
// dashboard uses (a trend line for "how many, over time").
function RadialGauge({ percent, size = 38, strokeWidth = 5, tone = "info" }: {
  percent: number;
  size?: number;
  strokeWidth?: number;
  tone?: "info" | "accent" | "warning" | "danger";
}) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const clamped = Math.max(0, Math.min(100, percent));
  const offset = circumference * (1 - clamped / 100);
  const center = size / 2;
  return (
    <svg className={`radial-gauge ${tone}`} width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden="true">
      <circle className="gauge-track" cx={center} cy={center} r={radius} strokeWidth={strokeWidth} fill="none" />
      <circle
        className="gauge-value"
        cx={center}
        cy={center}
        r={radius}
        strokeWidth={strokeWidth}
        fill="none"
        strokeDasharray={circumference}
        strokeDashoffset={offset}
        strokeLinecap="round"
        transform={`rotate(-90 ${center} ${center})`}
      />
    </svg>
  );
}

function Sparkline({ values, tone = "info" }: { values: number[]; tone?: string }) {
  if (values.length < 2) return <svg className={`sparkline ${tone}`} viewBox="0 0 100 32" aria-hidden="true" />;
  const max = Math.max(...values);
  const min = Math.min(...values);
  const range = max - min || 1;
  const points = values.map((value, index) => {
    const x = (index / (values.length - 1)) * 100;
    const y = 30 - ((value - min) / range) * 26;
    return `${x},${y}`;
  });
  const linePath = `M${points.join(" L")}`;
  const areaPath = `${linePath} L100,32 L0,32 Z`;
  return (
    <svg className={`sparkline ${tone}`} viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden="true">
      <path className="sparkline-area" d={areaPath} />
      <path className="sparkline-line" d={linePath} fill="none" />
    </svg>
  );
}

function GaugeMetric({ icon, label, display, percent, detail, available, tone = "info" }: {
  icon: React.ReactNode;
  label: string;
  display: string;
  percent: number;
  detail: string;
  available: boolean;
  tone?: "info" | "accent" | "warning" | "danger";
}) {
  return (
    <article className={`metric-card ${available ? "available" : ""}`}>
      <div className="metric-card-head">
        <span className={`metric-icon ${tone}`}>{icon}</span>
        <div className="metric-card-labels"><small>{label}</small><strong>{display}</strong></div>
      </div>
      <div className="metric-visual metric-visual-gauge" data-tooltip={detail}>
        <span className="gauge-wrap">
          <RadialGauge percent={available ? percent : 0} tone={tone} />
          <em>{available ? Math.round(percent) : "–"}{available && "%"}</em>
        </span>
      </div>
    </article>
  );
}

function SparkMetric({ icon, label, display, history, detail, available, tone = "info" }: {
  icon: React.ReactNode;
  label: string;
  display: string;
  history: number[];
  detail: string;
  available: boolean;
  tone?: string;
}) {
  return (
    <article className={`metric-card ${available ? "available" : ""}`}>
      <div className="metric-card-head">
        <span className={`metric-icon ${tone}`}>{icon}</span>
        <div className="metric-card-labels"><small>{label}</small><strong>{display}</strong></div>
      </div>
      <div className="metric-visual" data-tooltip={detail}>
        <Sparkline values={history} tone={tone} />
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

function FlowArrow() {
  return <div className="flow-arrow"><span /><ArrowRight size={15} /></div>;
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
