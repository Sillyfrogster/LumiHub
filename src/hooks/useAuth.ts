import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { useState, useEffect } from 'react';

export interface UserSettings {
  nsfwEnabled: boolean;
  nsfwUnblurred: boolean;
  defaultIncludeTags: string[];
  defaultExcludeTags: string[];
}

export interface User {
  id: string;
  discordId: string;
  username: string;
  displayName: string;
  avatar: string | null;
  banner: string | null;
  role: 'user' | 'admin' | 'moderator';
  createdAt: string;
  settings?: UserSettings;
}

interface AuthState {
  user: User | null;
  _hasChecked: boolean;
  setUser: (user: User | null) => void;
  reset: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      _hasChecked: false,
      setUser: (user) => set({ user, _hasChecked: true }),
      reset: () => set({ user: null, _hasChecked: false }),
    }),
    { name: 'lumihub-auth' }
  )
);

let _fetchPromise: Promise<User | null> | null = null;

async function fetchCurrentUser(): Promise<User | null> {
  if (!_fetchPromise) {
    _fetchPromise = (async () => {
      let res = await fetch('/api/v1/user/@me', { credentials: 'include' });

      // Session expired — try refreshing the token
      if (res.status === 401) {
        const refreshRes = await fetch('/api/v1/auth/refresh', {
          method: 'POST',
          credentials: 'include',
        });
        if (refreshRes.ok) {
          res = await fetch('/api/v1/user/@me', { credentials: 'include' });
        }
      }

      if (!res.ok) return null;

      const user: User = await res.json();

      // Fetch settings in parallel — non-blocking, best-effort
      try {
        const settingsRes = await fetch('/api/v1/user/@me/settings', { credentials: 'include' });
        if (settingsRes.ok) {
          user.settings = await settingsRes.json();
        }
      } catch {}

      return user;
    })().finally(() => { _fetchPromise = null; });
  }
  return _fetchPromise;
}

export function useAuth() {
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const [isLoading, setIsLoading] = useState(() => !useAuthStore.getState()._hasChecked);

  useEffect(() => {
    // Always re-validate session on mount, even if we have cached data
    fetchCurrentUser()
      .then((u) => setUser(u))
      .finally(() => setIsLoading(false));
  }, []);

  const logout = () => {
    fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' })
      .catch((err) => console.error('Logout request failed:', err));
    setUser(null);
  };

  return {
    user,
    isAuthenticated: !!user,
    isLoading,
    logout,
  };
}
