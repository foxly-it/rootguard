import { useCallback, useEffect, useState } from "react";
import { Archive, ArrowRight, Download, FileInput, HardDrive, KeyRound, LoaderCircle, RotateCcw, Settings2 } from "lucide-react";
import { Link } from "react-router";
import {
  exportEncryptedBackup,
  fetchBackupStatus,
  fetchUpdateStatus,
  previewEncryptedBackup,
  restoreEncryptedBackup,
  setBackupRetention,
  type BackupRestorePreview,
  type BackupStatus,
  type UpdateStatus,
} from "../api/client";
import { useI18n } from "../i18n";
import { formatBytes } from "../utils/format";
import "../styles/stack.css";

export default function Backups() {
  const { t, formatDate } = useI18n();
  const [backups, setBackups] = useState<BackupStatus | null>(null);
  const [updates, setUpdates] = useState<UpdateStatus | null>(null);
  const [retentionDraft, setRetentionDraft] = useState<number | null>(null);
  const [savingRetention, setSavingRetention] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [passphrase, setPassphrase] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState("");
  const [restoreFile, setRestoreFile] = useState<File | null>(null);
  const [restorePassphrase, setRestorePassphrase] = useState("");
  const [restorePreview, setRestorePreview] = useState<BackupRestorePreview | null>(null);
  const [restoreConfirmation, setRestoreConfirmation] = useState(false);
  const [restoring, setRestoring] = useState(false);

  const load = useCallback(async () => {
    try {
      const [nextBackups, nextUpdates] = await Promise.all([fetchBackupStatus(), fetchUpdateStatus()]);
      setBackups(nextBackups);
      setUpdates(nextUpdates);
      setRetentionDraft((current) => current ?? nextBackups.settings.retention_per_service);
      setError("");
    } catch (cause) {
      setError(errorMessage(cause, t("backups.loadError")));
    }
  }, [t]);

  useEffect(() => {
    const initial = window.setTimeout(load, 0);
    return () => window.clearTimeout(initial);
  }, [load]);

  const updateBusy = updates?.state === "checking" || updates?.state === "updating";
  const busy = updateBusy || savingRetention || exporting || restoring;

  async function previewRestore() {
    if (!restoreFile || restorePassphrase.length < 12) return;
    const target = restorePreview?.preflight.config;
    setRestoring(true);
    setError("");
    setRestorePreview(null);
    setRestoreConfirmation(false);
    try {
      setRestorePreview(await previewEncryptedBackup(restoreFile, restorePassphrase, target));
    } catch (cause) {
      setError(errorMessage(cause, t("stack.restorePreviewError")));
    } finally {
      setRestoring(false);
    }
  }

  async function startRestore() {
    if (!restoreFile || !restorePreview?.preflight.ready || !restoreConfirmation) return;
    setRestoring(true);
    setError("");
    try {
      await restoreEncryptedBackup(restoreFile, restorePassphrase, restorePreview.preflight.config);
      window.location.assign("/");
    } catch (cause) {
      setError(errorMessage(cause, t("stack.restoreError")));
      setRestoring(false);
    }
  }

  async function saveRetention() {
    if (!backups || retentionDraft === null || retentionDraft < 2 || retentionDraft > 50) return;
    if (retentionDraft < backups.settings.retention_per_service && !window.confirm(t("stack.backupRetentionConfirm"))) return;
    setSavingRetention(true);
    setError("");
    try {
      setBackups(await setBackupRetention(retentionDraft));
    } catch (cause) {
      setError(errorMessage(cause, t("stack.backupRetentionError")));
    } finally {
      setSavingRetention(false);
    }
  }

  async function startExport() {
    if (passphrase.length < 12 || passphrase !== confirmation) return;
    setExporting(true);
    setError("");
    try {
      const exported = await exportEncryptedBackup(passphrase);
      const url = URL.createObjectURL(exported.blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = exported.filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (cause) {
      setError(errorMessage(cause, t("stack.exportError")));
    } finally {
      setPassphrase("");
      setConfirmation("");
      setExporting(false);
    }
  }

  return (
    <div className="stack-page backups-page">
      <section className="stack-hero">
        <div>
          <span className="stack-eyebrow">{t("backups.eyebrow")}</span>
          <h1>{t("backups.title")}</h1>
          <p>{t("backups.intro")}</p>
        </div>
      </section>

      {error && <div className="stack-feedback error">{error}</div>}
      {updateBusy && <div className="stack-feedback working"><LoaderCircle className="spin" size={17} /><span>{t("backups.updateBusy")}</span></div>}

      <section className="backup-import-guide" aria-labelledby="backup-import-guide-title">
        <div><span className="stack-eyebrow">{t("backups.importEyebrow")}</span><h2 id="backup-import-guide-title">{t("backups.importTitle")}</h2><p>{t("backups.importIntro")}</p></div>
        <div className="backup-import-options">
          <a href="#backup-restore"><RotateCcw size={19} /><span><strong>{t("backups.importFull")}</strong><small>{t("backups.importFullHelp")}</small></span><ArrowRight size={16} /></a>
          <Link to="/unbound/advanced#unbound-section-advanced-transfer"><Settings2 size={19} /><span><strong>{t("backups.importBundle")}</strong><small>{t("backups.importBundleHelp")}</small></span><ArrowRight size={16} /></Link>
          <Link to="/unbound/advanced#unbound-section-advanced-import-conf"><FileInput size={19} /><span><strong>{t("backups.importConf")}</strong><small>{t("backups.importConfHelp")}</small></span><ArrowRight size={16} /></Link>
        </div>
      </section>

      {backups && (
        <section className="backup-retention-panel">
          <div className="backup-retention-heading">
            <span><HardDrive size={19} /></span>
            <div>
              <span className="stack-eyebrow">{t("stack.backupEyebrow")}</span>
              <h2>{t("stack.backupTitle")}</h2>
              <p>{t("stack.backupIntro")}</p>
            </div>
          </div>
          <div className="backup-usage-grid">
            <article><strong>{formatBytes(backups.managed_bytes)}</strong><span>{t("stack.backupManaged", { count: backups.count })}</span></article>
            {backups.services.map((service) => (
              <article key={service.service}>
                <strong>{formatBytes(service.bytes)}</strong>
                <span>{t(`stack.backupService.${service.service}`, { count: service.count })}</span>
                <small>{service.newest_at ? t("stack.backupNewest", { date: formatDate(service.newest_at) }) : t("stack.backupNone")}</small>
              </article>
            ))}
          </div>
          {backups.unmanaged_bytes > 0 && <p className="backup-retention-warning">{t("stack.backupUnmanaged", { size: formatBytes(backups.unmanaged_bytes) })}</p>}
          {backups.last_error && <p className="backup-retention-warning">{t("stack.backupPruneError", { error: backups.last_error })}</p>}
          <div className="backup-retention-control">
            <label>
              <span>{t("stack.backupRetention")}</span>
              <input type="number" min={2} max={50} value={retentionDraft ?? ""} onChange={(event) => setRetentionDraft(Number.isNaN(event.target.valueAsNumber) ? null : event.target.valueAsNumber)} />
              <small>{t("stack.backupRetentionHelp")}</small>
            </label>
            <button className="rg-button rg-button-secondary" type="button" disabled={busy || retentionDraft === null || retentionDraft < 2 || retentionDraft > 50 || retentionDraft === backups.settings.retention_per_service} onClick={saveRetention}>
              {savingRetention ? <LoaderCircle className="spin" size={15} /> : <Archive size={15} />}
              {t("stack.backupRetentionSave")}
            </button>
          </div>
        </section>
      )}

      <section className="encrypted-export-panel" id="backup-export">
        <div className="encrypted-export-heading">
          <span><KeyRound size={19} /></span>
          <div><span className="stack-eyebrow">{t("stack.exportEyebrow")}</span><h2>{t("stack.exportTitle")}</h2><p>{t("stack.exportIntro")}</p></div>
        </div>
        {!window.isSecureContext && <p className="encrypted-export-warning">{t("stack.exportTransportWarning")}</p>}
        <div className="encrypted-export-fields">
          <label><span>{t("stack.exportPassphrase")}</span><input type="password" autoComplete="new-password" value={passphrase} onChange={(event) => setPassphrase(event.target.value)} minLength={12} maxLength={1024} /><small>{t("stack.exportPassphraseHelp")}</small></label>
          <label><span>{t("stack.exportPassphraseConfirm")}</span><input type="password" autoComplete="new-password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} minLength={12} maxLength={1024} /><small>{confirmation && passphrase !== confirmation ? t("stack.exportMismatch") : t("stack.exportLossWarning")}</small></label>
        </div>
        <div className="encrypted-export-actions">
          <p>{t("stack.exportFormat")}</p>
          <button className="rg-button rg-button-primary" type="button" disabled={busy || passphrase.length < 12 || passphrase !== confirmation} onClick={startExport}>
            {exporting ? <LoaderCircle className="spin" size={15} /> : <Download size={15} />} {exporting ? t("stack.exporting") : t("stack.exportButton")}
          </button>
        </div>
      </section>

      <section className="encrypted-export-panel restore-panel" id="backup-restore">
        <div className="encrypted-export-heading">
          <span><RotateCcw size={19} /></span>
          <div><span className="stack-eyebrow">{t("stack.restoreEyebrow")}</span><h2>{t("stack.restoreTitle")}</h2><p>{t("stack.restoreIntro")}</p></div>
        </div>
        {!window.isSecureContext && <p className="encrypted-export-warning">{t("stack.exportTransportWarning")}</p>}
        <div className="encrypted-export-fields">
          <label><span>{t("stack.restoreArchive")}</span><input type="file" accept=".age,application/vnd.rootguard.backup+age" onChange={(event) => { setRestoreFile(event.target.files?.[0] ?? null); setRestorePreview(null); }} /></label>
          <label><span>{t("stack.exportPassphrase")}</span><input type="password" autoComplete="current-password" minLength={12} maxLength={1024} value={restorePassphrase} onChange={(event) => { setRestorePassphrase(event.target.value); setRestorePreview(null); }} /></label>
        </div>
        <div className="encrypted-export-actions">
          <p>{t("stack.restorePreviewHelp")}</p>
          <button className="rg-button rg-button-secondary" type="button" disabled={busy || !restoreFile || restorePassphrase.length < 12} onClick={previewRestore}>{restoring ? <LoaderCircle className="spin" size={15} /> : <Archive size={15} />} {t("stack.restorePreview")}</button>
        </div>
        {restorePreview && <div className="restore-preview">
          <p>{t("stack.restoreSummary", { date: formatDate(restorePreview.created_at), count: restorePreview.file_count, size: formatBytes(restorePreview.expanded_bytes) })}</p>
          <div className="encrypted-export-fields">
            <label><span>{t("stack.restoreAddress")}</span><input value={restorePreview.preflight.config.dns_bind_address} onChange={(event) => setRestorePreview({...restorePreview, preflight: {...restorePreview.preflight, ready: false, config: {...restorePreview.preflight.config, dns_bind_address: event.target.value}}})} /></label>
            <label><span>{t("stack.restorePort")}</span><input type="number" min={1} max={65535} value={restorePreview.preflight.config.dns_port} onChange={(event) => setRestorePreview({...restorePreview, preflight: {...restorePreview.preflight, ready: false, config: {...restorePreview.preflight.config, dns_port: event.target.valueAsNumber}}})} /></label>
          </div>
          {!restorePreview.preflight.ready && <button className="rg-button rg-button-secondary" type="button" disabled={busy} onClick={previewRestore}>{t("stack.restoreRecheck")}</button>}
          {!restorePreview.preflight.ready && <p className="encrypted-export-warning">{t("stack.restoreBlocked")}</p>}
          <label className="restore-confirm"><input type="checkbox" checked={restoreConfirmation} onChange={(event) => setRestoreConfirmation(event.target.checked)} disabled={!restorePreview.preflight.ready} /><span>{t("stack.restoreConfirm")}</span></label>
          <button className="rg-button rg-button-primary" type="button" disabled={busy || !restorePreview.preflight.ready || !restoreConfirmation} onClick={startRestore}><RotateCcw size={15} /> {restoring ? t("stack.restoring") : t("stack.restoreButton")}</button>
        </div>}
      </section>
    </div>
  );
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
