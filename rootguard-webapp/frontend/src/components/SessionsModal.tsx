import { useEffect, useState } from "react";
import { Laptop, LogOut } from "lucide-react";
import ContentModal from "./ContentModal";
import { fetchAuditLog, fetchSessions, revokeSession, type AuditEvent, type SessionSummary } from "../api/client";
import { useI18n } from "../i18n";
import "../styles/sessions.css";

const WARNING_AUDIT_EVENTS = new Set<AuditEvent["event"]>(["login_failure", "login_rate_limited", "recovery_failure"]);

export default function SessionsModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t, locale } = useI18n();
  const [sessions, setSessions] = useState<SessionSummary[] | null>(null);
  const [error, setError] = useState("");
  const [revokingId, setRevokingId] = useState("");
  const [auditEvents, setAuditEvents] = useState<AuditEvent[] | null>(null);
  const [auditError, setAuditError] = useState("");

  useEffect(() => {
    if (!open) return;
    setError("");
    fetchSessions()
      .then(setSessions)
      .catch(() => setError(t("sessions.loadError")));
    setAuditError("");
    fetchAuditLog()
      .then(setAuditEvents)
      .catch(() => setAuditError(t("sessions.activityLoadError")));
  }, [open, t]);

  async function handleRevoke(entry: SessionSummary) {
    if (entry.current && !window.confirm(t("sessions.revokeCurrentConfirm"))) return;
    setError("");
    setRevokingId(entry.id);
    try {
      await revokeSession(entry.id);
      if (entry.current) {
        window.location.reload();
        return;
      }
      setSessions((current) => current?.filter((item) => item.id !== entry.id) ?? current);
    } catch {
      setError(t("sessions.revokeError"));
    } finally {
      setRevokingId("");
    }
  }

  function formatDate(value: string) {
    return new Date(value).toLocaleString(locale === "de" ? "de-DE" : "en-US", {
      dateStyle: "medium",
      timeStyle: "short",
    });
  }

  // The raw User-Agent string is long, technical noise ("Mozilla/5.0
  // (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like
  // Gecko) Chrome/..."). Reducing it to "Chrome on macOS" is what an
  // operator actually needs to recognize their own devices at a glance.
  // Order matters: Edge/Opera both contain "Chrome" in their UA, and
  // Chrome itself contains "Safari" - each check below only runs once its
  // more specific competitor has already been ruled out.
  function describeDevice(userAgent: string): string {
    if (!userAgent) return t("sessions.unknownDevice");
    let browser = "";
    if (/Edg\//.test(userAgent)) browser = "Edge";
    else if (/OPR\//.test(userAgent)) browser = "Opera";
    else if (/Firefox\//.test(userAgent)) browser = "Firefox";
    else if (/Chrome\//.test(userAgent) || /HeadlessChrome\//.test(userAgent)) browser = "Chrome";
    else if (/Safari\//.test(userAgent)) browser = "Safari";

    let os = "";
    if (/Windows/.test(userAgent)) os = "Windows";
    else if (/Mac OS X/.test(userAgent)) os = "macOS";
    else if (/Android/.test(userAgent)) os = "Android";
    else if (/iPhone|iPad/.test(userAgent)) os = "iOS";
    else if (/CrOS/.test(userAgent)) os = "Chrome OS";
    else if (/Linux/.test(userAgent)) os = "Linux";

    if (browser && os) return t("sessions.browserOnOs", { browser, os });
    return browser || os || t("sessions.unknownDevice");
  }

  return (
    <ContentModal open={open} size="medium" eyebrow={t("sessions.eyebrow")} title={t("sessions.title")} closeLabel={t("common.close")} onClose={onClose}>
      {error && <p className="feedback error" role="alert">{error}</p>}
      {sessions === null && !error && <p className="muted-copy">…</p>}
      {sessions?.length === 0 && <p className="muted-copy">{t("sessions.empty")}</p>}
      {sessions && sessions.length > 0 && (
        <ul className="session-list">
          {sessions.map((entry) => (
            <li key={entry.id} className={entry.current ? "session-entry current" : "session-entry"}>
              <Laptop aria-hidden="true" />
              <div className="session-entry-detail">
                <strong>{describeDevice(entry.user_agent)}</strong>
                {entry.current && <span className="session-current-badge">{t("sessions.current")}</span>}
                <small>{t("sessions.since", { date: formatDate(entry.created_at) })}</small>
                <small>{t("sessions.expires", { date: formatDate(entry.expires_at) })}</small>
                {entry.remote_ip && <small>{entry.remote_ip}</small>}
              </div>
              <button
                type="button"
                className="text-action"
                disabled={revokingId === entry.id}
                onClick={() => void handleRevoke(entry)}
              >
                <LogOut aria-hidden="true" size={16} />
                {revokingId === entry.id ? t("sessions.revoking") : t("sessions.revoke")}
              </button>
            </li>
          ))}
        </ul>
      )}

      <h3 className="sessions-section-heading">{t("sessions.activityTitle")}</h3>
      {auditError && <p className="feedback error" role="alert">{auditError}</p>}
      {auditEvents === null && !auditError && <p className="muted-copy">…</p>}
      {auditEvents?.length === 0 && <p className="muted-copy">{t("sessions.activityEmpty")}</p>}
      {auditEvents && auditEvents.length > 0 && (
        <ul className="audit-list">
          {auditEvents.map((event) => (
            <li key={`${event.timestamp}-${event.event}-${event.remote_ip}`} className={WARNING_AUDIT_EVENTS.has(event.event) ? "audit-entry warning" : "audit-entry"}>
              <i aria-hidden="true" />
              <div>
                <strong>{t(`sessions.activity.${event.event}`)}</strong>
                <small>{formatDate(event.timestamp)} · {event.remote_ip}</small>
              </div>
            </li>
          ))}
        </ul>
      )}
    </ContentModal>
  );
}
