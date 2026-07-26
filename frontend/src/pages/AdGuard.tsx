import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router";
import { ArrowRight, Check, ExternalLink, Filter, KeyRound, LockKeyhole, Network, ShieldCheck } from "lucide-react";
import {
  bootstrapAdGuard,
  fetchAdGuardStatus,
  fetchInstallationStatus,
  type AdGuardStatus,
  type InstallationStatus,
} from "../api/client";
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
      setMessage("AdGuard Home wurde sicher eingerichtet und exklusiv mit Unbound verbunden.");
    } catch (cause) {
      setError(errorMessage(cause, "AdGuard Home konnte nicht eingerichtet werden."));
    } finally {
      setBootstrapping(false);
    }
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
            <Link className="adguard-primary-action" to="/setup">
              {t("adguard.setup")} <ArrowRight size={16} />
            </Link>
          )}
          {ready && (
            <a className="adguard-primary-action" href="/adguard-ui/" target="_blank" rel="noreferrer">
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
            <StatusRow label={t("adguard.config")} active={Boolean(status?.configured)} activeText={t("adguard.managed")} inactiveText={t("adguard.notSetup")} />
            <StatusRow label={t("adguard.service")} active={Boolean(status?.healthy)} activeText={t("adguard.reachable")} inactiveText={t("adguard.unreachable")} />
            <StatusRow label={t("adguard.upstream")} active={Boolean(status?.upstream_ready)} activeText={t("adguard.validated")} inactiveText={t("adguard.pending")} />
          </div>
          <div className="adguard-upstream">
            <span>{t("adguard.activeUpstream")}</span>
            <code>{status?.upstream || "172.29.53.2:5335"}</code>
          </div>
          {!loading && installation?.state === "installed" && !status?.configured && (
            <button className="adguard-primary-action" type="button" disabled={bootstrapping} onClick={initialize}>
              {bootstrapping ? t("adguard.settingUp") : t("adguard.finish")}
            </button>
          )}
        </section>

        <section className="adguard-panel">
          <div className="adguard-panel-heading">
            <div><span className="adguard-eyebrow">ROOTGUARD AIO</span><h2>{t("adguard.automatic")}</h2></div>
          </div>
          <div className="managed-steps">
            <ManagedStep icon={<Network />} number="01" title={t("adguard.privateNetwork")} text={t("adguard.privateNetworkText")} />
            <ManagedStep icon={<KeyRound />} number="02" title={t("adguard.credentials")} text={t("adguard.credentialsText")} />
            <ManagedStep icon={<Filter />} number="03" title={t("adguard.secureUpstream")} text={t("adguard.secureUpstreamText")} />
          </div>
        </section>
      </div>

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

function StatusRow({ label, active, activeText, inactiveText }: {
  label: string;
  active: boolean;
  activeText: string;
  inactiveText: string;
}) {
  return (
    <div>
      <span className={active ? "status-check active" : "status-check"}>{active ? <Check size={14} /> : "!"}</span>
      <strong>{label}</strong>
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
