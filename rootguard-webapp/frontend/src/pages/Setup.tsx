import { useCallback, useEffect, useState } from "react";
import {
  deployInstallation,
  fetchInstallationStatus,
  preflightInstallation,
  type InstallationConfig,
  type InstallationDiagnostic,
  type InstallationPreflight,
  type InstallationStatus,
} from "../api/client";
import "../styles/setup.css";
import { useI18n } from "../i18n";
import { AlertTriangle, ArrowRight, Check, Filter, Network, RotateCcw, ServerCog, ShieldCheck } from "lucide-react";

const defaultConfig: InstallationConfig = {
  dns_bind_address: "0.0.0.0",
  dns_port: 53,
  adguard_channel: "stable",
};

export default function Setup() {
  const { t } = useI18n();
  const [config, setConfig] = useState<InstallationConfig>(defaultConfig);
  const [status, setStatus] = useState<InstallationStatus | null>(null);
  const [preflight, setPreflight] = useState<InstallationPreflight | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const loadStatus = useCallback(async () => {
    try {
      const current = await fetchInstallationStatus();
      setStatus(current);
      if (current.config && current.state !== "deploying") {
        setConfig({ ...current.config, adguard_channel: current.config.adguard_channel ?? "stable" });
      }
    } catch (cause) {
      setError(messageFrom(cause, t("setup.unexpected")));
    }
  }, [t]);

  useEffect(() => {
    const initialLoad = window.setTimeout(loadStatus, 0);
    return () => window.clearTimeout(initialLoad);
  }, [loadStatus]);

  useEffect(() => {
    if (status?.state !== "deploying") {
      return;
    }
    const poll = window.setInterval(loadStatus, 1500);
    return () => window.clearInterval(poll);
  }, [loadStatus, status?.state]);

  async function runPreflight() {
    setBusy(true);
    setError("");
    try {
      setPreflight(await preflightInstallation(config));
    } catch (cause) {
      setError(messageFrom(cause, t("setup.unexpected")));
    } finally {
      setBusy(false);
    }
  }

  async function deploy() {
    setBusy(true);
    setError("");
    try {
      const nextStatus = await deployInstallation(config);
      setStatus(nextStatus);
      setPreflight(null);
    } catch (cause) {
      setError(messageFrom(cause, t("setup.unexpected")));
    } finally {
      setBusy(false);
    }
  }

  const deploying = status?.state === "deploying";
  const progressStep = status?.state === "installed" || status?.state === "deploying" || status?.state === "failed"
    ? 3
    : preflight
      ? 2
      : 1;

  return (
    <div className="setup-page">
      <section className="setup-hero">
        <div className="setup-heading">
          <span className="setup-eyebrow">ROOTGUARD AIO</span>
          <h1>{t("setup.title")}</h1>
          <p>{t("setup.intro")}</p>
          <StateBadge state={status?.state ?? "not_installed"} />
        </div>
        <div className="setup-blueprint" aria-label={t("setup.blueprint")}>
          <span className="blueprint-label">{t("setup.blueprint")}</span>
          <div className="blueprint-chain">
            <BlueprintNode icon={<Network />} label={t("setup.host")} detail={`${config.dns_bind_address}:${config.dns_port}`} />
            <span className="blueprint-link"><i /><ArrowRight size={15} /></span>
            <BlueprintNode icon={<Filter />} label="AdGuard Home" detail={t(`setup.channel.${config.adguard_channel}`)} />
            <span className="blueprint-link"><i /><ArrowRight size={15} /></span>
            <BlueprintNode icon={<ShieldCheck />} label="Unbound" detail={t("setup.resolver")} />
          </div>
          <div className="blueprint-footnote"><Check size={14} /> {t("setup.onlyDns")}</div>
        </div>
      </section>

      <div className="setup-progress" aria-label={t("setup.progressLabel")}>
        <ProgressStep number="01" label={t("setup.progress.network")} active={progressStep === 1} complete={progressStep > 1} />
        <span />
        <ProgressStep number="02" label={t("setup.progress.preflight")} active={progressStep === 2} complete={progressStep > 2} />
        <span />
        <ProgressStep number="03" label={t("setup.progress.deployment")} active={progressStep === 3} complete={status?.state === "installed"} />
      </div>

      <section className="setup-card network-card">
        <div className="setup-card-title">
          <span className="setup-number"><Network size={17} /></span>
          <div>
            <h2>{t("setup.network")}</h2>
            <p>{t("setup.networkHelp")}</p>
          </div>
        </div>

        <div className="network-card-grid">
          <div className="setup-fields">
            <label>
              {t("setup.address")}
              <input
                value={config.dns_bind_address}
                disabled={deploying}
                inputMode="decimal"
                onChange={(event) => {
                  setConfig({ ...config, dns_bind_address: event.target.value });
                  setPreflight(null);
                }}
                placeholder="192.168.178.10"
              />
              <small>{t("setup.addressHelp")}</small>
            </label>
            <label>
              {t("setup.port")}
              <input
                type="number"
                min={1}
                max={65535}
                value={config.dns_port}
                disabled={deploying}
                onChange={(event) => {
                  setConfig({ ...config, dns_port: Number(event.target.value) });
                  setPreflight(null);
                }}
              />
              <small>{t("setup.portHelp")}</small>
            </label>
          </div>
          <aside className="endpoint-preview">
            <span><ServerCog size={17} /></span>
            <small>{t("setup.endpointPreview")}</small>
            <strong>{config.dns_bind_address}:{config.dns_port}</strong>
            <p>{t("setup.endpointHelp")}</p>
            <div><ShieldCheck size={14} /><span><b>{t("setup.privateAdmin")}</b>{t("setup.privateAdminHelp")}</span></div>
          </aside>
        </div>

        <fieldset className="release-channel" disabled={deploying}>
          <legend>{t("setup.channel.title")}</legend>
          <p>{t("setup.channel.help")}</p>
          <div className="release-channel-options">
            {(["stable", "beta"] as const).map((channel) => (
              <label className={config.adguard_channel === channel ? "selected" : ""} key={channel}>
                <input
                  type="radio"
                  name="adguard-channel"
                  value={channel}
                  checked={config.adguard_channel === channel}
                  onChange={() => {
                    setConfig({ ...config, adguard_channel: channel });
                    setPreflight(null);
                  }}
                />
                <span>
                  <strong>{t(`setup.channel.${channel}`)}</strong>
                  <small>{t(`setup.channel.${channel}Help`)}</small>
                </span>
                {channel === "stable" && <i>{t("setup.channel.recommended")}</i>}
              </label>
            ))}
          </div>
        </fieldset>

        {config.adguard_channel === "beta" && (
          <div className="beta-warning" role="alert">
            <AlertTriangle size={20} />
            <div><strong>{t("setup.channel.betaWarningTitle")}</strong><p>{t("setup.channel.betaWarning")}</p></div>
          </div>
        )}

        <div className="setup-actions">
          <button className="rg-button rg-button-secondary setup-button secondary" disabled={busy || deploying} onClick={runPreflight}>
            {busy ? t("setup.checking") : t("setup.runPreflight")}
          </button>
        </div>
      </section>

      {preflight && (
        <section className="setup-card">
          <div className="setup-card-title">
            <span className="setup-number"><ShieldCheck size={17} /></span>
            <div>
              <h2>{t("setup.preflight")}</h2>
              <p>{t("setup.noChanges")}</p>
            </div>
          </div>
          <div className="check-list">
            {preflight.checks.map((check) => (
              <div className={`check-row ${check.ok ? "ok" : "failed"}`} key={check.id}>
                <span aria-hidden="true">{check.ok ? "✓" : "!"}</span>
                <div>
                  <p>{diagnosticText(t, check.code, "message", check.message, check.detail)}</p>
                  {!check.ok && check.action && (
                    <small>{diagnosticText(t, check.code, "action", check.action, check.detail)}</small>
                  )}
                </div>
              </div>
            ))}
          </div>
          <div className="setup-actions">
            <button
              className="rg-button rg-button-primary setup-button primary"
              disabled={!preflight.ready || busy}
              onClick={deploy}
            >
              {t("setup.install")}
            </button>
          </div>
        </section>
      )}

      {status && status.steps.length > 0 && (
        <section className="setup-card">
          <div className="setup-card-title">
            <span className="setup-number"><ServerCog size={17} /></span>
            <div>
              <h2>{t("setup.deployment")}</h2>
              <p>{t("setup.deploymentHelp")}</p>
            </div>
          </div>
          <div className="step-list">
            {status.steps.map((step) => (
              <div className={`step-row ${step.status}`} key={step.id}>
                <span className="step-dot" aria-hidden="true" />
                <div>
                  <strong>{t(`setup.step.${step.id}`)}</strong>
                  <p>{t(`setup.step.${step.id}`)}</p>
                </div>
                <span className="step-status">{t(`setup.step.${step.status}`)}</span>
              </div>
            ))}
          </div>
          {status.state === "installed" && status.config && (
            <div className="setup-success">
              {t("setup.ready", { address: status.config.dns_bind_address })}
            </div>
          )}
        </section>
      )}

      {status?.diagnostic && !error && <DiagnosticPanel diagnostic={status.diagnostic} />}

      {(error || (status?.error && !status.diagnostic)) && (
        <div className="setup-error" role="alert">
          <strong>{t("setup.failed")}</strong>
          <span>{error || status?.error}</span>
        </div>
      )}
    </div>
  );
}

function StateBadge({ state }: { state: InstallationStatus["state"] }) {
  const { t } = useI18n();
  return <span className={`setup-state ${state}`}>{t(`setup.state.${state}`)}</span>;
}

function BlueprintNode({ icon, label, detail }: { icon: React.ReactNode; label: string; detail: string }) {
  return <div className="blueprint-node"><span>{icon}</span><strong>{label}</strong><small>{detail}</small></div>;
}

function ProgressStep({ number, label, active, complete }: { number: string; label: string; active: boolean; complete?: boolean }) {
  return <div className={`${active ? "active" : ""} ${complete ? "complete" : ""}`}><i>{complete ? <Check size={13} /> : number}</i><strong>{label}</strong></div>;
}

function DiagnosticPanel({ diagnostic }: { diagnostic: InstallationDiagnostic }) {
  const { t } = useI18n();
  return (
    <section className="setup-diagnostic" role="alert">
      <div className="diagnostic-heading">
        <span><AlertTriangle size={18} /></span>
        <div>
          <small>{t("setup.diagnostic.title")}</small>
          <strong>{diagnosticText(t, diagnostic.code, "message", diagnostic.message)}</strong>
        </div>
        <code>{diagnostic.code}</code>
      </div>
      <div className="diagnostic-action">
        <b>{t("setup.diagnostic.action")}</b>
        <p>{diagnosticText(t, diagnostic.code, "action", diagnostic.action)}</p>
      </div>
      {diagnostic.retryable && (
        <div className="diagnostic-retry"><RotateCcw size={14} /> {t("setup.diagnostic.retryable")}</div>
      )}
      {diagnostic.detail && (
        <details>
          <summary>{t("setup.diagnostic.technical")}</summary>
          <pre>{diagnostic.detail}</pre>
        </details>
      )}
    </section>
  );
}

function diagnosticText(
  t: (key: string, values?: Record<string, string | number>) => string,
  code: string,
  field: "message" | "action",
  fallback: string,
  detail = ""
): string {
  if (!code) {
    return fallback;
  }
  const key = `setup.diagnostic.${code}.${field}`;
  const translated = t(key, { detail: detail ? ` (${detail})` : "" });
  return translated === key ? fallback : translated;
}

function messageFrom(cause: unknown, fallback: string): string {
  return cause instanceof Error ? cause.message : fallback;
}
