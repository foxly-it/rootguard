import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Archive,
  CheckCircle2,
  Cpu,
  Download,
  FileText,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  ServerCog,
  ShieldCheck,
  PanelsTopLeft,
} from "lucide-react";
import {
  checkControlPlaneUpdates,
  checkUpdates,
  fetchControlPlaneUpdateStatus,
  fetchServices,
  fetchServiceLogs,
  fetchUpdateStatus,
  installServiceUpdate,
  installControlPlaneUpdates,
  serviceAction,
  type ServiceInfo,
  type ServiceLogs,
  type UpdateServiceStatus,
  type UpdateStatus,
  type ControlPlaneUpdateStatus,
  type UpdateHistoryEntry,
} from "../api/client";
import "../styles/stack.css";
import { useI18n } from "../i18n";

export default function Stack() {
  const { t, formatDate } = useI18n();
  const [updates, setUpdates] = useState<UpdateStatus | null>(null);
  const [controlPlane, setControlPlane] = useState<ControlPlaneUpdateStatus | null>(null);
  const [services, setServices] = useState<ServiceInfo[]>([]);
  const [serviceLogs, setServiceLogs] = useState<Partial<Record<ServiceInfo["name"], ServiceLogs>>>({});
  const [loadingLogs, setLoadingLogs] = useState<ServiceInfo["name"] | "">("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const [nextUpdates, nextControlPlane, nextServices] = await Promise.all([
        fetchUpdateStatus(),
        fetchControlPlaneUpdateStatus(),
        fetchServices(),
      ]);
      setUpdates(nextUpdates);
      setControlPlane(nextControlPlane);
      setServices(nextServices);
      setError("");
    } catch (cause) {
      setError(errorMessage(cause, "Stack-Status konnte nicht geladen werden."));
    }
  }, []);

  useEffect(() => {
    const initial = window.setTimeout(load, 0);
    return () => window.clearTimeout(initial);
  }, [load]);

  const busy = updates?.state === "checking" || updates?.state === "updating"
    || controlPlane?.state === "checking" || controlPlane?.state === "updating";
  useEffect(() => {
    if (!busy) return;
    const timer = window.setInterval(load, 1500);
    return () => window.clearInterval(timer);
  }, [busy, load]);

  const available = useMemo(
    () => (updates?.services.filter((service) => service.update_available).length ?? 0)
      + (controlPlane?.services.filter((service) => service.update_available).length ?? 0),
    [updates, controlPlane],
  );
  const history = useMemo(
    () => [
      ...(updates?.history ?? []).map((entry) => ({ ...entry, scope: entry.service || "DNS" })),
      ...(controlPlane?.history ?? []).map((entry) => ({ ...entry, scope: "Control Plane" })),
    ].sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at)).slice(0, 12),
    [updates, controlPlane],
  );

  async function startCheck() {
    setError("");
    try {
      const [nextUpdates, nextControlPlane] = await Promise.all([
        checkUpdates(),
        checkControlPlaneUpdates(),
      ]);
      setUpdates(nextUpdates);
      setControlPlane(nextControlPlane);
    } catch (cause) {
      setError(errorMessage(cause, "Update-Prüfung konnte nicht gestartet werden."));
    }
  }

  async function startControlPlaneUpdate() {
    if (!window.confirm(t("stack.controlPlaneConfirm"))) return;
    setError("");
    try {
      setControlPlane(await installControlPlaneUpdates());
    } catch (cause) {
      setError(errorMessage(cause, t("stack.controlPlaneStartError")));
    }
  }

  async function startUpdate(service: UpdateServiceStatus) {
    const accepted = window.confirm(
      t("stack.confirmUpdate", { service: service.display_name }),
    );
    if (!accepted) return;
    setError("");
    try {
      setUpdates(await installServiceUpdate(service.name));
    } catch (cause) {
      setError(errorMessage(cause, "Update konnte nicht gestartet werden."));
    }
  }

  async function control(name: ServiceInfo["name"], action: "start" | "stop" | "restart") {
    if (action === "stop" && !window.confirm(t("stack.confirmStop", { service: name }))) return;
    try {
      await serviceAction(name, action);
      await load();
    } catch (cause) {
      setError(errorMessage(cause, "Dienstaktion fehlgeschlagen."));
    }
  }

  async function toggleLogs(name: ServiceInfo["name"]) {
    if (serviceLogs[name]) {
      setServiceLogs((current) => {
        const next = { ...current };
        delete next[name];
        return next;
      });
      return;
    }
    setLoadingLogs(name);
    setError("");
    try {
      const logs = await fetchServiceLogs(name);
      setServiceLogs((current) => ({ ...current, [name]: logs }));
    } catch (cause) {
      setError(errorMessage(cause, t("stack.logsError")));
    } finally {
      setLoadingLogs("");
    }
  }

  return (
    <div className="stack-page">
      <section className="stack-hero">
        <div>
          <span className="stack-eyebrow">CONTROLLED LIFECYCLE</span>
          <h1>{t("stack.title")}</h1>
          <p>{t("stack.intro")}</p>
        </div>
        <button type="button" className="stack-check-button" disabled={busy} onClick={startCheck}>
          {updates?.state === "checking" ? <LoaderCircle className="spin" size={17} /> : <RefreshCw size={17} />}
          {updates?.state === "checking" ? t("stack.checking") : t("stack.check")}
        </button>
      </section>

      {error && <div className="stack-feedback error">{error}</div>}
      {updates && (
        <div className={`stack-feedback ${updates.state === "failed" ? "error" : busy ? "working" : "success"}`}>
          {busy && <LoaderCircle className="spin" size={17} />}
          {!busy && updates.state !== "failed" && <CheckCircle2 size={17} />}
          <span>{updates.message}</span>
        </div>
      )}

      <section className="stack-summary">
        <Summary icon={<ServerCog />} label={t("stack.managed")} value={t("stack.runningCount", { count: services.filter((s) => s.status === "running").length })} />
        <Summary icon={<Download />} label={t("stack.available")} value={available ? t("stack.found", { count: available }) : t("stack.none")} />
        <Summary icon={<Archive />} label={t("stack.protection")} value={t("stack.backup")} />
        <Summary icon={<RotateCcw />} label={t("stack.failure")} value={t("stack.rollback")} />
      </section>

      {controlPlane && (
        <section className="control-plane-panel">
          <div className="control-plane-heading">
            <div>
              <span className="stack-eyebrow">{t("stack.controlPlaneEyebrow")}</span>
              <h2>{t("stack.controlPlaneTitle")}</h2>
              <p>{t("stack.controlPlaneIntro")}</p>
            </div>
            <button
              type="button"
              className="stack-check-button"
              disabled={busy || !controlPlane.services.some((service) => service.update_available)}
              onClick={startControlPlaneUpdate}
            >
              {controlPlane.state === "updating" ? <LoaderCircle className="spin" size={17} /> : <Download size={17} />}
              {controlPlane.state === "updating" ? t("stack.controlPlaneUpdating") : t("stack.controlPlaneInstall")}
            </button>
          </div>
          <div className={`control-plane-status ${controlPlane.state}`}>
            {controlPlane.state === "checking" || controlPlane.state === "updating"
              ? <LoaderCircle className="spin" size={15} />
              : <ShieldCheck size={15} />}
            <span>{controlPlane.message}</span>
          </div>
          <div className="control-plane-services">
            {controlPlane.services.map((service) => (
              <article key={service.name}>
                <span>{service.name === "core" ? <Cpu /> : <PanelsTopLeft />}</span>
                <div>
                  <strong>{service.display_name}</strong>
                  <small>{service.current_image || t("stack.notInspected")}</small>
                </div>
                <em className={service.update_available ? "available" : ""}>
                  {service.update_available ? t("stack.update") : service.checked_at ? t("stack.current") : t("stack.unchecked")}
                </em>
              </article>
            ))}
          </div>
          <p className="control-plane-note"><RotateCcw size={15} /> {t("stack.controlPlaneRollback")}</p>
        </section>
      )}

      <section className="stack-services">
        {updates?.services.map((service) => {
          const runtime = services.find((item) => item.name === service.name);
          const active = updates.active_service === service.name;
          return (
            <article className="stack-service-card" key={service.name}>
              <div className="stack-card-heading">
                <div>
                  <span className={`stack-runtime ${runtime?.status === "running" ? "running" : ""}`}>
                    {runtime?.status === "running" ? t("stack.running") : t("stack.stopped")}
                  </span>
                  <h2>{service.display_name}</h2>
                </div>
                <span className={`stack-update-badge ${service.update_available ? "available" : ""}`}>
                  {active ? t("stack.working") : service.update_available ? t("stack.update") : service.checked_at ? t("stack.current") : t("stack.unchecked")}
                </span>
              </div>

              <div className={`stack-runtime-explainer ${runtimeTone(runtime)}`}>
                <strong>{runtimeHeadline(runtime, t)}</strong>
                <p>{runtimeGuidance(runtime, t)}</p>
              </div>

              <dl className="stack-runtime-data">
                <div><dt>{t("stack.version")}</dt><dd>{imageVersion(runtime?.image)}</dd></div>
                <div><dt>{t("stack.health")}</dt><dd>{healthLabel(runtime, t)}</dd></div>
                <div><dt>{t("stack.started")}</dt><dd>{runtime?.startedAt ? formatDate(runtime.startedAt) : "–"}</dd></div>
                <div><dt>{t("stack.restarts")}</dt><dd>{runtime?.restartCount ?? 0}</dd></div>
                <div><dt>{t("stack.publishedPorts")}</dt><dd>{runtime?.ports?.length ? runtime.ports.join(", ") : t("stack.noPublishedPorts")}</dd></div>
              </dl>

              <dl className="stack-image-data">
                <div><dt>{t("stack.runningImage")}</dt><dd>{runtime?.image || service.current_image || "–"}</dd></div>
                <div><dt>{t("stack.target")}</dt><dd>{service.target_image}</dd></div>
                <div><dt>{t("stack.imageId")}</dt><dd>{shortID(runtime?.imageId || service.current_id)}</dd></div>
                <div><dt>{t("stack.lastCheck")}</dt><dd>{service.checked_at && !service.checked_at.startsWith("0001-") ? formatDate(service.checked_at) : t("stack.neverChecked")}</dd></div>
              </dl>

              {service.error && <p className="stack-service-error">{service.error}</p>}

              <div className="stack-card-actions">
                <button type="button" disabled={busy || !service.update_available} onClick={() => startUpdate(service)}>
                  <Download size={15} /> {t("stack.install")}
                </button>
                <button type="button" disabled={busy} onClick={() => control(service.name, "restart")}>{t("common.restart")}</button>
                <button type="button" className="quiet" disabled={busy} onClick={() => control(service.name, runtime?.status === "running" ? "stop" : "start")}>
                  {runtime?.status === "running" ? t("common.stop") : t("common.start")}
                </button>
                <button type="button" className="quiet" disabled={loadingLogs === service.name} onClick={() => toggleLogs(service.name)}>
                  {loadingLogs === service.name ? <LoaderCircle className="spin" size={15} /> : <FileText size={15} />}
                  {serviceLogs[service.name] ? t("stack.hideLogs") : t("stack.showLogs")}
                </button>
              </div>

              {serviceLogs[service.name] && (
                <section className="stack-log-view" aria-label={t("stack.logsFor", { service: service.display_name })}>
                  <div>
                    <strong>{t("stack.logsTitle")}</strong>
                    <span>{t("stack.logsWindow")}</span>
                  </div>
                  <p>{t("stack.logsPrivacy")}</p>
                  <pre>{serviceLogs[service.name]?.lines.length
                    ? serviceLogs[service.name]?.lines.join("\n")
                    : t("stack.logsEmpty")}</pre>
                  {serviceLogs[service.name]?.truncated && <small>{t("stack.logsTruncated")}</small>}
                </section>
              )}
            </article>
          );
        })}
      </section>

      <section className="stack-history">
        <div className="stack-history-heading">
          <div>
            <span className="stack-eyebrow">{t("stack.historyEyebrow")}</span>
            <h2>{t("stack.historyTitle")}</h2>
          </div>
          <p>{t("stack.historyIntro")}</p>
        </div>
        {history.length ? (
          <div className="stack-history-list">
            {history.map((entry, index) => (
              <HistoryRow key={`${entry.created_at}-${entry.scope}-${index}`} entry={entry} scope={entry.scope} formatDate={formatDate} t={t} />
            ))}
          </div>
        ) : <p className="stack-history-empty">{t("stack.historyEmpty")}</p>}
      </section>

      <section className="stack-safety">
        <ShieldCheck size={23} />
        <div>
          <strong>{t("stack.safetyTitle")}</strong>
          <p>{t("stack.safetyText")}</p>
        </div>
      </section>
    </div>
  );
}

function HistoryRow({
  entry,
  scope,
  formatDate,
  t,
}: {
  entry: UpdateHistoryEntry;
  scope: string;
  formatDate: (value: string) => string;
  t: Translate;
}) {
  const removed = (entry.cleanup?.removed_images?.length ?? 0) + (entry.cleanup?.removed_volumes?.length ?? 0);
  const skipped = entry.cleanup?.skipped?.length ?? 0;
  return (
    <article>
      <span className={`stack-history-state ${entry.outcome}`}>{t(`stack.outcome.${entry.outcome}`)}</span>
      <div>
        <strong>{scope}</strong>
        <p>{entry.message}</p>
        <small>
          {removed
            ? t("stack.cleanupRemoved", { count: removed })
            : skipped
              ? t("stack.cleanupSkipped", { count: skipped })
              : t("stack.cleanupNoop")}
        </small>
      </div>
      <time dateTime={entry.created_at}>{formatDate(entry.created_at)}</time>
    </article>
  );
}

function Summary({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return <article><span>{icon}</span><div><small>{label}</small><strong>{value}</strong></div></article>;
}

function shortID(value?: string) {
  if (!value) return "–";
  return value.replace("sha256:", "").slice(0, 12);
}

type Translate = (key: string, values?: Record<string, string | number>) => string;

function runtimeTone(service?: ServiceInfo) {
  if (!service || service.status !== "running" || service.health === "unhealthy") return "danger";
  if (service.health === "starting" || service.health === "unknown") return "attention";
  return "good";
}

function runtimeHeadline(service: ServiceInfo | undefined, t: Translate) {
  if (!service || service.status !== "running") return t("stack.runtimeStoppedTitle");
  if (service.health === "unhealthy") return t("stack.runtimeUnhealthyTitle");
  if (service.health === "starting") return t("stack.runtimeStartingTitle");
  if (service.health === "not_configured") return t("stack.runtimeNoHealthcheckTitle");
  if (service.health === "unknown") return t("stack.runtimeUnknownTitle");
  return t("stack.runtimeHealthyTitle");
}

function runtimeGuidance(service: ServiceInfo | undefined, t: Translate) {
  if (!service || service.status !== "running") return t("stack.runtimeStoppedText");
  if (service.health === "unhealthy") return t("stack.runtimeUnhealthyText");
  if (service.health === "starting") return t("stack.runtimeStartingText");
  if (service.health === "not_configured") return t("stack.runtimeNoHealthcheckText");
  if (service.health === "unknown") return t("stack.runtimeUnknownText");
  return t("stack.runtimeHealthyText");
}

function healthLabel(service: ServiceInfo | undefined, t: Translate) {
  if (!service || service.status !== "running") return t("stack.stopped");
  return t(`stack.health.${service.health}`);
}

function imageVersion(image?: string) {
  if (!image) return "–";
  const digest = image.indexOf("@");
  if (digest >= 0) return image.slice(digest + 1, digest + 20);
  const slash = image.lastIndexOf("/");
  const colon = image.lastIndexOf(":");
  return colon > slash ? image.slice(colon + 1) : "latest";
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
