import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent, type UIEvent } from "react";
import { createPortal } from "react-dom";
import { Maximize2, Minimize2 } from "lucide-react";
import {
  fetchUnboundCustom,
  fetchUnboundDirectives,
  previewUnboundCustom,
  updateUnboundCustom,
  type UnboundCustomPreview,
  type UnboundDirectiveReference,
} from "../api/client";
import "../styles/unbound-expert.css";
import { useI18n } from "../i18n";
import ContentModal from "./ContentModal";

const templates = [
  { label: "expert.template.local", content: 'server:\n    local-zone: "home.arpa." static\n    local-data: "router.home.arpa. 300 IN A 192.168.1.1"\n' },
  { label: "expert.template.private", content: 'server:\n    private-domain: "home.arpa"\n' },
  { label: "expert.template.forward", content: 'forward-zone:\n    name: "corp.example."\n    forward-addr: 192.168.1.53\n' },
  { label: "expert.template.hardening", content: "server:\n    hide-identity: yes\n    hide-version: yes\n    harden-glue: yes\n    harden-dnssec-stripped: yes\n    aggressive-nsec: yes\n" },
];

export default function UnboundExpertEditor({ version, baseConfig, onActivated }: { version?: string; baseConfig?: string; onActivated: () => Promise<void> }) {
  const { locale, t } = useI18n();
  const [open, setOpen] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [active, setActive] = useState("");
  const [draft, setDraft] = useState("");
  const [maxBytes, setMaxBytes] = useState(65536);
  const [directives, setDirectives] = useState<UnboundDirectiveReference[]>([]);
  const [preview, setPreview] = useState<UnboundCustomPreview | null>(null);
  const [cursor, setCursor] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [catalogOpen, setCatalogOpen] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const highlightRef = useRef<HTMLPreElement>(null);

  const load = useCallback(async () => {
    const [document, reference] = await Promise.all([fetchUnboundCustom(), fetchUnboundDirectives()]);
    setActive(document.content);
    setDraft(document.content);
    setMaxBytes(document.max_bytes);
    setDirectives(reference);
    setPreview(null);
  }, []);

  useEffect(() => {
    load().catch((err: unknown) => setError(errorMessage(err, t("expert.loadError"))));
  }, [load, t, version]);

  useEffect(() => {
    if (!open) setFullscreen(false);
  }, [open]);

  useEffect(() => {
    if (!fullscreen) return;
    function onKeyDown(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") setFullscreen(false);
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [fullscreen]);

  const prefix = useMemo(() => directivePrefix(draft, cursor), [draft, cursor]);
  const suggestions = useMemo(() => {
    if (!prefix) return [];
    return directives.filter((item) => item.name.toLowerCase().startsWith(prefix.toLowerCase())).slice(0, 6);
  }, [directives, prefix]);
  const selectedDirective = useMemo(() => directiveAt(draft, cursor, directives), [draft, cursor, directives]);
  const bytes = new TextEncoder().encode(draft).length;

  async function createPreview() {
    if (busy) return;
    setBusy(true);
    clearFeedback();
    try {
      const result = await previewUnboundCustom(draft);
      setDraft(result.content);
      setPreview(result);
      setMessage(t("expert.previewReady"));
    } catch (err) {
      setPreview(null);
      setError(errorMessage(err, t("expert.invalid")));
    } finally {
      setBusy(false);
    }
  }

  async function activate() {
    if (!preview?.changed || busy || !window.confirm(t("expert.confirmActivate"))) return;
    setBusy(true);
    clearFeedback();
    try {
      const document = await updateUnboundCustom(preview.content);
      setActive(document.content);
      setDraft(document.content);
      setPreview(null);
      await onActivated();
      setMessage(t("expert.activated"));
    } catch (err) {
      setError(errorMessage(err, t("expert.activateError")));
    } finally {
      setBusy(false);
    }
  }

  function insertSuggestion(reference: UnboundDirectiveReference) {
    const start = Math.max(0, cursor - prefix.length);
    const indentation = draft.slice(draft.lastIndexOf("\n", start - 1) + 1, start).match(/^\s*/)?.[0] ?? "";
    const example = reference.example.includes("\n") ? reference.example : indentation + reference.example.trimStart();
    const next = draft.slice(0, start - indentation.length) + example + draft.slice(cursor);
    const nextCursor = start - indentation.length + example.length;
    setDraft(next);
    setPreview(null);
    requestAnimationFrame(() => {
      textareaRef.current?.focus();
      textareaRef.current?.setSelectionRange(nextCursor, nextCursor);
      setCursor(nextCursor);
    });
  }

  function insertTemplate(content: string) {
    const separator = draft && !draft.endsWith("\n\n") ? (draft.endsWith("\n") ? "\n" : "\n\n") : "";
    setDraft(draft + separator + content);
    setPreview(null);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Tab") return;
    event.preventDefault();
    const element = event.currentTarget;
    const next = draft.slice(0, element.selectionStart) + "    " + draft.slice(element.selectionEnd);
    const nextCursor = element.selectionStart + 4;
    setDraft(next);
    setPreview(null);
    requestAnimationFrame(() => element.setSelectionRange(nextCursor, nextCursor));
  }

  function syncScroll(event: UIEvent<HTMLTextAreaElement>) {
    if (!highlightRef.current) return;
    highlightRef.current.scrollTop = event.currentTarget.scrollTop;
    highlightRef.current.scrollLeft = event.currentTarget.scrollLeft;
  }

  function clearFeedback() {
    setError("");
    setMessage("");
  }

  // Fullscreen escapes via a portal: .unbound-page carries a permanent
  // `animation: ... both` that freezes a non-none transform on it after
  // the entrance animation ends, which makes it a containing block for
  // position:fixed descendants - without the portal the "fullscreen"
  // panel would size itself against that page div instead of the
  // viewport (see ContentModal/SearchModal for the same escape pattern).
  const panel = (
    <section className={`glass-card expert-panel${fullscreen ? " expert-panel-fullscreen" : ""}`}>
      <div className="panel-heading expert-heading">
        <div><p className="unbound-eyebrow">{t("expert.eyebrow")}</p><h2>{t("expert.title")}</h2></div>
        <div className="expert-heading-actions">
          {open && (
            <button
              className="rg-button rg-button-secondary secondary-action"
              type="button"
              onClick={() => setFullscreen(!fullscreen)}
              aria-pressed={fullscreen}
            >
              {fullscreen ? <Minimize2 size={15} aria-hidden="true" /> : <Maximize2 size={15} aria-hidden="true" />}
              {fullscreen ? t("expert.exitFullscreen") : t("expert.fullscreen")}
            </button>
          )}
          <button className="rg-button rg-button-secondary secondary-action" type="button" onClick={() => setOpen(!open)}>{open ? t("expert.close") : t("expert.open")}</button>
        </div>
      </div>
      <p className="muted-copy">{t("expert.intro")}</p>
      {!open && <div className="expert-summary"><span>{active ? t("expert.bytes", { count: new TextEncoder().encode(active).length }) : t("expert.none")}</span><span>{t("expert.safety")}</span></div>}
      {open && <>
        <div className="expert-warning"><strong>{t("expert.title")}</strong><span>{t("expert.warning")}</span></div>
        {message && <div className="feedback success">{message}</div>}
        {error && <div className="feedback error">{error}</div>}
        {baseConfig && (
          <details className="live-config-disclosure expert-base-config">
            <summary>{t("expert.baseConfigSummary")}</summary>
            <pre>{baseConfig}</pre>
          </details>
        )}
        <div className="template-row">{templates.map((template) => <button type="button" key={template.label} onClick={() => insertTemplate(template.content)}>{t(template.label)}</button>)}</div>
        <div className="expert-grid">
          <div className="expert-editor-column">
            <div className="editor-toolbar"><span>90-rootguard-custom.conf</span><span className={bytes > maxBytes ? "limit-exceeded" : ""}>{bytes.toLocaleString(locale)} / {maxBytes.toLocaleString(locale)} Bytes</span></div>
            <div className="code-editor">
              <pre ref={highlightRef} aria-hidden="true">{highlightConfig(draft)}{"\n"}</pre>
              <textarea ref={textareaRef} aria-label={t("expert.title")} spellCheck={false} value={draft} onChange={(event) => { setDraft(event.target.value); setCursor(event.target.selectionStart); setPreview(null); }} onSelect={(event) => setCursor(event.currentTarget.selectionStart)} onKeyDown={handleKeyDown} onScroll={syncScroll} />
            </div>
            {suggestions.length > 0 && <div className="completion-list"><span>{t("expert.complete")}</span>{suggestions.map((item) => <button type="button" key={`${item.section}-${item.name}`} onClick={() => insertSuggestion(item)}><code>{item.name}</code><small>{item.section}</small></button>)}</div>}
            <div className="editor-actions">
              <button className="rg-button rg-button-primary" type="button" disabled={busy || bytes > maxBytes} onClick={createPreview}>{busy ? t("expert.checking") : t("expert.check")}</button>
              <button className="rg-button rg-button-secondary secondary-action" type="button" disabled={busy || draft === active} onClick={() => { setDraft(active); setPreview(null); }}>{t("expert.loadActive")}</button>
            </div>
          </div>
          <aside className="directive-help">
            <p className="unbound-eyebrow">{t("expert.context")}</p>
            {selectedDirective ? <><div className="directive-title"><code>{selectedDirective.name}</code><span className={`risk-${selectedDirective.risk}`}>{t(`expert.risk.${selectedDirective.risk}`)}</span></div><p>{selectedDirective.description}</p><pre>{selectedDirective.example}</pre></> : <p className="muted-copy">{t("expert.contextHelp")}</p>}
            <button className="rg-button rg-button-secondary secondary-action catalog-action" type="button" onClick={() => setCatalogOpen(true)}>{t("expert.catalog")}</button>
          </aside>
        </div>
        <ContentModal open={catalogOpen} eyebrow={t("expert.context")} title={t("expert.catalog")} closeLabel={t("common.close")} onClose={() => setCatalogOpen(false)}>
          <div className="directive-catalog modal-catalog">{directives.map((item) => <button type="button" key={`${item.section}-${item.name}`} onClick={() => { insertSuggestion(item); setCatalogOpen(false); }}><code>{item.name}</code><small>{item.description}</small><em>{item.section}</em></button>)}</div>
        </ContentModal>
        {preview && <div className="custom-preview">
          <div className="validation-ok"><strong>{t("expert.valid")}</strong><span>{preview.validation}</span></div>
          <div className="custom-advice">{preview.advice.map((item) => <article className={`advice-item ${item.severity}`} key={item.id}><strong>{item.title}{item.line ? ` · ${t("expert.line", { line: item.line })}` : ""}</strong><p>{item.description}</p><small>{item.suggestion}</small></article>)}</div>
          {!preview.changed ? <p>{t("expert.same")}</p> : <button className="rg-button rg-button-primary" type="button" disabled={busy} onClick={activate}>{t("expert.activate")}</button>}
        </div>}
      </>}
    </section>
  );

  return fullscreen ? createPortal(panel, document.body) : panel;
}

function directivePrefix(content: string, cursor: number) {
  return content.slice(0, cursor).match(/([a-z][a-z0-9-]*)$/i)?.[1] ?? "";
}

function directiveAt(content: string, cursor: number, directives: UnboundDirectiveReference[]) {
  const start = content.lastIndexOf("\n", Math.max(0, cursor - 1)) + 1;
  const end = content.indexOf("\n", cursor);
  const line = content.slice(start, end < 0 ? content.length : end).trim();
  const key = line.split(":", 1)[0].toLowerCase();
  return directives.find((item) => item.name.replace(/:$/, "").toLowerCase() === key);
}

function highlightConfig(content: string) {
  return content.split("\n").map((line, index) => {
    const commentAt = line.indexOf("#");
    const code = commentAt >= 0 ? line.slice(0, commentAt) : line;
    const comment = commentAt >= 0 ? line.slice(commentAt) : "";
    const separator = code.indexOf(":");
    if (separator < 0) return <span key={index}>{code}{comment && <span className="syntax-comment">{comment}</span>}{"\n"}</span>;
    const key = code.slice(0, separator);
    const value = code.slice(separator + 1);
    const valueClass = /\b(yes|no|allow|deny|refuse|static|transparent)\b/i.test(value) ? "syntax-keyword" : /\d/.test(value) ? "syntax-number" : "syntax-value";
    return <span key={index}>{key.slice(0, key.length - key.trimStart().length)}<span className="syntax-directive">{key.trim()}</span>:<span className={valueClass}>{value}</span>{comment && <span className="syntax-comment">{comment}</span>}{"\n"}</span>;
  });
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
