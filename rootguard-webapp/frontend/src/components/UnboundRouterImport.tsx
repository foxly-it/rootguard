import { useMemo, useState } from "react";
import { Check, Loader2, Plus, Router, RotateCcw, Trash2, Wifi, WifiOff } from "lucide-react";
import {
  discoverFritzBoxHosts,
  type DiscoveredHost,
  type UnboundLocalHost,
  type UnboundLocalZone,
} from "../api/client";
import { errorMessage, useUnboundDraftWorkflow } from "../hooks/useUnboundDraftWorkflow";
import GuidedFlowSteps from "./GuidedFlowSteps";
import { useI18n } from "../i18n";
import "../styles/unbound-router-import.css";

const defaultZoneName = "home.lab.";
const maxHostsPerImport = 256;
const hostLabelPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

interface DraftHost {
  key: string;
  source: DiscoveredHost;
  hostname: string;
  selected: boolean;
}

export default function UnboundRouterImport({
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
  const [zoneName, setZoneName] = useState(defaultZoneName);

  const [address, setAddress] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [discovering, setDiscovering] = useState(false);
  const [discovered, setDiscovered] = useState<DraftHost[]>([]);
  const [truncated, setTruncated] = useState(false);

  const workflow = useUnboundDraftWorkflow({
    version,
    onActivated,
    loadErrorMessage: t("routerImport.loadError"),
    concurrentMessage: t("routerImport.concurrent"),
    previewRejectedMessage: t("routerImport.previewRejected"),
    confirmActivateMessage: t("routerImport.confirmActivate"),
    activateErrorMessage: t("routerImport.activateError"),
    sameSettings: (a, b) => JSON.stringify(a.local_zones ?? []) === JSON.stringify(b.local_zones ?? []),
    onLoad: (settings) => setZones(structuredClone(settings.local_zones ?? [])),
  });

  const dirty = useMemo(
    () => workflow.source !== null && JSON.stringify(zones) !== JSON.stringify(workflow.source.local_zones ?? []),
    [zones, workflow.source],
  );

  const selectedCount = useMemo(() => discovered.filter((host) => host.selected).length, [discovered]);

  async function discover() {
    if (discovering || !address.trim()) return;
    setDiscovering(true);
    workflow.setMessage("");
    workflow.setError("");
    try {
      const result = await discoverFritzBoxHosts(address.trim(), username, password);
      setTruncated(result.truncated);
      setDiscovered(result.hosts.map((host, index) => ({
        key: `${host.mac || host.ipv4}-${index}`,
        source: host,
        hostname: suggestHostname(host.hostname, index),
        selected: false,
      })));
      workflow.setMessage(result.hosts.length > 0 ? t("routerImport.discovered", { count: result.hosts.length }) : t("routerImport.noHosts"));
    } catch (cause) {
      setDiscovered([]);
      workflow.setError(errorMessage(cause, t("routerImport.discoverError")));
    } finally {
      setDiscovering(false);
    }
  }

  function toggleSelected(key: string) {
    setDiscovered((current) => current.map((host) => (host.key === key ? { ...host, selected: !host.selected } : host)));
  }

  function renameDraft(key: string, value: string) {
    setDiscovered((current) => current.map((host) => (host.key === key ? { ...host, hostname: value } : host)));
  }

  function addSelectedToZone() {
    workflow.setError("");
    workflow.setMessage("");
    const selected = discovered.filter((host) => host.selected);
    if (selected.length === 0) {
      workflow.setError(t("routerImport.selectAtLeastOne"));
      return;
    }
    try {
      const canonicalZone = normalizeZoneName(zoneName, t);
      const existingIndex = zones.findIndex((zone) => zone.name === canonicalZone);
      const totalAfter = (existingIndex >= 0 ? zones[existingIndex].hosts.length : 0)
        + zones.filter((_, index) => index !== existingIndex).reduce((sum, zone) => sum + zone.hosts.length, 0)
        + selected.length;
      if (totalAfter > maxHostsPerImport) throw new Error(t("routerImport.tooManyHosts", { count: maxHostsPerImport }));

      const existingHostnames = new Set(existingIndex >= 0 ? zones[existingIndex].hosts.map((host) => host.hostname) : []);
      const seen = new Set<string>();
      const newHosts: UnboundLocalHost[] = [];
      for (const draft of selected) {
        const hostname = normalizeHostLabel(draft.hostname, t);
        if (existingHostnames.has(hostname) || seen.has(hostname)) {
          throw new Error(t("routerImport.duplicateHostname", { name: hostname }));
        }
        seen.add(hostname);
        newHosts.push({ hostname, ipv4: draft.source.ipv4, ptr: false });
      }

      setZones(existingIndex >= 0
        ? zones.map((zone, index) => (index === existingIndex ? { ...zone, hosts: [...zone.hosts, ...newHosts] } : zone))
        : [...zones, { name: canonicalZone, hosts: newHosts }]);
      setDiscovered((current) => current.filter((host) => !host.selected));
      workflow.resetPreview();
      workflow.setMessage(t("routerImport.added", { count: newHosts.length }));
    } catch (cause) {
      workflow.setError(errorMessage(cause, t("routerImport.invalidHostname")));
    }
  }

  function removeHost(zoneNameValue: string, hostname: string) {
    if (!window.confirm(t("routerImport.confirmRemove", { name: hostname }))) return;
    setZones((current) => current
      .map((zone) => (zone.name === zoneNameValue ? { ...zone, hosts: zone.hosts.filter((host) => host.hostname !== hostname) } : zone))
      .filter((zone) => zone.hosts.length > 0));
    workflow.resetPreview();
  }

  async function createPreview() {
    const result = await workflow.createPreview((active) => ({ ...active, local_zones: zones }));
    if (result) workflow.setMessage(t("routerImport.previewAccepted"));
  }

  async function activate() {
    if (await workflow.activate()) workflow.setMessage(t("routerImport.activated"));
  }

  return (
    <section id={id} className="glass-card router-import-panel" tabIndex={-1}>
      <div className="panel-heading">
        <div>
          <p className="unbound-eyebrow">{t("routerImport.eyebrow")}</p>
          <h2>{t("routerImport.title")}</h2>
          <p className="muted-copy">{t("routerImport.intro")}</p>
        </div>
      </div>

      <GuidedFlowSteps steps={[
        { label: t("routerImport.step1"), active: !workflow.preview },
        { label: t("routerImport.step2"), active: Boolean(workflow.preview) },
        { label: t("routerImport.step3"), active: false },
      ]} />

      {workflow.message && <div className="feedback success">{workflow.message}</div>}
      {workflow.error && <div className="feedback error" role="alert">{workflow.error}</div>}

      <div className="router-import-connect">
        <label>
          <span>{t("routerImport.address")}</span>
          <input
            value={address}
            onChange={(event) => setAddress(event.target.value)}
            placeholder="192.168.178.1"
            autoCapitalize="none"
            spellCheck={false}
            onKeyDown={(event) => { if (event.key === "Enter") { event.preventDefault(); discover(); } }}
          />
        </label>
        <button
          className="rg-button rg-button-secondary unbound-action"
          type="button"
          disabled={discovering || !address.trim()}
          onClick={discover}
        >
          {discovering ? <Loader2 size={15} className="spin" /> : <Router size={15} />}
          <span>{discovering ? t("routerImport.discovering") : t("routerImport.discover")}</span>
        </button>
        <details className="router-import-credentials">
          <summary>{t("routerImport.showCredentials")}</summary>
          <p className="muted-copy">{t("routerImport.credentialsHelp")}</p>
          <div>
            <label>
              <span>{t("routerImport.username")}</span>
              <input value={username} onChange={(event) => setUsername(event.target.value)} autoCapitalize="none" spellCheck={false} />
            </label>
            <label>
              <span>{t("routerImport.password")}</span>
              <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="off" />
            </label>
          </div>
        </details>
      </div>

      {truncated && <div className="router-import-truncated">{t("routerImport.truncated", { count: maxHostsPerImport })}</div>}

      {discovered.length > 0 && (
        <div className="router-import-results">
          <div className="router-import-results-heading">
            <strong>{t("routerImport.resultsTitle")}</strong>
            <small>{t("routerImport.resultsHelp")}</small>
          </div>
          <ul className="router-import-list">
            {discovered.map((host) => (
              <li key={host.key} className={host.selected ? "selected" : ""}>
                <label className="router-import-checkbox">
                  <span className="sr-only">{t("routerImport.selectHost", { name: host.source.hostname || host.source.ipv4 })}</span>
                  <input type="checkbox" checked={host.selected} onChange={() => toggleSelected(host.key)} />
                </label>
                <span className={`router-import-status ${host.source.active ? "active" : "inactive"}`}>
                  {host.source.active ? <Wifi size={14} /> : <WifiOff size={14} />}
                </span>
                <label className="router-import-hostname">
                  <span className="sr-only">{t("routerImport.hostname")}</span>
                  <input
                    value={host.hostname}
                    onChange={(event) => renameDraft(host.key, event.target.value)}
                    autoCapitalize="none"
                    spellCheck={false}
                  />
                </label>
                <code>{host.source.ipv4}</code>
                <small>{host.source.mac}</small>
              </li>
            ))}
          </ul>
          <div className="router-import-target">
            <label>
              <span>{t("routerImport.zoneName")}</span>
              <input value={zoneName} onChange={(event) => setZoneName(event.target.value)} placeholder={defaultZoneName} autoCapitalize="none" spellCheck={false} />
            </label>
            <button className="rg-button rg-button-primary unbound-action primary" type="button" disabled={selectedCount === 0} onClick={addSelectedToZone}>
              <Plus size={15} />
              <span>{t("routerImport.addSelected", { count: selectedCount })}</span>
            </button>
          </div>
        </div>
      )}

      <div className="router-import-zones-heading">
        <div><strong>{t("routerImport.importedTitle")}</strong><small>{t("routerImport.importedHelp")}</small></div>
        <button className="icon-action" type="button" disabled={workflow.busy} onClick={() => workflow.load().catch((cause: unknown) => workflow.setError(errorMessage(cause, t("routerImport.loadError"))))} aria-label={t("common.refresh")} title={t("common.refresh")}>
          <RotateCcw size={17} />
        </button>
      </div>

      {zones.length === 0 && (
        <div className="guided-empty">
          <Router size={22} />
          <div><strong>{t("routerImport.empty")}</strong><p>{t("routerImport.emptyHelp")}</p></div>
        </div>
      )}

      {zones.map((zone) => (
        <div className="router-import-zone" key={zone.name}>
          <p className="router-import-zone-name">{zone.name}</p>
          <ul className="router-import-list router-import-list-static">
            {zone.hosts.map((host) => (
              <li key={host.hostname}>
                <span className="router-import-status active"><Wifi size={14} /></span>
                <strong>{host.hostname}</strong>
                <code>{host.ipv4 || host.ipv6}</code>
                <button type="button" aria-label={t("routerImport.remove", { name: host.hostname })} onClick={() => removeHost(zone.name, host.hostname)}>
                  <Trash2 size={14} />
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}

      {dirty && (
        <div className="guided-review">
          <div><strong>{t("routerImport.draftReady")}</strong><small>{t("routerImport.notActive")}</small></div>
          <button className="rg-button rg-button-primary unbound-action primary" type="button" disabled={workflow.busy} onClick={createPreview}>
            <Check size={15} /><span>{workflow.busy ? t("routerImport.validating") : t("routerImport.review")}</span>
          </button>
        </div>
      )}

      {workflow.preview && (
        <div className="router-import-preview" aria-live="polite">
          <div><Check size={16} /><strong>{t("routerImport.valid")}</strong></div>
          <details open>
            <summary>{t("routerImport.showGenerated")}</summary>
            <pre tabIndex={0} aria-label={t("routerImport.showGenerated")}>{localZoneSection(workflow.preview.rendered_config)}</pre>
          </details>
          <button className="rg-button rg-button-primary" type="button" disabled={workflow.busy || !workflow.preview.changed} onClick={activate}>
            {workflow.busy ? t("routerImport.activating") : t("routerImport.activate")}
          </button>
        </div>
      )}
    </section>
  );
}

function suggestHostname(raw: string, index: number): string {
  const sanitized = raw
    .normalize("NFKD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 63);
  return sanitized || `host-${index + 1}`;
}

function normalizeHostLabel(value: string, t: (key: string) => string): string {
  const normalized = value.trim().toLowerCase();
  if (!hostLabelPattern.test(normalized)) throw new Error(t("routerImport.validation.hostname"));
  return normalized;
}

function normalizeZoneName(value: string, t: (key: string) => string): string {
  const normalized = value.trim().toLowerCase().replace(/\.*$/, "") + ".";
  if (normalized === ".") throw new Error(t("routerImport.validation.zoneRoot"));
  const labels = normalized.slice(0, -1).split(".");
  if (normalized.length > 254 || !labels.every((label) => hostLabelPattern.test(label))) {
    throw new Error(t("routerImport.validation.zoneName"));
  }
  return normalized;
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
