import { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowUp, Check, CirclePlus, Network, Pencil, Plus, ShieldAlert, ShieldCheck, Trash2 } from "lucide-react";
import {
  checkUnboundForwardTargets,
  fetchUnboundSettings,
  previewUnboundSettings,
  updateUnboundSettings,
  type UnboundForwardTargetCheck,
  type UnboundForwardZone,
  type UnboundPreview,
  type UnboundSettings,
} from "../api/client";
import { useI18n } from "../i18n";
import "../styles/unbound-forwarding.css";

const maxZones = 32;
const maxServersPerZone = 8;
const maxTargets = 32;

const emptyZone = (): UnboundForwardZone => ({
  name: "corp.example.",
  servers: ["192.168.1.53"],
  forward_first: false,
  allow_unsigned: false,
  allow_private_addresses: false,
});

export default function UnboundForwardZones({
  version,
  onActivated,
}: {
  version?: string;
  onActivated: () => Promise<void>;
}) {
  const { t } = useI18n();
  const [source, setSource] = useState<UnboundSettings | null>(null);
  const [zones, setZones] = useState<UnboundForwardZone[]>([]);
  const [draft, setDraft] = useState<UnboundForwardZone>(emptyZone);
  const [editing, setEditing] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const [preview, setPreview] = useState<UnboundPreview | null>(null);
  const [candidate, setCandidate] = useState<UnboundSettings | null>(null);
  const [checks, setChecks] = useState<UnboundForwardTargetCheck[]>([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    const settings = normalizeSettings(await fetchUnboundSettings());
    setSource(settings);
    setZones(structuredClone(settings.forward_zones));
    setPreview(null);
    setCandidate(null);
    setChecks([]);
    setError("");
  }, []);

  useEffect(() => {
    load().catch((cause: unknown) => setError(errorMessage(cause, t("forward.loadError"))));
  }, [load, t, version]);

  const dirty = useMemo(
    () => source !== null && JSON.stringify(zones) !== JSON.stringify(source.forward_zones),
    [source, zones],
  );
  const allReachable = checks.every((check) => check.reachable);

  function saveDraft() {
    setError("");
    try {
      const normalized = normalizeForwardZone(draft, t);
      if (zones.some((zone, index) => zone.name === normalized.name && index !== editing)) {
        throw new Error(t("forward.duplicate", { name: normalized.name }));
      }
      const next = editing === null
        ? [...zones, normalized]
        : zones.map((zone, index) => index === editing ? normalized : zone);
      validateLimits(next, t);
      setZones(next);
      setDraft(emptyZone());
      setEditing(null);
      setOpen(false);
      resetPreview();
      setMessage(t("forward.draftSaved"));
    } catch (cause) {
      setError(errorMessage(cause, t("forward.invalid")));
    }
  }

  function editZone(index: number) {
    setDraft(structuredClone(zones[index]));
    setEditing(index);
    setOpen(true);
    resetPreview();
    setMessage("");
    setError("");
  }

  function removeZone(index: number) {
    if (!window.confirm(t("forward.confirmRemove", { name: zones[index].name }))) return;
    setZones((current) => current.filter((_, zoneIndex) => zoneIndex !== index));
    resetPreview();
    setMessage(t("forward.removed"));
  }

  function updateServer(index: number, value: string) {
    setDraft((current) => ({
      ...current,
      servers: current.servers.map((server, serverIndex) => serverIndex === index ? value : server),
    }));
  }

  function moveServer(index: number, direction: -1 | 1) {
    const destination = index + direction;
    if (destination < 0 || destination >= draft.servers.length) return;
    setDraft((current) => {
      const servers = [...current.servers];
      [servers[index], servers[destination]] = [servers[destination], servers[index]];
      return { ...current, servers };
    });
  }

  async function createPreview() {
    if (!source || busy) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      validateLimits(zones, t);
      const active = normalizeSettings(await fetchUnboundSettings());
      if (!sameSettings(active, source)) throw new Error(t("forward.concurrent"));
      const proposed = { ...active, forward_zones: zones };
      const [configurationPreview, targetChecks] = await Promise.all([
        previewUnboundSettings(proposed),
        checkUnboundForwardTargets(zones),
      ]);
      setCandidate(proposed);
      setPreview(configurationPreview);
      setChecks(targetChecks);
      setMessage(targetChecks.every((check) => check.reachable)
        ? t("forward.previewAccepted")
        : t("forward.previewUnreachable"));
    } catch (cause) {
      resetPreview();
      setError(errorMessage(cause, t("forward.previewRejected")));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!source || !candidate || !preview?.changed || !allReachable || busy) return;
    if (!window.confirm(t("forward.confirmActivate"))) return;
    setBusy(true);
    setError("");
    try {
      const active = normalizeSettings(await fetchUnboundSettings());
      if (!sameSettings(active, source)) throw new Error(t("forward.concurrent"));
      await updateUnboundSettings(candidate);
      await onActivated();
      await load();
      setMessage(t("forward.activated"));
    } catch (cause) {
      setError(errorMessage(cause, t("forward.activateError")));
    } finally {
      setBusy(false);
    }
  }

  function resetPreview() {
    setPreview(null);
    setCandidate(null);
    setChecks([]);
  }

  return (
    <section className="glass-card forward-zones-panel">
      <div className="panel-heading forward-heading">
        <div>
          <p className="unbound-eyebrow">{t("forward.eyebrow")}</p>
          <h2>{t("forward.title")}</h2>
          <p className="muted-copy">{t("forward.intro")}</p>
        </div>
        <button className="secondary-action unbound-action" type="button" disabled={busy || (!open && zones.length >= maxZones)} onClick={() => {
          setDraft(emptyZone());
          setEditing(null);
          setOpen(!open);
          setError("");
        }}>
          <Plus size={15} /> <span>{open ? t("common.close") : t("forward.add")}</span>
        </button>
      </div>

      {message && <div className={`feedback ${checks.length > 0 && !allReachable ? "error" : "success"}`}>{message}</div>}
      {error && <div className="feedback error" role="alert">{error}</div>}

      {open && (
        <div className="forward-wizard">
          <div className="wizard-intro">
            <Network size={20} />
            <div><strong>{editing === null ? t("forward.new") : t("forward.edit")}</strong><small>{t("forward.zoneHelp")}</small></div>
          </div>
          <label className="guided-field">
            <span>{t("forward.zone")} <small>{t("forward.zoneExample")}</small></span>
            <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} placeholder="corp.example." autoCapitalize="none" spellCheck={false} />
          </label>
          <div className="forward-server-heading">
            <div><strong>{t("forward.servers")}</strong><small>{t("forward.serversHelp")}</small></div>
            <button className="secondary-action unbound-action" type="button" disabled={draft.servers.length >= maxServersPerZone} onClick={() => setDraft({ ...draft, servers: [...draft.servers, ""] })}><CirclePlus size={15} /> <span>{t("forward.addServer")}</span></button>
          </div>
          <div className="forward-servers">
            {draft.servers.map((server, index) => (
              <div key={index} className="forward-server-row">
                <span aria-hidden="true">{index + 1}</span>
                <label><span className="sr-only">{t("forward.serverNumber", { number: index + 1 })}</span><input value={server} onChange={(event) => updateServer(index, event.target.value)} placeholder={index === 0 ? "192.168.1.53" : "fd00::53"} autoCapitalize="none" spellCheck={false} /></label>
                <button type="button" aria-label={t("forward.moveUp")} disabled={index === 0} onClick={() => moveServer(index, -1)}><ArrowUp size={14} /></button>
                <button type="button" aria-label={t("forward.moveDown")} disabled={index === draft.servers.length - 1} onClick={() => moveServer(index, 1)}><ArrowDown size={14} /></button>
                <button type="button" aria-label={t("forward.removeServer")} disabled={draft.servers.length === 1} onClick={() => setDraft({ ...draft, servers: draft.servers.filter((_, serverIndex) => serverIndex !== index) })}><Trash2 size={14} /></button>
              </div>
            ))}
          </div>
          <div className="forward-options">
            <label className="forward-fallback">
              <input type="checkbox" checked={draft.forward_first} onChange={(event) => setDraft({ ...draft, forward_first: event.target.checked })} />
              <span><strong>{t("forward.fallback")}</strong><small>{t("forward.fallbackHelp")}</small></span>
            </label>
            <label className="forward-fallback forward-unsigned">
              <input type="checkbox" checked={draft.allow_unsigned} onChange={(event) => setDraft({ ...draft, allow_unsigned: event.target.checked })} />
              <span><strong>{t("forward.allowUnsigned")}</strong><small>{t("forward.allowUnsignedHelp")}</small></span>
            </label>
            <label className="forward-fallback forward-private">
              <input type="checkbox" checked={draft.allow_private_addresses} onChange={(event) => setDraft({ ...draft, allow_private_addresses: event.target.checked })} />
              <span><strong>{t("forward.allowPrivate")}</strong><small>{t("forward.allowPrivateHelp")}</small></span>
            </label>
          </div>
          <div className="wizard-actions">
            <button className="unbound-action primary" type="button" onClick={saveDraft}><Check size={15} /><span>{editing === null ? t("forward.addDraft") : t("forward.applyEdit")}</span></button>
          </div>
        </div>
      )}

      <div className="forward-zone-list">
        {zones.length === 0 && <div className="guided-empty"><Network size={22} /><div><strong>{t("forward.empty")}</strong><p>{t("forward.emptyHelp")}</p></div></div>}
        {zones.map((zone, index) => (
          <article key={zone.name}>
            <div className="forward-zone-name"><span><Network size={15} /></span><div><strong>{zone.name}</strong><small>{t("forward.targetCount", { count: zone.servers.length })}</small></div></div>
            <div className="forward-targets">{zone.servers.map((server, serverIndex) => <code key={server}><span>{serverIndex + 1}</span>{server}</code>)}</div>
            <div className="forward-policies">
              <div className={`forward-policy ${zone.forward_first ? "fallback" : "strict"}`}><ShieldAlert size={14} />{zone.forward_first ? t("forward.policyFallback") : t("forward.policyStrict")}</div>
              <div className={`forward-policy ${zone.allow_unsigned ? "unsigned" : "validated"}`}>{zone.allow_unsigned ? <ShieldAlert size={14} /> : <ShieldCheck size={14} />}{zone.allow_unsigned ? t("forward.policyUnsigned") : t("forward.policyDNSSEC")}</div>
              <div className={`forward-policy ${zone.allow_private_addresses ? "private" : "validated"}`}>{zone.allow_private_addresses ? <ShieldAlert size={14} /> : <ShieldCheck size={14} />}{zone.allow_private_addresses ? t("forward.policyPrivate") : t("forward.policyRebinding")}</div>
            </div>
            <div className="zone-actions"><button type="button" onClick={() => editZone(index)}><Pencil size={14} /> {t("common.edit")}</button><button type="button" onClick={() => removeZone(index)}><Trash2 size={14} /> {t("common.remove")}</button></div>
          </article>
        ))}
      </div>

      {dirty && !open && (
        <div className="guided-review">
          <div><strong>{t("forward.draftReady")}</strong><small>{t("forward.notActive")}</small></div>
          <button type="button" disabled={busy} onClick={createPreview}>{busy ? t("forward.checking") : t("forward.review")}</button>
        </div>
      )}

      {preview && (
        <div className="forward-preview" aria-live="polite">
          <div className="forward-check-list">
            {checks.map((check) => (
              <div key={`${check.zone}-${check.address}`} className={check.reachable ? "reachable" : "unreachable"}>
                <span>{check.reachable ? <Check size={15} /> : "!"}</span>
                <div><strong>{check.address}</strong><small>{check.zone} · {check.reachable ? t("forward.reachable") : t("forward.unreachable")}</small><em>{check.detail}</em></div>
              </div>
            ))}
          </div>
          <details><summary>{t("forward.showGenerated")}</summary><pre>{forwardingSection(preview.rendered_config) || t("forward.removalPreview")}</pre></details>
          <button type="button" disabled={busy || !preview.changed || !allReachable} onClick={activate}>{busy ? t("forward.activating") : allReachable ? t("forward.activate") : t("forward.fixTargets")}</button>
        </div>
      )}
    </section>
  );
}

function normalizeSettings(settings: UnboundSettings): UnboundSettings {
  return {
    ...settings,
    forward_zones: (settings.forward_zones ?? []).map((zone) => ({
      ...zone,
      allow_unsigned: zone.allow_unsigned ?? false,
      allow_private_addresses: zone.allow_private_addresses ?? false,
    })),
  };
}

function sameSettings(left: UnboundSettings, right: UnboundSettings) {
  return JSON.stringify(normalizeSettings(left)) === JSON.stringify(normalizeSettings(right));
}

function normalizeForwardZone(zone: UnboundForwardZone, t: (key: string, values?: Record<string, string | number>) => string): UnboundForwardZone {
  const name = normalizeZoneName(zone.name, t);
  if (zone.servers.length === 0) throw new Error(t("forward.validation.serverRequired"));
  const servers = zone.servers.map((server) => normalizeIPAddress(server, t));
  if (new Set(servers).size !== servers.length) throw new Error(t("forward.validation.duplicateServer"));
  return {
    name,
    servers,
    forward_first: zone.forward_first,
    allow_unsigned: zone.allow_unsigned,
    allow_private_addresses: zone.allow_private_addresses,
  };
}

function normalizeZoneName(value: string, t: (key: string) => string) {
  const normalized = value.trim().toLowerCase().replace(/\.*$/, "") + ".";
  if (normalized === ".") throw new Error(t("forward.validation.rootZone"));
  const labels = normalized.slice(0, -1).split(".");
  if (normalized.length > 254 || !labels.every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label))) {
    throw new Error(t("forward.validation.invalidZone"));
  }
  return normalized;
}

function normalizeIPAddress(value: string, t: (key: string, values?: Record<string, string | number>) => string) {
  const address = value.trim().toLowerCase();
  if (validIPv4(address)) {
    const parts = address.split(".").map(Number);
    if (parts[0] === 127 || parts.every((part) => part === 0) || parts[0] >= 224 || (parts[0] === 172 && parts[1] === 29 && parts[2] === 53)) {
      throw new Error(t("forward.validation.reservedAddress", { address }));
    }
    return parts.join(".");
  }
  const ipv6 = canonicalIPv6(address);
  if (!ipv6) throw new Error(t("forward.validation.invalidAddress", { address: value.trim() }));
  if (ipv6 === "::" || ipv6 === "::1" || ipv6.startsWith("fe8") || ipv6.startsWith("fe9") || ipv6.startsWith("fea") || ipv6.startsWith("feb") || ipv6.startsWith("ff")) {
    throw new Error(t("forward.validation.reservedAddress", { address: ipv6 }));
  }
  return ipv6;
}

function validIPv4(value: string) {
  const parts = value.split(".");
  return parts.length === 4 && parts.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255);
}

function canonicalIPv6(value: string) {
  if (!/^[0-9a-f:]+$/.test(value) || value.includes(":::") || value.split("::").length > 2) return "";
  const halves = value.split("::");
  const left = halves[0] ? halves[0].split(":") : [];
  const right = halves.length === 2 && halves[1] ? halves[1].split(":") : [];
  if (![...left, ...right].every((part) => /^[0-9a-f]{1,4}$/.test(part))) return "";
  if (halves.length === 1 && left.length !== 8) return "";
  if (halves.length === 2 && left.length + right.length >= 8) return "";
  const groups = halves.length === 2
    ? [...left, ...Array(8 - left.length - right.length).fill("0"), ...right]
    : left;
  const normalized = groups.map((group) => Number.parseInt(group, 16).toString(16));
  let bestStart = -1;
  let bestLength = 0;
  for (let index = 0; index < normalized.length;) {
    if (normalized[index] !== "0") {
      index++;
      continue;
    }
    let end = index;
    while (end < normalized.length && normalized[end] === "0") end++;
    if (end - index > bestLength && end - index >= 2) {
      bestStart = index;
      bestLength = end - index;
    }
    index = end;
  }
  if (bestStart < 0) return normalized.join(":");
  const before = normalized.slice(0, bestStart).join(":");
  const after = normalized.slice(bestStart + bestLength).join(":");
  return `${before}::${after}`;
}

function validateLimits(zones: UnboundForwardZone[], t: (key: string, values?: Record<string, string | number>) => string) {
  if (zones.length > maxZones) throw new Error(t("forward.tooManyZones", { count: maxZones }));
  const targets = zones.reduce((total, zone) => total + zone.servers.length, 0);
  if (targets > maxTargets) throw new Error(t("forward.tooManyTargets", { count: maxTargets }));
}

function forwardingSection(config: string) {
  const forwardStart = config.indexOf("# Conditional forwarding:");
  if (forwardStart < 0) return "";
  const serverPolicyLines = config
    .split("\n")
    .filter((line) =>
      line.includes("# Split DNS:") ||
      line.includes("# DNS rebinding:") ||
      line.trimStart().startsWith("domain-insecure:") ||
      line.trimStart().startsWith("private-domain:")
    );
  const serverPolicy = serverPolicyLines.length > 0 ? `server:\n${serverPolicyLines.join("\n")}\n\n` : "";
  return serverPolicy + config.slice(forwardStart);
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
