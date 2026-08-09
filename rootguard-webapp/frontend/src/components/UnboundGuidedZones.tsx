import { useMemo, useState } from "react";
import { Check, CirclePlus, MapPin, Pencil, Plus, Trash2 } from "lucide-react";
import {
  type UnboundLocalHost,
  type UnboundLocalZone,
} from "../api/client";
import { errorMessage, useUnboundDraftWorkflow } from "../hooks/useUnboundDraftWorkflow";
import GuidedFlowSteps from "./GuidedFlowSteps";
import "../styles/unbound-guided.css";
import { useI18n } from "../i18n";

const hostLabelPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

interface DraftHost {
  hostname: string;
  ipv4: string;
  ipv6: string;
  ptr: boolean;
}

interface DraftZone {
  name: string;
  hosts: DraftHost[];
}

const emptyHost = (): DraftHost => ({ hostname: "router", ipv4: "192.168.1.1", ipv6: "", ptr: true });
const emptyZone = (): DraftZone => ({ name: "home.arpa", hosts: [emptyHost()] });

export default function UnboundGuidedZones({
  id,
  version,
  onActivated,
}: {
  id?: string;
  version?: string;
  onActivated: () => Promise<void>;
}) {
  const { t } = useI18n();
  const [zones, setZones] = useState<UnboundLocalZone[]>([]);
  const [draft, setDraft] = useState<DraftZone>(emptyZone);
  const [editing, setEditing] = useState<number | null>(null);
  const [open, setOpen] = useState(false);

  const workflow = useUnboundDraftWorkflow({
    version,
    onActivated,
    loadErrorMessage: t("zones.loadError"),
    concurrentMessage: t("zones.concurrent"),
    previewRejectedMessage: t("zones.previewRejected"),
    confirmActivateMessage: t("zones.confirmActivate"),
    activateErrorMessage: t("zones.activateError"),
    sameSettings: (a, b) => JSON.stringify(a.local_zones ?? []) === JSON.stringify(b.local_zones ?? []),
    onLoad: (settings) => setZones(structuredClone(settings.local_zones ?? [])),
  });

  const dirty = useMemo(
    () => workflow.source !== null && JSON.stringify(zones) !== JSON.stringify(workflow.source.local_zones ?? []),
    [zones, workflow.source],
  );

  function saveDraft() {
    workflow.setError("");
    try {
      const normalized = normalizeDraftZone(draft, t);
      const duplicate = zones.some((zone, index) => zone.name === normalized.name && index !== editing);
      if (duplicate) throw new Error(t("zones.validation.duplicateZone", { name: normalized.name }));
      const next = editing === null
        ? [...zones, normalized]
        : zones.map((zone, index) => (index === editing ? normalized : zone));
      validatePTRUniqueness(next, t);
      setZones(next);
      setDraft(emptyZone());
      setEditing(null);
      setOpen(false);
      workflow.resetPreview();
      workflow.setMessage(t("zones.draftAdded"));
    } catch (cause) {
      workflow.setError(errorMessage(cause, t("zones.invalid")));
    }
  }

  function editZone(index: number) {
    const zone = zones[index];
    setDraft({
      name: zone.name,
      hosts: zone.hosts.map((host) => ({ hostname: host.hostname, ipv4: host.ipv4 ?? "", ipv6: host.ipv6 ?? "", ptr: host.ptr })),
    });
    setEditing(index);
    setOpen(true);
    workflow.resetPreview();
    workflow.setMessage("");
    workflow.setError("");
  }

  function removeZone(index: number) {
    if (!window.confirm(t("zones.confirmRemove", { name: zones[index].name }))) return;
    setZones((current) => current.filter((_, zoneIndex) => zoneIndex !== index));
    workflow.resetPreview();
    workflow.setMessage(t("zones.removed"));
  }

  function updateHost(index: number, patch: Partial<DraftHost>) {
    setDraft((current) => ({
      ...current,
      hosts: current.hosts.map((host, hostIndex) => (hostIndex === index ? { ...host, ...patch } : host)),
    }));
  }

  async function createPreview() {
    const result = await workflow.createPreview((active) => ({ ...active, local_zones: zones }));
    if (result) workflow.setMessage(t("zones.previewAccepted"));
  }

  async function activate() {
    if (await workflow.activate()) workflow.setMessage(t("zones.activated"));
  }

  return (
    <section id={id} className="glass-card guided-zones-panel" tabIndex={-1}>
      <div className="panel-heading guided-heading">
        <div>
          <p className="unbound-eyebrow">{t("zones.eyebrow")}</p>
          <h2>{t("zones.title")}</h2>
          <p className="muted-copy">{t("zones.intro")}</p>
        </div>
        <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={workflow.busy} onClick={() => {
          setDraft(emptyZone());
          setEditing(null);
          setOpen(!open);
          workflow.setError("");
        }}>
          <Plus size={15} /> {open ? t("zones.close") : t("zones.add")}
        </button>
      </div>

      <GuidedFlowSteps steps={[
        { label: t("zones.step1"), active: !workflow.preview },
        { label: t("zones.step2"), active: Boolean(workflow.preview) },
        { label: t("zones.step3"), active: false },
      ]} />

      {workflow.message && <div className="feedback success">{workflow.message}</div>}
      {workflow.error && <div className="feedback error" role="alert">{workflow.error}</div>}

      {open && (
        <div className="zone-wizard">
          <div className="wizard-intro">
            <MapPin size={20} />
            <div><strong>{editing === null ? t("zones.new") : t("zones.edit")}</strong><small>{t("zones.homeArpa")}</small></div>
          </div>

          <label className="guided-field">
            <span>{t("zones.name")} <small>{t("zones.nameExample")}</small></span>
            <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} placeholder="home.arpa" autoCapitalize="none" spellCheck={false} />
          </label>

          <div className="record-heading"><strong>{t("zones.hosts")}</strong><small>{t("zones.hostsHelp")}</small></div>
          <div className="guided-records">
            {draft.hosts.map((host, index) => (
              <div className="guided-record" key={index}>
                <label><span>{t("zones.hostname")}</span><input value={host.hostname} onChange={(event) => updateHost(index, { hostname: event.target.value })} placeholder="router" autoCapitalize="none" spellCheck={false} /></label>
                <label><span>{t("zones.ipv4")}</span><input value={host.ipv4} onChange={(event) => updateHost(index, { ipv4: event.target.value })} placeholder="192.168.1.1" autoCapitalize="none" spellCheck={false} /></label>
                <label><span>{t("zones.ipv6")}</span><input value={host.ipv6} onChange={(event) => updateHost(index, { ipv6: event.target.value })} placeholder="fd00::1" autoCapitalize="none" spellCheck={false} /></label>
                <label className="record-ptr"><input type="checkbox" checked={host.ptr} onChange={(event) => updateHost(index, { ptr: event.target.checked })} /><span><b>PTR</b><small>{t("zones.ptrHelp")}</small></span></label>
                <button className="record-delete" type="button" aria-label={t("zones.deleteHost")} disabled={draft.hosts.length === 1} onClick={() => setDraft({ ...draft, hosts: draft.hosts.filter((_, hostIndex) => hostIndex !== index) })}><Trash2 size={15} /></button>
              </div>
            ))}
          </div>
          <div className="wizard-actions">
            <button className="text-action" type="button" onClick={() => setDraft({ ...draft, hosts: [...draft.hosts, emptyHost()] })}><CirclePlus size={15} /> {t("zones.addHost")}</button>
            <button className="rg-button rg-button-primary" type="button" onClick={saveDraft}>{editing === null ? t("zones.addDraft") : t("zones.applyEdit")}</button>
          </div>
        </div>
      )}

      <div className="guided-zone-list">
        {zones.length === 0 && <div className="guided-empty"><MapPin size={22} /><div><strong>{t("zones.empty")}</strong><p>{t("zones.emptyHelp")}</p></div></div>}
        {zones.map((zone, index) => (
          <article key={zone.name}>
            <div className="zone-name"><span><MapPin size={15} /></span><div><strong>{zone.name}</strong><small>{zone.hosts.length === 1 ? t("zones.oneHost") : t("zones.manyHosts", { count: zone.hosts.length })}</small></div></div>
            <div className="zone-record-summary">{zone.hosts.map((host) => <code key={host.hostname}>{host.hostname} · {host.ipv4 || host.ipv6}{host.ptr ? " · PTR" : ""}</code>)}</div>
            <div className="zone-actions"><button className="rg-button rg-button-secondary" type="button" onClick={() => editZone(index)}><Pencil size={14} /> {t("common.edit")}</button><button className="rg-button rg-button-danger" type="button" onClick={() => removeZone(index)}><Trash2 size={14} /> {t("common.remove")}</button></div>
          </article>
        ))}
      </div>

      {dirty && !open && (
        <div className="guided-review">
          <div><strong>{t("zones.draftReady")}</strong><small>{t("zones.notActive")}</small></div>
          <button className="rg-button rg-button-primary" type="button" disabled={workflow.busy} onClick={createPreview}>{workflow.busy ? t("zones.validating") : t("zones.validate")}</button>
        </div>
      )}

      {workflow.preview && (
        <div className="guided-preview" aria-live="polite">
          <div className="guided-preview-state"><Check size={16} /><strong>{t("zones.valid")}</strong></div>
          <details open>
            <summary>{t("zones.showGenerated")}</summary>
            <pre tabIndex={0} aria-label={t("zones.showGenerated")}>{localZoneSection(workflow.preview.rendered_config)}</pre>
          </details>
          <button className="rg-button rg-button-primary" type="button" disabled={workflow.busy || !workflow.preview.changed} onClick={activate}>{workflow.busy ? t("zones.activating") : workflow.preview.changed ? t("zones.activate") : t("zones.alreadyActive")}</button>
        </div>
      )}
    </section>
  );
}

function normalizeDraftZone(zone: DraftZone, t: (key: string, values?: Record<string, string | number>) => string): UnboundLocalZone {
  const name = normalizeZoneName(zone.name, t);
  if (zone.hosts.length === 0) throw new Error(t("zones.validation.hostRequired"));
  const seen = new Set<string>();
  const hosts: UnboundLocalHost[] = zone.hosts.map((host) => {
    const hostname = normalizeHostLabel(host.hostname, t);
    if (seen.has(hostname)) throw new Error(t("zones.validation.duplicateHostname", { name: hostname }));
    seen.add(hostname);
    const ipv4 = host.ipv4.trim();
    const ipv6 = host.ipv6.trim();
    if (!ipv4 && !ipv6) throw new Error(t("zones.validation.addressRequired", { name: hostname }));
    if (ipv4 && !validIPv4(ipv4)) throw new Error(t("zones.validation.ipv4", { name: hostname }));
    if (ipv6 && !validIPv6(ipv6)) throw new Error(t("zones.validation.ipv6", { name: hostname }));
    return { hostname, ipv4: ipv4 || undefined, ipv6: ipv6 || undefined, ptr: host.ptr };
  });
  return { name, hosts };
}

function validatePTRUniqueness(zones: UnboundLocalZone[], t: (key: string, values?: Record<string, string | number>) => string) {
  const addresses = new Set<string>();
  for (const zone of zones) {
    for (const host of zone.hosts) {
      if (!host.ptr) continue;
      for (const address of [host.ipv4, host.ipv6]) {
        if (!address) continue;
        if (addresses.has(address)) throw new Error(t("zones.ptrDuplicate", { address }));
        addresses.add(address);
      }
    }
  }
}

function normalizeHostLabel(value: string, t: (key: string) => string): string {
  const normalized = value.trim().toLowerCase();
  if (!hostLabelPattern.test(normalized)) throw new Error(t("zones.validation.hostname"));
  return normalized;
}

function normalizeZoneName(value: string, t: (key: string) => string): string {
  const normalized = value.trim().toLowerCase().replace(/\.*$/, "") + ".";
  if (normalized === ".") throw new Error(t("zones.validation.zoneRoot"));
  const labels = normalized.slice(0, -1).split(".");
  if (normalized.length > 254 || !labels.every((label) => hostLabelPattern.test(label))) {
    throw new Error(t("zones.validation.zoneName"));
  }
  return normalized;
}

function validIPv4(value: string) {
  const parts = value.split(".");
  return parts.length === 4 && parts.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255);
}

function validIPv6(value: string) {
  return value.includes(":") && /^[0-9a-f:]+$/i.test(value) && value.length <= 45;
}

function localZoneSection(config: string): string {
  const lines = config.split("\n").filter((line) =>
    line.includes("# Local host inventory:") ||
    line.trimStart().startsWith("local-zone:") ||
    line.trimStart().startsWith("local-data:") ||
    line.trimStart().startsWith("local-data-ptr:"),
  );
  return lines.length > 0 ? `server:\n${lines.join("\n")}` : "# No local-zone directives.";
}
