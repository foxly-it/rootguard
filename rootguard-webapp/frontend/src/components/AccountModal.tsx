import { useEffect, useRef, useState, type FormEvent, type RefObject } from "react";
import { Eye, EyeOff } from "lucide-react";
import ContentModal from "./ContentModal";
import { updateAccount } from "../api/client";
import { useAuth } from "../auth";
import { useI18n } from "../i18n";
import "../styles/account.css";

export default function AccountModal({ open, onClose, returnFocusTo }: { open: boolean; onClose: () => void; returnFocusTo?: RefObject<Element | null> }) {
  const { t } = useI18n();
  const { username, updateUsername } = useAuth();
  const [newUsername, setNewUsername] = useState(username);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPasswords, setShowPasswords] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  // Only reset the form when the modal actually opens, not on every
  // `username` change - a successful rename calls updateUsername() right
  // before setting the success message below, and that context update
  // would otherwise re-run this effect and immediately clear it again. The
  // ref keeps the latest username available without making the effect
  // depend on (and re-run for) it.
  const latestUsername = useRef(username);
  useEffect(() => {
    latestUsername.current = username;
  }, [username]);

  useEffect(() => {
    if (!open) return;
    setNewUsername(latestUsername.current);
    setCurrentPassword("");
    setNewPassword("");
    setConfirmPassword("");
    setError("");
    setMessage("");
  }, [open]);

  const trimmedUsername = newUsername.trim();
  const usernameChanged = trimmedUsername !== "" && trimmedUsername !== username;
  const passwordMismatch = newPassword !== "" && newPassword !== confirmPassword;

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setError("");
    setMessage("");
    if (!usernameChanged && newPassword === "") {
      setError(t("account.nothingToUpdate"));
      return;
    }
    if (newPassword !== "" && newPassword.length < 12) {
      setError(t("account.weakPassword"));
      return;
    }
    if (passwordMismatch) {
      setError(t("account.passwordMismatch"));
      return;
    }
    setBusy(true);
    try {
      const result = await updateAccount({
        current_password: currentPassword,
        new_username: usernameChanged ? trimmedUsername : undefined,
        new_password: newPassword || undefined,
      });
      updateUsername(result.username);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setMessage(t("account.updateSuccess"));
    } catch (cause) {
      const code = cause instanceof Error ? cause.message : "";
      if (code === "invalid_current_password") setError(t("account.invalidCurrentPassword"));
      else if (code === "weak_password") setError(t("account.weakPassword"));
      else if (code === "username_too_long") setError(t("account.usernameTooLong"));
      else if (code === "nothing_to_update") setError(t("account.nothingToUpdate"));
      else if (code === "rate_limited") setError(t("account.rateLimited"));
      // Rare: the credential change itself was durably saved, but the
      // server then failed to invalidate other sessions (and failed to
      // roll the credential change back too) - the account was genuinely
      // changed despite the error, so this needs its own message rather
      // than the generic "could not update" one, which would wrongly
      // imply nothing happened.
      else if (code === "partial_update") setError(t("account.partialUpdate"));
      else setError(t("account.updateError"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <ContentModal open={open} size="medium" eyebrow={t("account.eyebrow")} title={t("account.title")} closeLabel={t("common.close")} onClose={onClose} returnFocusTo={returnFocusTo}>
      <p className="muted-copy">{t("account.intro")}</p>
      <form className="account-form" onSubmit={submit}>
        <label>
          <span>{t("account.username")}</span>
          <input
            autoComplete="username"
            value={newUsername}
            onChange={(event) => setNewUsername(event.target.value)}
            disabled={busy}
            maxLength={128}
            required
          />
        </label>

        <div className="account-form-divider" />

        <label>
          <span>{t("account.newPassword")}</span>
          <div className="account-password-field">
            <input
              type={showPasswords ? "text" : "password"}
              autoComplete="new-password"
              minLength={12}
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              disabled={busy}
            />
            <button type="button" onClick={() => setShowPasswords((visible) => !visible)} aria-label={showPasswords ? t("login.hidePassword") : t("login.showPassword")}>
              {showPasswords ? <EyeOff /> : <Eye />}
            </button>
          </div>
          <small>{t("account.newPasswordHelp")}</small>
        </label>
        {newPassword !== "" && (
          <label>
            <span>{t("account.confirmPassword")}</span>
            <input
              type={showPasswords ? "text" : "password"}
              autoComplete="new-password"
              minLength={12}
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              disabled={busy}
            />
            {passwordMismatch && <small className="account-field-warning">{t("account.passwordMismatch")}</small>}
          </label>
        )}

        <div className="account-form-divider" />

        <label>
          <span>{t("account.currentPassword")}</span>
          <input
            type={showPasswords ? "text" : "password"}
            autoComplete="current-password"
            value={currentPassword}
            onChange={(event) => setCurrentPassword(event.target.value)}
            disabled={busy}
            required
          />
          <small>{t("account.currentPasswordHelp")}</small>
        </label>

        {error && <div className="feedback error" role="alert">{error}</div>}
        {message && <div className="feedback success" role="status">{message}</div>}

        <div className="account-form-actions">
          <button className="rg-button rg-button-primary" type="submit" disabled={busy || currentPassword === ""}>
            {busy ? t("account.saving") : t("account.save")}
          </button>
        </div>
      </form>
    </ContentModal>
  );
}
