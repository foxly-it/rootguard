import { useMemo, useState } from "react";
import { Check, GlobeLock, Plus, RotateCcw, ShieldAlert, ShieldCheck, Trash2 } from "lucide-react";
import {
  type UnboundReverseZonePolicy,
  type UnboundSettings,
} from "../api/client";
import { errorMessage, useUnboundDraftWorkflow } from "../hooks/useUnboundDraftWorkflow";
import GuidedFlowSteps from "./GuidedFlowSteps";
import { useI18n } from "../i18n";
import "../styles/unbound-private.css";

const maxPrivateDomains = 32;
const reverseNetworks: UnboundReverseZonePolicy["network"][] = [
  "10.0.0.0/8",
  "172.16.0.0/12",
  "192.168.0.0/16",
];

export default function UnboundPrivateDomains({
  id,
  version,
  onActivated,
}: {
  id?: string;
  version?: string;
  onActivated: () => Promise<void>;
}) {
  const { t } = useI18n();
  const [domains, setDomains] = useState<string[]>([]);
  const [reverseZones, setReverseZones] = useState<UnboundReverseZonePolicy[]>(defaultReverseZones);
  const [draft, setDraft] = useState("home.arpa.");

  const workflow = useUnboundDraftWorkflow({
    version,
    onActivated,
    loadErrorMessage: t("private.loadError"),
    concurrentMessage: t("private.concurrent"),
    previewRejectedMessage: t("private.previewRejected"),
    confirmActivateMessage: t("private.confirmActivate"),
    activateErrorMessage: t("private.activateError"),
    normalize: normalizeSettings,
    onLoad: (settings) => {
      setDomains(structuredClone(settings.private_domains));
      setReverseZones(structuredClone(settings.reverse_zones));
    },
  });

  const dirty = useMemo(() => workflow.source !== null && (
    JSON.stringify(domains) !== JSON.stringify(workflow.source.private_domains) ||
    JSON.stringify(reverseZones) !== JSON.stringify(workflow.source.reverse_zones)
  ), [domains, reverseZones, workflow.source]);

  function addDomain() {
    workflow.setError("");
    try {
      const domain = normalizeDomain(draft, t);
      if (domains.includes(domain)) throw new Error(t("private.duplicate", { name: domain }));
      if (domains.length >= maxPrivateDomains) throw new Error(t("private.limit", { count: maxPrivateDomains }));
      setDomains([...domains, domain]);
      setDraft("");
      workflow.resetPreview();
      workflow.setMessage(t("private.draftSaved"));
    } catch (cause) {
      workflow.setError(errorMessage(cause, t("private.invalid")));
    }
  }

  function removeDomain(domain: string) {
    if (!window.confirm(t("private.confirmRemove", { name: domain }))) return;
    setDomains((current) => current.filter((item) => item !== domain));
    workflow.resetPreview();
    workflow.setMessage(t("private.removed"));
  }

  function setReverseMode(network: UnboundReverseZonePolicy["network"], mode: UnboundReverseZonePolicy["mode"]) {
    setReverseZones((current) => current.map((policy) => policy.network === network ? { ...policy, mode } : policy));
    workflow.resetPreview();
    workflow.setMessage("");
  }

  async function createPreview() {
    const result = await workflow.createPreview((active) => ({ ...active, private_domains: domains, reverse_zones: reverseZones }));
    if (result) workflow.setMessage(t("private.previewAccepted"));
  }

  async function activate() {
    if (await workflow.activate()) workflow.setMessage(t("private.activated"));
  }

  return (
    <section id={id} className="glass-card private-domains-panel" tabIndex={-1}>
      <div className="panel-heading private-heading">
        <div>
          <p className="unbound-eyebrow">{t("private.eyebrow")}</p>
          <h2>{t("private.title")}</h2>
          <p className="muted-copy">{t("private.intro")}</p>
        </div>
        <span className="private-protection"><ShieldCheck size={15} /> {t("private.scoped")}</span>
      </div>

      <GuidedFlowSteps steps={[
        { label: t("private.step1"), active: !workflow.preview },
        { label: t("private.step2"), active: Boolean(workflow.preview) },
        { label: t("private.step3"), active: false },
      ]} />

      {workflow.message && <div className="feedback success">{workflow.message}</div>}
      {workflow.error && <div className="feedback error" role="alert">{workflow.error}</div>}

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
        <button className="rg-button rg-button-secondary secondary-action unbound-action private-domain-add" type="button" disabled={!draft.trim() || domains.length >= maxPrivateDomains} onClick={addDomain} aria-label={t("private.add")} title={t("private.add")}><Plus size={15} /> <span>{t("private.add")}</span></button>
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
        <button className="icon-action" type="button" disabled={workflow.busy} onClick={() => workflow.load().catch((cause: unknown) => workflow.setError(errorMessage(cause, t("private.loadError"))))} aria-label={t("common.refresh")} title={t("common.refresh")}><RotateCcw size={17} /></button>
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
          <button className="rg-button rg-button-primary unbound-action primary" type="button" disabled={workflow.busy} onClick={createPreview}><Check size={15} /><span>{workflow.busy ? t("private.validating") : t("private.review")}</span></button>
        </div>
      )}

      {workflow.preview && (
        <div className="private-preview" aria-live="polite">
          <div><Check size={16} /><strong>{t("private.valid")}</strong></div>
          <details open><summary>{t("private.showGenerated")}</summary><pre tabIndex={0} aria-label={t("private.showGenerated")}>{privateSection(workflow.preview.rendered_config)}</pre></details>
          <button className="rg-button rg-button-primary" type="button" disabled={workflow.busy || !workflow.preview.changed} onClick={activate}>{workflow.busy ? t("private.activating") : t("private.activate")}</button>
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
