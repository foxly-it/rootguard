import { useState, type FormEvent } from "react";
import { ArrowRight, Check, Eye, EyeOff, Filter, LockKeyhole, Network, ShieldCheck } from "lucide-react";
import { useLocation, useNavigate } from "react-router";
import logo from "../assets/rootguard-icon.svg";
import { useAuth } from "../auth";
import { useI18n } from "../i18n";
import "../styles/login.css";

export default function Login() {
  const { login } = useAuth();
  const { locale, locales, setLocale, t } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await login(username.trim(), password);
      const requestedPath = (location.state as { from?: string } | null)?.from;
      navigate(requestedPath && requestedPath !== "/login" ? requestedPath : "/dashboard", { replace: true });
    } catch {
      setError(t("login.invalid"));
      setPassword("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="login-page">
      <div className="login-orb login-orb-one" />
      <div className="login-orb login-orb-two" />

      <header className="login-header">
        <div className="login-brand">
          <img src={logo} alt="" />
          <strong>RootGuard</strong>
        </div>
        <label className="login-language">
          <span>{t("language.label")}</span>
          <select value={locale} onChange={(event) => setLocale(event.target.value)} aria-label={t("language.label")}>
            {locales.map((item) => <option value={item.code} key={item.code}>{item.label}</option>)}
          </select>
        </label>
      </header>

      <section className="login-shell">
        <div className="login-story">
          <span className="login-eyebrow"><ShieldCheck size={14} /> {t("login.protectedConsole")}</span>
          <h1>{t("login.storyTitle")}</h1>
          <p>{t("login.storyText")}</p>

          <div className="login-chain" aria-label={t("login.chainLabel")}>
            <ChainNode icon={<Network />} label={t("setup.host")} />
            <ChainLink />
            <ChainNode icon={<Filter />} label="AdGuard Home" />
            <ChainLink />
            <ChainNode icon={<ShieldCheck />} label="Unbound" />
          </div>

          <div className="login-security-note">
            <Check size={16} />
            <span><strong>{t("login.localSession")}</strong>{t("login.localSessionHelp")}</span>
          </div>
        </div>

        <div className="login-card">
          <div className="login-lock"><LockKeyhole /></div>
          <p className="login-eyebrow">{t("login.adminArea")}</p>
          <h2>{t("login.title")}</h2>
          <p className="login-card-copy">{t("login.intro")}</p>

          <form onSubmit={submit}>
            <label>
              <span>{t("login.username")}</span>
              <input
                autoComplete="username"
                autoFocus
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                disabled={busy}
                required
              />
            </label>
            <label>
              <span>{t("login.password")}</span>
              <div className="login-password">
                <input
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  disabled={busy}
                  required
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((visible) => !visible)}
                  aria-label={showPassword ? t("login.hidePassword") : t("login.showPassword")}
                >
                  {showPassword ? <EyeOff /> : <Eye />}
                </button>
              </div>
            </label>

            {error && <div className="login-error" role="alert">{error}</div>}

            <button className="login-submit" type="submit" disabled={busy}>
              <span>{busy ? t("login.signingIn") : t("login.signIn")}</span>
              <ArrowRight />
            </button>
          </form>
          <p className="login-footnote"><LockKeyhole size={13} /> {t("login.cookieNote")}</p>
        </div>
      </section>
    </main>
  );
}

function ChainNode({ icon, label }: { icon: React.ReactNode; label: string }) {
  return <div className="login-chain-node"><span>{icon}</span><strong>{label}</strong></div>;
}

function ChainLink() {
  return <span className="login-chain-link"><i /></span>;
}
