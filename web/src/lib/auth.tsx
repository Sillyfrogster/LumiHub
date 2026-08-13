"use client";

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { components } from "@/lib/api/schema";

export type SignedInAccount = components["schemas"]["Account"];

type AuthContextValue = {
  account: SignedInAccount | null | undefined;
  refresh: () => Promise<void>;
  setAccount: (account: SignedInAccount | null) => void;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [account, setAccount] = useState<SignedInAccount | null | undefined>(
    undefined,
  );

  const refresh = useCallback(async () => {
    try {
      const response = await fetch("/api/v1/auth/session", {
        cache: "no-store",
        credentials: "same-origin",
      });
      if (!response.ok) {
        setAccount(null);
        return;
      }
      const state = (await response.json()) as { user: SignedInAccount | null };
      setAccount(state.user);
    } catch {
      setAccount(null);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const signOut = useCallback(async () => {
    const response = await fetch("/api/v1/auth/sign-out", {
      method: "POST",
      credentials: "same-origin",
    });
    if (!response.ok) throw new Error("Could not sign out");
    setAccount(null);
  }, []);

  const value = useMemo(
    () => ({ account, refresh, setAccount, signOut }),
    [account, refresh, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}
