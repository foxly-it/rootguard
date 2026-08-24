import { createContext, useContext } from "react";

export interface AuthState {
  loading: boolean;
  authenticated: boolean;
  username: string;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  // Lets a successful /api/auth/account update reflect a renamed account
  // immediately (e.g. in UserMenu's displayed name) without forcing a
  // reload or a fresh /api/auth/session round trip.
  updateUsername: (username: string) => void;
}

export const AuthContext = createContext<AuthState | null>(null);

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
