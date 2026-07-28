import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, ChevronRight, CirclePlus, MapPin, Pencil, Plus, Trash2 } from "lucide-react";
import {
  fetchUnboundCustom,
  previewUnboundCustom,
  updateUnboundCustom,
  type UnboundCustomPreview,
} from "../api/client";
import "../styles/unbound-guided.css";
import { useI18n } from "../i18n";

const beginMarker = "# BEGIN ROOTGUARD GUIDED LOCAL ZONES";
const endMarker = "# END ROOTGUARD GUIDED LOCAL ZONES";
const zoneMetadataPrefix = "# rootguard-zone: ";

type RecordType = "A" | "AAAA" | "CNAME";

interface LocalRecord {
  name: string;
  type: RecordType;
  value: string;
  ttl: number;
  create_ptr?: boolean;
}

interface LocalZone {
  zone: string;
  records: LocalRecord[];
}

const emptyRecord = (): LocalRecord => ({ name: "router", type: "A", value: "192.168.1.1", ttl: 300, create_ptr: true });
const emptyZone = (): LocalZone => ({ zone: "home.arpa", records: [emptyRecord()] });

export default function UnboundGuidedZones({
  version,
  onActivated,
}: {
  version?: string;
  onActivated: () => Promise<void>;
}) {
  const { t } = useI18n();
  const [source, setSource] = useState("");
  const [base, setBase] = useState("");
  const [zones, setZones] = useState<LocalZone[]>([]);
  const [draft, setDraft] = useState<LocalZone>(emptyZone);
  const [editing, setEditing] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const [preview, setPreview] = useState<UnboundCustomPreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const document = await fetchUnboundCustom();
    const parsed = parseGuidedZones(document.content);
    setSource(document.content);
    setBase(parsed.base);
    setZones(parsed.zones);
    setPreview(null);
    setError("");
  }, []);

  useEffect(() => {
    load().catch((cause: unknown) => setError(errorMessage(cause, t("zones.loadError"))));
  }, [load, t, version]);

  const candidate = useMemo(() => renderGuidedZones(base, zones), [base, zones]);

  function saveDraft() {
    setError("");
    try {
      const normalized = normalizeZone(draft);
      const duplicate = zones.some((zone, index) => zone.zone === normalized.zone && index !== editing);
      if (duplicate) throw new Error(`Die Zone ${normalized.zone} ist bereits vorhanden.`);
      const next = editing === null
        ? [...zones, normalized]
        : zones.map((zone, index) => index === editing ? normalized : zone);
      validatePTRUniqueness(next, t);
      setZones(next);
      setDraft(emptyZone());
      setEditing(null);
      setOpen(false);
      setPreview(null);
      setMessage(t("zones.draftAdded"));
    } catch (cause) {
      setError(errorMessage(cause, t("zones.invalid")));
    }
  }

  function editZone(index: number) {
    setDraft(structuredClone(zones[index]));
    setEditing(index);
    setOpen(true);
    setPreview(null);
    setMessage("");
    setError("");
  }

  function removeZone(index: number) {
    if (!window.confirm(`Lokale Zone ${zones[index].zone} aus dem Entwurf entfernen?`)) return;
    setZones((current) => current.filter((_, zoneIndex) => zoneIndex !== index));
    setPreview(null);
    setMessage(t("zones.removed"));
  }

  function updateRecord(index: number, patch: Partial<LocalRecord>) {
    setDraft((current) => ({
      ...current,
      records: current.records.map((record, recordIndex) => recordIndex === index ? { ...record, ...patch } : record),
    }));
  }

  async function createPreview() {
    if (busy) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const result = await previewUnboundCustom(candidate);
      setPreview(result);
      setMessage(t("zones.previewAccepted"));
    } catch (cause) {
      setPreview(null);
      setError(errorMessage(cause, t("zones.previewRejected")));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!preview?.changed || busy || !window.confirm(t("zones.confirmActivate"))) return;
    setBusy(true);
    setError("");
    try {
      const current = await fetchUnboundCustom();
      if (current.content !== source) {
        throw new Error(t("zones.concurrent"));
      }
      const document = await updateUnboundCustom(preview.content);
      const parsed = parseGuidedZones(document.content);
      setSource(document.content);
      setBase(parsed.base);
      setZones(parsed.zones);
      setPreview(null);
      await onActivated();
      setMessage(t("zones.activated"));
    } catch (cause) {
      setError(errorMessage(cause, t("zones.activateError")));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="glass-card guided-zones-panel">
      <div className="panel-heading guided-heading">
        <div>
          <p className="unbound-eyebrow">{t("zones.eyebrow")}</p>
          <h2>{t("zones.title")}</h2>
          <p className="muted-copy">{t("zones.intro")}</p>
        </div>
        <button className="secondary-action" type="button" disabled={busy} onClick={() => {
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
      {error && <div className="feedback error">{error}</div>}

      {open && (
        <div className="zone-wizard">
          <div className="wizard-intro">
            <MapPin size={20} />
            <div><strong>{editing === null ? t("zones.new") : t("zones.edit")}</strong><small>{t("zones.homeArpa")}</small></div>
          </div>

          <label className="guided-field">
            <span>{t("zones.name")} <small>{t("zones.nameExample")}</small></span>
            <input value={draft.zone} onChange={(event) => setDraft({ ...draft, zone: event.target.value })} placeholder="home.arpa" />
          </label>

          <div className="record-heading"><strong>{t("zones.records")}</strong><small>{t("zones.relativeHelp")}</small></div>
          <div className="guided-records">
            {draft.records.map((record, index) => (
              <div className="guided-record" key={index}>
                <label><span>{t("zones.recordName")}</span><input value={record.name} onChange={(event) => updateRecord(index, { name: event.target.value })} placeholder="router" /></label>
                <label><span>{t("zones.type")}</span><select value={record.type} onChange={(event) => {
                  const type = event.target.value as RecordType;
                  updateRecord(index, { type, create_ptr: type === "CNAME" ? false : record.create_ptr });
                }}><option>A</option><option>AAAA</option><option>CNAME</option></select></label>
                <label className="record-value"><span>{record.type === "CNAME" ? t("zones.targetName") : t("zones.address")}</span><input value={record.value} onChange={(event) => updateRecord(index, { value: event.target.value })} placeholder={record.type === "AAAA" ? "fd00::1" : record.type === "CNAME" ? "server.home.arpa" : "192.168.1.1"} /></label>
                <label><span>{t("zones.ttl")}</span><input type="number" min={30} max={86400} value={record.ttl} onChange={(event) => updateRecord(index, { ttl: Number(event.target.value) })} /></label>
                {record.type !== "CNAME" && <label className="record-ptr"><input type="checkbox" checked={record.create_ptr ?? false} onChange={(event) => updateRecord(index, { create_ptr: event.target.checked })} /><span><b>PTR</b><small>{t("zones.ptrHelp")}</small></span></label>}
                <button className="record-delete" type="button" aria-label={t("zones.deleteRecord")} disabled={draft.records.length === 1} onClick={() => setDraft({ ...draft, records: draft.records.filter((_, recordIndex) => recordIndex !== index) })}><Trash2 size={15} /></button>
              </div>
            ))}
          </div>
          <div className="wizard-actions">
            <button className="text-action" type="button" onClick={() => setDraft({ ...draft, records: [...draft.records, emptyRecord()] })}><CirclePlus size={15} /> {t("zones.addRecord")}</button>
            <button type="button" onClick={saveDraft}>{editing === null ? t("zones.addDraft") : t("zones.applyEdit")}</button>
          </div>
        </div>
      )}

      <div className="guided-zone-list">
        {zones.length === 0 && <div className="guided-empty"><MapPin size={22} /><div><strong>{t("zones.empty")}</strong><p>{t("zones.emptyHelp")}</p></div></div>}
        {zones.map((zone, index) => (
          <article key={zone.zone}>
            <div className="zone-name"><span><MapPin size={15} /></span><div><strong>{zone.zone}</strong><small>{zone.records.length === 1 ? t("zones.oneRecord") : t("zones.manyRecords", { count: zone.records.length })}</small></div></div>
            <div className="zone-record-summary">{zone.records.map((record) => <code key={`${record.name}-${record.type}`}>{record.name} · {record.type} · {record.value}{record.create_ptr ? " · PTR" : ""}</code>)}</div>
            <div className="zone-actions"><button type="button" onClick={() => editZone(index)}><Pencil size={14} /> {t("common.edit")}</button><button type="button" onClick={() => removeZone(index)}><Trash2 size={14} /> {t("common.remove")}</button></div>
          </article>
        ))}
      </div>

      {candidate !== source && !open && (
        <div className="guided-review">
          <div><strong>{t("zones.draftReady")}</strong><small>{t("zones.notActive")}</small></div>
          <button type="button" disabled={busy} onClick={createPreview}>{busy ? t("zones.validating") : t("zones.validate")}</button>
        </div>
      )}

      {preview && (
        <div className="guided-preview">
          <div className="guided-preview-state"><Check size={16} /><strong>{t("zones.valid")}</strong></div>
          <pre>{guidedSection(preview.content) || "# Alle geführten lokalen Zonen werden entfernt."}</pre>
          <button type="button" disabled={busy || !preview.changed} onClick={activate}>{busy ? t("zones.activating") : preview.changed ? t("zones.activate") : t("zones.alreadyActive")}</button>
        </div>
      )}
    </section>
  );
}

function FlowStep({ number, label, active }: { number: string; label: string; active: boolean }) {
  return <span className={active ? "active" : ""}><i>{number}</i>{label}</span>;
}

function parseGuidedZones(content: string): { base: string; zones: LocalZone[] } {
  const start = content.indexOf(beginMarker);
  const end = content.indexOf(endMarker);
  if (start < 0 && end < 0) return { base: content, zones: [] };
  if (start < 0 || end < start) throw new Error("Der geführte Zonenblock ist unvollständig. Bitte im Experteneditor prüfen.");
  const sectionEnd = end + endMarker.length;
  const section = content.slice(start, sectionEnd);
  const zones: LocalZone[] = [];
  for (const line of section.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed.startsWith(zoneMetadataPrefix)) continue;
    try {
      zones.push(normalizeZone(JSON.parse(trimmed.slice(zoneMetadataPrefix.length)) as LocalZone));
    } catch {
      throw new Error("Metadaten einer geführten Zone sind ungültig. Bitte im Experteneditor prüfen.");
    }
  }
  const before = content.slice(0, start).trimEnd();
  const after = content.slice(sectionEnd).trimStart();
  const base = [before, after].filter(Boolean).join("\n\n");
  return { base: base ? base + "\n" : "", zones };
}

function renderGuidedZones(base: string, zones: LocalZone[]) {
  const cleanBase = base.trim();
  if (zones.length === 0) return cleanBase ? cleanBase + "\n" : "";
  const ptrCounts = new Map<string, number>();
  for (const zone of zones) {
    for (const record of zone.records) {
      if (record.create_ptr && record.type !== "CNAME") {
        ptrCounts.set(record.value, (ptrCounts.get(record.value) ?? 0) + 1);
      }
    }
  }
  const lines = [beginMarker, "# Generated by RootGuard. Use the guided UI to edit this block.", "server:"];
  for (const zone of zones) {
    lines.push(`    ${zoneMetadataPrefix}${JSON.stringify(zone)}`);
    lines.push(`    # Local zone ${zone.zone}`);
    lines.push(`    local-zone: "${zone.zone}" static`);
    for (const record of zone.records) {
      lines.push(`    local-data: "${absoluteRecordName(record.name, zone.zone)} ${record.ttl} IN ${record.type} ${recordValue(record)}"`);
      if (record.create_ptr && record.type !== "CNAME" && ptrCounts.get(record.value) === 1) {
        lines.push(`    local-data-ptr: "${record.value} ${absoluteRecordName(record.name, zone.zone)}"`);
      }
    }
  }
  lines.push(endMarker);
  return (cleanBase ? cleanBase + "\n\n" : "") + lines.join("\n") + "\n";
}

function normalizeZone(zone: LocalZone): LocalZone {
  const name = normalizeDNSName(zone.zone, "Zonenname");
  if (zone.records.length === 0) throw new Error("Mindestens ein DNS-Eintrag ist erforderlich.");
  const records = zone.records.map((record) => {
    const recordName = record.name.trim();
    if (recordName !== "@") normalizeDNSName(absoluteRecordName(recordName, name), "Hostname");
    if (!Number.isInteger(record.ttl) || record.ttl < 30 || record.ttl > 86400) throw new Error("TTL muss zwischen 30 und 86.400 Sekunden liegen.");
    const value = record.value.trim();
    if (record.type === "A" && !validIPv4(value)) throw new Error(`${record.name}: Bitte eine gültige IPv4-Adresse eintragen.`);
    if (record.type === "AAAA" && !validIPv6(value)) throw new Error(`${record.name}: Bitte eine gültige IPv6-Adresse eintragen.`);
    if (record.type === "CNAME") normalizeDNSName(value, `${record.name}: CNAME-Ziel`);
    const normalized: LocalRecord = { name: recordName, type: record.type, value, ttl: record.ttl };
    if (record.create_ptr !== undefined) normalized.create_ptr = record.type !== "CNAME" && record.create_ptr;
    return normalized;
  });
  const keys = new Set<string>();
  for (const record of records) {
    const key = `${absoluteRecordName(record.name, name)}|${record.type}`;
    if (keys.has(key)) throw new Error(`Der Eintrag ${record.name} (${record.type}) ist doppelt vorhanden.`);
    keys.add(key);
  }
  return { zone: name, records };
}

function validatePTRUniqueness(zones: LocalZone[], t: (key: string, values?: Record<string, string | number>) => string) {
  const addresses = new Set<string>();
  for (const zone of zones) {
    for (const record of zone.records) {
      if (!record.create_ptr || record.type === "CNAME") continue;
      if (addresses.has(record.value)) throw new Error(t("zones.ptrDuplicate", { address: record.value }));
      addresses.add(record.value);
    }
  }
}

function normalizeDNSName(value: string, label: string) {
  const normalized = value.trim().toLowerCase().replace(/\.$/, "") + ".";
  if (normalized.length > 254 || !normalized.slice(0, -1).split(".").every((part) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(part))) {
    throw new Error(`${label} ist kein gültiger DNS-Name.`);
  }
  return normalized;
}

function absoluteRecordName(name: string, zone: string) {
  if (name.trim() === "@") return zone;
  const clean = name.trim().toLowerCase();
  const normalizedZone = zone.endsWith(".") ? zone : zone + ".";
  if (clean.endsWith(".")) return clean;
  if ((clean + ".").endsWith(normalizedZone)) return clean + ".";
  return clean + "." + normalizedZone;
}

function recordValue(record: LocalRecord) {
  return record.type === "CNAME" ? normalizeDNSName(record.value, "CNAME-Ziel") : record.value;
}

function validIPv4(value: string) {
  const parts = value.split(".");
  return parts.length === 4 && parts.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255);
}

function validIPv6(value: string) {
  return value.includes(":") && /^[0-9a-f:]+$/i.test(value) && value.length <= 45;
}

function guidedSection(content: string) {
  const start = content.indexOf(beginMarker);
  const end = content.indexOf(endMarker);
  return start >= 0 && end >= start ? content.slice(start, end + endMarker.length) : "";
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
