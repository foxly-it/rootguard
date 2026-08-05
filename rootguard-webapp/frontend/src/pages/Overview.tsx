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
  RefreshCw,
  Server,
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

export default function Overview() {
  const { locale, t } = useI18n();
  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null);
  const [installation, setInstallation] = useState<InstallationStatus | null>(null);
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [adGuard, setAdGuard] = useState<AdGuardStatus | null>(null);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [busyService, setBusyService] = useState("");
  const [error, setError] = useState("");

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
      if (nextInstallation.state === "installed") {
        setAdGuard(await fetchAdGuardStatus().catch(() => null));
      } else {
        setAdGuard(null);
      }
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
          <span className="overview-eyebrow">ROOTGUARD CONTROL PLANE</span>
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
      </section>

      {error && <div className="overview-error">{error}</div>}

      <section className="overview-kpis">
        <KpiCard icon={<Network />} label={t("overview.endpoint")} value={bindAddress} note={t("overview.endpointNote")} />
        <KpiCard icon={<Server />} label={t("overview.services")} value={t("overview.servicesValue", { count: runningServices, total: services.length })} note={t("overview.servicesNote")} good={services.length > 0 && runningServices === services.length} />
        <KpiCard icon={<ShieldCheck />} label="DNSSEC" value={dashboard?.dns.dnssec ? t("overview.validationActive") : t("overview.unavailable")} note={t("overview.dnssecNote")} good={dashboard?.dns.dnssec === true} />
        <KpiCard icon={<Filter />} label={t("overview.filterChain")} value={adGuard?.upstream_ready ? t("overview.connected") : t("overview.checkRequired")} note={t("overview.noFallback")} good={adGuard?.upstream_ready === true} />
      </section>

      <section className="overview-resources" aria-label={t("overview.resources")}>
        <ResourceCard
          icon={<Cpu />}
          label={t("overview.cpu")}
          value={dashboard?.docker.metrics_available ? formatCPU(dashboard.docker.cpu) : t("overview.metricUnavailable")}
          detail={t("overview.cpuHelp")}
          available={dashboard?.docker.metrics_available === true}
        />
        <ResourceCard
          icon={<MemoryStick />}
          label={t("overview.memory")}
          value={dashboard?.docker.metrics_available ? formatBytes(dashboard.docker.memory) : t("overview.metricUnavailable")}
          detail={t("overview.memoryHelp")}
          available={dashboard?.docker.metrics_available === true}
        />
        <ResourceCard
          icon={<Activity />}
          label={t("overview.queries")}
          value={adGuard?.stats_available ? formatInteger(adGuard.queries) : t("overview.metricUnavailable")}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
        />
        <ResourceCard
          icon={<Ban />}
          label={t("overview.blocked")}
          value={adGuard?.stats_available ? formatInteger(adGuard.blocked) : t("overview.metricUnavailable")}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
        />
        <ResourceCard
          icon={<Filter />}
          label={t("overview.blockRate")}
          value={adGuard?.stats_available ? formatBlockRate(adGuard.blocked, adGuard.queries) : t("overview.metricUnavailable")}
          detail={t("overview.statisticsPeriod")}
          available={adGuard?.stats_available === true}
        />
      </section>

      <div className="overview-content-grid">
        <section className="overview-panel dns-flow-panel">
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
        </section>

        <section className="overview-panel service-panel">
          <PanelHeading eyebrow={t("overview.runtime")} title={t("overview.dnsServices")} />
          <div className="dashboard-services">
            {services.map((service) => (
              <article key={service.name}>
                <span className={`service-light ${service.status}`} />
                <div>
                  <strong>{service.displayName}</strong>
                  <p>{service.description}</p>
                </div>
                <button
                  className="rg-button rg-button-secondary"
                  type="button"
                  disabled={service.status !== "running" || busyService === service.name}
                  onClick={() => restart(service.name)}
                >
                  <RefreshCw size={14} className={busyService === service.name ? "spinning" : ""} />
                  {t("common.restart")}
                </button>
              </article>
            ))}
          </div>
          <div className="panel-footer">
            <span>{t("overview.lastChecked", { time: lastChecked ? lastChecked.toLocaleTimeString(locale) : "–" })}</span>
            <Link to="/setup">{t("overview.manageStack")} <ArrowRight size={14} /></Link>
          </div>
        </section>
      </div>
    </div>
  );
}

function KpiCard({ icon, label, value, note, good }: {
  icon: React.ReactNode;
  label: string;
  value: string;
  note: string;
  good?: boolean;
}) {
  return (
    <article className="overview-kpi">
      <span className={`kpi-icon ${good ? "good" : ""}`}>{icon}</span>
      <div><small>{label}</small><strong>{value}</strong><p>{note}</p></div>
    </article>
  );
}

function ResourceCard({ icon, label, value, detail, available }: {
  icon: React.ReactNode;
  label: string;
  value: string;
  detail: string;
  available: boolean;
}) {
  return (
    <article className={`overview-resource ${available ? "available" : ""}`}>
      <span>{icon}</span>
      <div><small>{label}</small><strong>{value}</strong><p>{detail}</p></div>
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
  const rate = queries === 0 ? 0 : (blocked / queries) * 100;
  return `${rate.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })} %`;
}
