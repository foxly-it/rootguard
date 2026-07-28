import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, GlobeLock, Plus, RotateCcw, ShieldAlert, ShieldCheck, Trash2 } from "lucide-react";
import {
  fetchUnboundSettings,
  previewUnboundSettings,
  updateUnboundSettings,
  type UnboundPreview,
  type UnboundReverseZonePolicy,
  type UnboundSettings,
} from "../api/client";
import { useI18n } from "../i18n";
import "../styles/unbound-private.css";

const maxPrivateDomains = 32;
const reverseNetworks: UnboundReverseZonePolicy["network"][] = [
  "10.0.0.0/8",
  "172.16.0.0/12",
  "192.168.0.0/16",
];

export default function UnboundPrivateDomains({
  version,
  onActivated,
}: {
  version?: string;
  onActivated: () => Promise<void>;
}) {
  const { t } = useI18n();
  const [source, setSource] = useState<UnboundSettings | null>(null);
  const [domains, setDomains] = useState<string[]>([]);
  const [reverseZones, setReverseZones] = useState<UnboundReverseZonePolicy[]>(defaultReverseZones);
  const [draft, setDraft] = useState("home.arpa.");
  const [preview, setPreview] = useState<UnboundPreview | null>(null);
  const [candidate, setCandidate] = useState<UnboundSettings | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const settings = normalizeSettings(await fetchUnboundSettings());
    setSource(settings);
    setDomains(structuredClone(settings.private_domains));
    setReverseZones(structuredClone(settings.reverse_zones));
    setPreview(null);
    setCandidate(null);
    setError("");
  }, []);

  useEffect(() => {
    load().catch((cause: unknown) => setError(errorMessage(cause, t("private.loadError"))));
  }, [load, t, version]);

  const dirty = useMemo(() => source !== null && (
    JSON.stringify(domains) !== JSON.stringify(source.private_domains) ||
    JSON.stringify(reverseZones) !== JSON.stringify(source.reverse_zones)
  ), [domains, reverseZones, source]);

  function addDomain() {
    setError("");
    try {
      const domain = normalizeDomain(draft, t);
      if (domains.includes(domain)) throw new Error(t("private.duplicate", { name: domain }));
      if (domains.length >= maxPrivateDomains) throw new Error(t("private.limit", { count: maxPrivateDomains }));
      setDomains([...domains, domain]);
      setDraft("");
      resetPreview();
      setMessage(t("private.draftSaved"));
    } catch (cause) {
      setError(errorMessage(cause, t("private.invalid")));
    }
  }

  function removeDomain(domain: string) {
    if (!window.confirm(t("private.confirmRemove", { name: domain }))) return;
    setDomains((current) => current.filter((item) => item !== domain));
    resetPreview();
    setMessage(t("private.removed"));
  }

  function setReverseMode(network: UnboundReverseZonePolicy["network"], mode: UnboundReverseZonePolicy["mode"]) {
    setReverseZones((current) => current.map((policy) => policy.network === network ? { ...policy, mode } : policy));
    resetPreview();
    setMessage("");
  }

  async function createPreview() {
    if (!source || busy) return;
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const active = normalizeSettings(await fetchUnboundSettings());
      if (!sameSettings(active, source)) throw new Error(t("private.concurrent"));
      const proposed = { ...active, private_domains: domains, reverse_zones: reverseZones };
      const result = await previewUnboundSettings(proposed);
      setCandidate(proposed);
      setPreview(result);
      setMessage(t("private.previewAccepted"));
    } catch (cause) {
      resetPreview();
      setError(errorMessage(cause, t("private.previewRejected")));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!source || !candidate || !preview?.changed || busy) return;
    if (!window.confirm(t("private.confirmActivate"))) return;
    setBusy(true);
    setError("");
    try {
      const active = normalizeSettings(await fetchUnboundSettings());
      if (!sameSettings(active, source)) throw new Error(t("private.concurrent"));
      await updateUnboundSettings(candidate);
      await onActivated();
      await load();
      setMessage(t("private.activated"));
    } catch (cause) {
      setError(errorMessage(cause, t("private.activateError")));
    } finally {
      setBusy(false);
    }
  }

  function resetPreview() {
    setPreview(null);
    setCandidate(null);
  }

  return (
    <section className="glass-card private-domains-panel">
      <div className="panel-heading private-heading">
        <div>
          <p className="unbound-eyebrow">{t("private.eyebrow")}</p>
          <h2>{t("private.title")}</h2>
          <p className="muted-copy">{t("private.intro")}</p>
        </div>
        <span className="private-protection"><ShieldCheck size={15} /> {t("private.scoped")}</span>
      </div>

      {message && <div className="feedback success">{message}</div>}
      {error && <div className="feedback error" role="alert">{error}</div>}

      <div className="private-domain-editor">
        <div>
          <strong>{t("private.domains")}</strong>
          <small>{t("private.domainsHelp")}</small>
        </div>
        <label>
          <span className="sr-only">{t("private.domain")}</span>
          <input value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="home.arpa." autoCapitalize="none" spellCheck={false} onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              addDomain();
            }
          }} />
        </label>
        <button type="button" disabled={!draft.trim() || domains.length >= maxPrivateDomains} onClick={addDomain}><Plus size={15} /> {t("private.add")}</button>
      </div>

      <div className="private-domain-list">
        {domains.length === 0 && <div className="guided-empty"><GlobeLock size={22} /><div><strong>{t("private.empty")}</strong><p>{t("private.emptyHelp")}</p></div></div>}
        {domains.map((domain) => (
          <article key={domain}>
            <span><GlobeLock size={15} /></span>
            <div><strong>{domain}</strong><small>{t("private.domainPolicy")}</small></div>
            <button type="button" aria-label={t("private.remove", { name: domain })} onClick={() => removeDomain(domain)}><Trash2 size={14} /></button>
          </article>
        ))}
      </div>

      <div className="reverse-heading">
        <div><strong>{t("private.reverseTitle")}</strong><small>{t("private.reverseHelp")}</small></div>
        <RotateCcw size={18} />
      </div>
      <div className="reverse-policy-list">
        {reverseZones.map((policy) => (
          <article className={policy.mode} key={policy.network}>
            <div><code>{policy.network}</code><small>{t(`private.network.${policy.network}`)}</small></div>
            <div className="reverse-mode" role="radiogroup" aria-label={t("private.reverseMode", { network: policy.network })}>
              <label>
                <input type="radio" name={`reverse-${policy.network}`} checked={policy.mode === "nxdomain"} onChange={() => setReverseMode(policy.network, "nxdomain")} />
                <span><ShieldCheck size={14} /><b>NXDOMAIN</b><small>{t("private.nxdomainHelp")}</small></span>
              </label>
              <label>
                <input type="radio" name={`reverse-${policy.network}`} checked={policy.mode === "transparent"} onChange={() => setReverseMode(policy.network, "transparent")} />
                <span><ShieldAlert size={14} /><b>{t("private.transparent")}</b><small>{t("private.transparentHelp")}</small></span>
              </label>
            </div>
          </article>
        ))}
      </div>

      {reverseZones.some((policy) => policy.mode === "transparent") && (
        <div className="private-warning"><ShieldAlert size={17} /><span><strong>{t("private.leakWarning")}</strong>{t("private.leakWarningHelp")}</span></div>
      )}

      {dirty && (
        <div className="guided-review">
          <div><strong>{t("private.draftReady")}</strong><small>{t("private.notActive")}</small></div>
          <button type="button" disabled={busy} onClick={createPreview}>{busy ? t("private.validating") : t("private.review")}</button>
        </div>
      )}

      {preview && (
        <div className="private-preview" aria-live="polite">
          <div><Check size={16} /><strong>{t("private.valid")}</strong></div>
          <details open><summary>{t("private.showGenerated")}</summary><pre>{privateSection(preview.rendered_config)}</pre></details>
          <button type="button" disabled={busy || !preview.changed} onClick={activate}>{busy ? t("private.activating") : t("private.activate")}</button>
        </div>
      )}
    </section>
  );
}

function defaultReverseZones(): UnboundReverseZonePolicy[] {
  return reverseNetworks.map((network) => ({ network, mode: "nxdomain" }));
}

function normalizeSettings(settings: UnboundSettings): UnboundSettings {
  const policies = new Map((settings.reverse_zones ?? []).map((policy) => [policy.network, policy.mode]));
  return {
    ...settings,
    forward_zones: settings.forward_zones ?? [],
    private_domains: settings.private_domains ?? [],
    reverse_zones: reverseNetworks.map((network) => ({ network, mode: policies.get(network) ?? "nxdomain" })),
  };
}

function sameSettings(left: UnboundSettings, right: UnboundSettings) {
  return JSON.stringify(normalizeSettings(left)) === JSON.stringify(normalizeSettings(right));
}

function normalizeDomain(value: string, t: (key: string) => string) {
  const normalized = value.trim().toLowerCase().replace(/\.*$/, "") + ".";
  if (normalized === ".") throw new Error(t("private.validation.root"));
  const labels = normalized.slice(0, -1).split(".");
  if (normalized.length > 254 || !labels.every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label))) {
    throw new Error(t("private.validation.name"));
  }
  return normalized;
}

function privateSection(config: string) {
  const lines = config.split("\n").filter((line) =>
    line.includes("# Private domain:") ||
    line.includes("# RFC1918 reverse DNS:") ||
    line.trimStart().startsWith("private-domain:") ||
    line.trimStart().startsWith("local-zone:")
  );
  return lines.length > 0 ? `server:\n${lines.join("\n")}` : "# No private-domain directives.";
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
