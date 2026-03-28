import { createContext, useContext, useMemo, useState } from "react";
import { parseJwt } from "../lib/jwt";

const STORAGE_KEY = "workers_marketplace_auth";

const AuthContext = createContext(null);

function loadSession() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

function persistSession(session) {
  if (!session) {
    localStorage.removeItem(STORAGE_KEY);
    return;
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
}

export function resolveHomePath(role) {
  if (role === "worker") return "/worker/profile";
  if (role === "admin") return "/admin/overview";
  return "/customer/workers";
}

export function AuthProvider({ children }) {
  const [session, setSession] = useState(loadSession);

  const login = (tokens) => {
    const payload = parseJwt(tokens.access_token);
    const nextSession = {
      accessToken: tokens.access_token,
      refreshToken: tokens.refresh_token || null,
      expiresAt: tokens.expires_at || null,
      role: payload?.role || null,
      userId: payload?.user_id || payload?.sub || null,
    };
    persistSession(nextSession);
    setSession(nextSession);
    return nextSession;
  };

  const logout = () => {
    persistSession(null);
    setSession(null);
  };

  const value = useMemo(
    () => ({
      session,
      token: session?.accessToken || null,
      role: session?.role || null,
      userId: session?.userId || null,
      isAuthenticated: Boolean(session?.accessToken),
      homePath: resolveHomePath(session?.role),
      login,
      logout,
    }),
    [session],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return context;
}
