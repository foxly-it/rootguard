import { useRef, useState } from "react";
import { Download, Upload } from "lucide-react";
import {
  applyUnboundImport,
  fetchUnboundExport,
  previewUnboundImport,
  type UnboundBundlePreview,
  type UnboundConfigBundle,
} from "../api/client";
import { useI18n } from "../i18n";
import "../styles/unbound-expert.css";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function isConfigBundle(value: unknown): value is UnboundConfigBundle {
  if (typeof value !== "object" || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.schema_version === "number" &&
    typeof candidate.settings === "object" && candidate.settings !== null &&
    typeof candidate.custom_config === "string"
  );
}

/**
 * The complete logical resolver configuration (guided settings + expert
 * custom config together) as a single downloadable/uploadable file - for
 * backup or migrating a configuration to another RootGuard instance.
 * Distinct from the per-version history above: that's for in-place
 * rollback, this is for taking the configuration somewhere else.
 */
export default function UnboundConfigTransfer({ id, onActivated }: { id?: string; onActivated: () => Promise<void> }) {
  const { t } = useI18n();
  const [pending, setPending] = useState<UnboundConfigBundle | null>(null);
  const [preview, setPreview] = useState<UnboundBundlePreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  async function exportConfig() {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const bundle = await fetchUnboundExport();
      const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `rootguard-unbound-config-${new Date().toISOString().slice(0, 10)}.json`;
      link.click();
      URL.revokeObjectURL(url);
      setMessage(t("transfer.exported"));
    } catch (cause) {
      setError(errorMessage(cause, t("transfer.exportError")));
    } finally {
      setBusy(false);
    }
  }

  async function chooseFile() {
    fileInputRef.current?.click();
  }

  async function onFileSelected(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setBusy(true);
    setError("");
    setMessage("");
    setPreview(null);
    setPending(null);
    try {
      const text = await file.text();
      let parsed: unknown;
      try {
        parsed = JSON.parse(text);
      } catch {
        throw new Error(t("transfer.parseError"));
      }
      if (!isConfigBundle(parsed)) throw new Error(t("transfer.shapeError"));
      const result = await previewUnboundImport(parsed);
      setPending(parsed);
      setPreview(result);
      setMessage(result.changed || result.custom_changed ? t("transfer.previewReady") : t("transfer.noChanges"));
    } catch (cause) {
      setError(errorMessage(cause, t("transfer.previewRejected")));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!pending || busy || !window.confirm(t("transfer.confirmActivate"))) return;
    setBusy(true);
    setError("");
    try {
      await applyUnboundImport(pending);
      await onActivated();
      setPending(null);
      setPreview(null);
      setMessage(t("transfer.activated"));
    } catch (cause) {
      setError(errorMessage(cause, t("transfer.activateError")));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section id={id} className="glass-card" tabIndex={-1}>
      <div className="panel-heading">
        <div><p className="unbound-eyebrow">{t("transfer.eyebrow")}</p><h2>{t("transfer.title")}</h2></div>
      </div>
      <p className="muted-copy">{t("transfer.intro")}</p>
      {message && <div className="feedback success">{message}</div>}
      {error && <div className="feedback error">{error}</div>}
      <div className="transfer-actions">
        <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy} onClick={exportConfig}>
          <Download size={15} aria-hidden="true" /> {t("transfer.exportButton")}
        </button>
        <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy} onClick={chooseFile}>
          <Upload size={15} aria-hidden="true" /> {t("transfer.importButton")}
        </button>
        <input ref={fileInputRef} type="file" accept="application/json" hidden onChange={onFileSelected} aria-label={t("transfer.importButton")} />
      </div>
      {preview && (
        <div className="custom-preview">
          <div className="validation-ok">
            <strong>{preview.changed || preview.custom_changed ? t("transfer.changesFound") : t("transfer.noChanges")}</strong>
          </div>
          <details>
            <summary>{t("transfer.showGenerated")}</summary>
            <pre tabIndex={0} aria-label={t("transfer.showGenerated")}>{preview.rendered_config}</pre>
          </details>
          {(preview.changed || preview.custom_changed) && (
            <button className="rg-button rg-button-primary" type="button" disabled={busy} onClick={activate}>
              {busy ? t("transfer.activating") : t("transfer.activate")}
            </button>
          )}
        </div>
      )}
    </section>
  );
}
