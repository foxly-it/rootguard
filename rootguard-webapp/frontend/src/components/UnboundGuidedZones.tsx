import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, ChevronRight, CirclePlus, MapPin, Pencil, Plus, Trash2 } from "lucide-react";
import {
  fetchUnboundSettings,
  previewUnboundSettings,
  updateUnboundSettings,
  type UnboundLocalHost,
  type UnboundLocalZone,
  type UnboundPreview,
  type UnboundSettings,
} from "../api/client";
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
  const [source, setSource] = useState<UnboundSettings | null>(null);
  const [zones, setZones] = useState<UnboundLocalZone[]>([]);
  const [draft, setDraft] = useState<DraftZone>(emptyZone);
  const [editing, setEditing] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const [preview, setPreview] = useState<UnboundPreview | null>(null);
  const [candidate, setCandidate] = useState<UnboundSettings | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const settings = await fetchUnboundSettings();
    setSource(settings);
    setZones(structuredClone(settings.local_zones ?? []));
    setPreview(null);
    setCandidate(null);
    setError("");
  }, []);

  useEffect(() => {
    load().catch((cause: unknown) => setError(errorMessage(cause, t("zones.loadError"))));
  }, [load, t, version]);

  const dirty = useMemo(
    () => source !== null && JSON.stringify(zones) !== JSON.stringify(source.local_zones ?? []),
    [zones, source],
  );

  function saveDraft() {
    setError("");
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
      setPreview(null);
      setCandidate(null);
      setMessage(t("zones.draftAdded"));
    } catch (cause) {
      setError(errorMessage(cause, t("zones.invalid")));
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
    setPreview(null);
    setCandidate(null);
    setMessage("");
    setError("");
  }

  function removeZone(index: number) {
    if (!window.confirm(t("zones.confirmRemove", { name: zones[index].name }))) return;
    setZones((current) => current.filter((_, zoneIndex) => zoneIndex !== index));
    setPreview(null);
    setCandidate(null);
    setMessage(t("zones.removed"));
  }

  function updateHost(index: number, patch: Partial<DraftHost>) {
    setDraft((current) => ({
      ...current,
      hosts: current.hosts.map((host, hostIndex) => (hostIndex === index ? { ...host, ...patch } : host)),
    }));
  }

  async function createPreview() {
    if (!source || busy) return;
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const active = await fetchUnboundSettings();
      if (!sameZones(active.local_zones, source.local_zones)) throw new Error(t("zones.concurrent"));
      const proposed = { ...active, local_zones: zones };
      const result = await previewUnboundSettings(proposed);
      setCandidate(proposed);
      setPreview(result);
      setMessage(t("zones.previewAccepted"));
    } catch (cause) {
      setPreview(null);
      setCandidate(null);
      setError(errorMessage(cause, t("zones.previewRejected")));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!source || !candidate || !preview?.changed || busy) return;
    if (!window.confirm(t("zones.confirmActivate"))) return;
    setBusy(true);
    setError("");
    try {
      const active = await fetchUnboundSettings();
      if (!sameZones(active.local_zones, source.local_zones)) throw new Error(t("zones.concurrent"));
      await updateUnboundSettings(candidate);
      await onActivated();
      await load();
      setMessage(t("zones.activated"));
    } catch (cause) {
      setError(errorMessage(cause, t("zones.activateError")));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section id={id} className="glass-card guided-zones-panel" tabIndex={-1}>
      <div className="panel-heading guided-heading">
        <div>
          <p className="unbound-eyebrow">{t("zones.eyebrow")}</p>
          <h2>{t("zones.title")}</h2>
          <p className="muted-copy">{t("zones.intro")}</p>
        </div>
        <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy} onClick={() => {
          setDraft(emptyZone());
          setEditing(null);
          setOpen(!open);
          setError("");
        }}>
          <Plus size={15} /> {open ? t("zones.close") : t("zones.add")}
        </button>
      </div>

      <div className="guided-flow">
        <FlowStep number="1" label={t("zones.step1")} active={open} />
        <ChevronRight size={16} />
        <FlowStep number="2" label={t("zones.step2")} active={Boolean(preview)} />
        <ChevronRight size={16} />
        <FlowStep number="3" label={t("zones.step3")} active={false} />
      </div>

      {message && <div className="feedback success">{message}</div>}
      {error && <div className="feedback error" role="alert">{error}</div>}

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
          <button className="rg-button rg-button-primary" type="button" disabled={busy} onClick={createPreview}>{busy ? t("zones.validating") : t("zones.validate")}</button>
        </div>
      )}

      {preview && (
        <div className="guided-preview" aria-live="polite">
          <div className="guided-preview-state"><Check size={16} /><strong>{t("zones.valid")}</strong></div>
          <details open>
            <summary>{t("zones.showGenerated")}</summary>
            <pre tabIndex={0} aria-label={t("zones.showGenerated")}>{localZoneSection(preview.rendered_config)}</pre>
          </details>
          <button className="rg-button rg-button-primary" type="button" disabled={busy || !preview.changed} onClick={activate}>{busy ? t("zones.activating") : preview.changed ? t("zones.activate") : t("zones.alreadyActive")}</button>
        </div>
      )}
    </section>
  );
}

function FlowStep({ number, label, active }: { number: string; label: string; active: boolean }) {
  return <span className={active ? "active" : ""}><i>{number}</i>{label}</span>;
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

function sameZones(left?: UnboundLocalZone[], right?: UnboundLocalZone[]): boolean {
  return JSON.stringify(left ?? []) === JSON.stringify(right ?? []);
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

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}
