import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { AuthContext } from "./state";

interface SessionResponse {
  authenticated: boolean;
  username?: string;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [loading, setLoading] = useState(true);
  const [authenticated, setAuthenticated] = useState(false);
  const [username, setUsername] = useState("");

  const clearSession = useCallback(() => {
    setAuthenticated(false);
    setUsername("");
  }, []);

  useEffect(() => {
    fetch("/api/auth/session", { cache: "no-store" })
      .then(async (response) => {
        if (!response.ok) throw new Error("No active session");
        const session = await response.json() as SessionResponse;
        setAuthenticated(session.authenticated);
        setUsername(session.username ?? "");
      })
      .catch(clearSession)
      .finally(() => setLoading(false));
  }, [clearSession]);

  useEffect(() => {
    window.addEventListener("rootguard:unauthorized", clearSession);
    return () => window.removeEventListener("rootguard:unauthorized", clearSession);
  }, [clearSession]);

  const login = useCallback(async (loginUsername: string, password: string) => {
    const response = await fetch("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: loginUsername, password }),
    });
    if (!response.ok) {
      // The body carries which failure this was (invalid_credentials vs.
      // rate_limited) so the login form can show the right message instead
      // of always blaming the password.
      const body = await response.json().catch(() => null) as { error?: string } | null;
      throw new Error(body?.error ?? "invalid_credentials");
    }
    const session = await response.json() as SessionResponse;
    setAuthenticated(true);
    setUsername(session.username ?? loginUsername);
  }, []);

  const logout = useCallback(async () => {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } finally {
      clearSession();
    }
  }, [clearSession]);

  const value = useMemo(
    () => ({ loading, authenticated, username, login, logout, updateUsername: setUsername }),
    [loading, authenticated, username, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
