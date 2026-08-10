import { useRef, useState } from "react";
import { FileUp } from "lucide-react";
import {
  applyUnboundImport,
  classifyUnboundImportConf,
  fetchUnboundExport,
  previewUnboundImport,
  type UnboundBundlePreview,
  type UnboundConfigBundle,
  type UnboundImportResult,
} from "../api/client";
import { useI18n } from "../i18n";
import GuidedFlowSteps from "./GuidedFlowSteps";
import "../styles/unbound-expert.css";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

const dispositionClass: Record<string, string> = {
  guided: "success",
  fixed_base: "recommendation",
  expert: "recommendation",
  blocked: "warning",
};

/**
 * Classifies a pasted/uploaded hand-written unbound.conf against RootGuard's
 * ownership model (fixed base / guided / expert / blocked, see
 * docs/unbound-configuration-roadmap.md) and offers the result for adoption
 * through the same bundle preview/activate lifecycle as UnboundConfigTransfer.
 * Directives with no guided mapping yet (forward-zone, local-zone, ...) are
 * offered whole for expert adoption rather than silently dropped - the same
 * outcome as pasting them into the expert editor by hand today.
 */
export default function UnboundConfImport({ id, onActivated }: { id?: string; onActivated: () => Promise<void> }) {
  const { t } = useI18n();
  const [content, setContent] = useState("");
  const [result, setResult] = useState<UnboundImportResult | null>(null);
  const [bundle, setBundle] = useState<UnboundConfigBundle | null>(null);
  const [preview, setPreview] = useState<UnboundBundlePreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  function reset() {
    setResult(null);
    setBundle(null);
    setPreview(null);
    setMessage("");
    setError("");
  }

  async function chooseFile() {
    fileInputRef.current?.click();
  }

  async function onFileSelected(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setContent(await file.text());
    reset();
  }

  async function classify() {
    setBusy(true);
    reset();
    try {
      const classified = await classifyUnboundImportConf(content);
      const active = await fetchUnboundExport();
      setResult(classified);
      setBundle({ ...active, settings: classified.settings, custom_config: classified.custom_adopted });
    } catch (cause) {
      setError(errorMessage(cause, t("importConf.classifyError")));
    } finally {
      setBusy(false);
    }
  }

  async function createPreview() {
    if (!bundle) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const bundlePreview = await previewUnboundImport(bundle);
      setPreview(bundlePreview);
      setMessage(bundlePreview.changed || bundlePreview.custom_changed ? t("importConf.previewReady") : t("importConf.noChanges"));
    } catch (cause) {
      setPreview(null);
      setError(errorMessage(cause, t("importConf.previewRejected")));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!bundle || busy || !window.confirm(t("importConf.confirmActivate"))) return;
    setBusy(true);
    setError("");
    try {
      await applyUnboundImport(bundle);
      await onActivated();
      setContent("");
      reset();
      setMessage(t("importConf.activated"));
    } catch (cause) {
      setError(errorMessage(cause, t("importConf.activateError")));
    } finally {
      setBusy(false);
    }
  }

  const counts = result
    ? { guided: 0, fixed_base: 0, expert: 0, blocked: 0, ...countByDisposition(result.findings) }
    : null;
  const adoptable = result ? result.findings.some((f) => f.disposition === "guided" || f.disposition === "expert") : false;

  return (
    <section id={id} className="glass-card import-conf-panel" tabIndex={-1}>
      <div className="panel-heading">
        <div><p className="unbound-eyebrow">{t("importConf.eyebrow")}</p><h2>{t("importConf.title")}</h2></div>
      </div>
      <p className="muted-copy">{t("importConf.intro")}</p>
      <GuidedFlowSteps
        steps={[
          { label: t("importConf.step1"), active: !result },
          { label: t("importConf.step2"), active: Boolean(result) && !preview },
          { label: t("importConf.step3"), active: Boolean(preview) },
        ]}
      />
      {message && <div className="feedback success">{message}</div>}
      {error && <div className="feedback error">{error}</div>}
      <textarea
        className="import-conf-textarea"
        aria-label={t("importConf.pasteLabel")}
        placeholder={t("importConf.pasteLabel")}
        value={content}
        onChange={(event) => { setContent(event.target.value); reset(); }}
        spellCheck={false}
        rows={10}
      />
      <div className="transfer-actions">
        <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy} onClick={chooseFile}>
          <FileUp size={15} aria-hidden="true" /> {t("importConf.chooseFile")}
        </button>
        <input ref={fileInputRef} type="file" accept=".conf,text/plain" hidden onChange={onFileSelected} aria-label={t("importConf.chooseFile")} />
        <button className="rg-button rg-button-primary" type="button" disabled={busy || !content.trim()} onClick={classify}>
          {busy && !result ? t("importConf.classifying") : t("importConf.classify")}
        </button>
      </div>
      {result && counts && (
        <div className="custom-preview">
          <div className="custom-advice">
            <article className={`advice-item ${dispositionClass.guided}`}><strong>{t("importConf.guidedCount", { count: counts.guided })}</strong></article>
            <article className={`advice-item ${dispositionClass.expert}`}><strong>{t("importConf.expertCount", { count: counts.expert })}</strong></article>
            <article className={`advice-item ${dispositionClass.fixed_base}`}><strong>{t("importConf.fixedBaseCount", { count: counts.fixed_base })}</strong></article>
            <article className={`advice-item ${dispositionClass.blocked}`}><strong>{t("importConf.blockedCount", { count: counts.blocked })}</strong></article>
          </div>
          <details>
            <summary>{t("importConf.showFindings")}</summary>
            <ul className="import-conf-findings">
              {result.findings.map((finding, index) => (
                <li key={`${finding.section}-${finding.line}-${index}`} className={`finding-${finding.disposition}`}>
                  <code>{finding.section !== "server" && finding.section !== finding.directive ? `${finding.section}.` : ""}{finding.directive}{finding.value ? `: ${finding.value}` : ""}</code>
                  <small>{t(`importConf.disposition.${finding.disposition}`)} · {finding.detail}</small>
                </li>
              ))}
            </ul>
          </details>
          {adoptable && !preview && (
            <button className="rg-button rg-button-primary" type="button" disabled={busy} onClick={createPreview}>
              {busy ? t("importConf.validating") : t("importConf.previewAdoption")}
            </button>
          )}
        </div>
      )}
      {preview && (
        <div className="custom-preview">
          <details open>
            <summary>{t("importConf.showGenerated")}</summary>
            <pre tabIndex={0} aria-label={t("importConf.showGenerated")}>{preview.rendered_config}</pre>
          </details>
          {(preview.changed || preview.custom_changed) && (
            <button className="rg-button rg-button-primary" type="button" disabled={busy} onClick={activate}>
              {busy ? t("importConf.activating") : t("importConf.activate")}
            </button>
          )}
        </div>
      )}
    </section>
  );
}

function countByDisposition(findings: UnboundImportResult["findings"]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const finding of findings) counts[finding.disposition] = (counts[finding.disposition] ?? 0) + 1;
  return counts;
}
