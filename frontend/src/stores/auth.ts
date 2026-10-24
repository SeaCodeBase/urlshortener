import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { api } from '@/lib/api';
import { getQueryClient } from '@/lib/query';
import type { Group, User } from '@/types';

interface AuthState {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  /** Group sent as X-Group-ID on every request (persisted). */
  activeGroupId: number | null;
  groups: Group[];
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => void;
  checkAuth: () => Promise<void>;
  setAuth: (token: string, user: User) => void;
  setActiveGroup: (id: number | null) => void;
  /** Fetches the user's groups and makes sure activeGroupId is one of them. */
  loadGroups: () => Promise<void>;
}

/** Keeps activeGroupId if it still exists, else the user's default, else the first group. */
function pickActiveGroup(current: number | null, groups: Group[], user: User | null): number | null {
  if (current != null && groups.some((g) => g.id === current)) return current;
  const def = user?.default_group_id;
  if (def != null && groups.some((g) => g.id === def)) return def;
  return groups[0]?.id ?? null;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      isLoading: true,
      activeGroupId: null,
      groups: [],

      login: async (email: string, password: string) => {
        const response = await api.login(email, password);
        get().setAuth(response.token, response.user);
        await get().loadGroups().catch(() => {});
      },

      register: async (email: string, password: string) => {
        const response = await api.register(email, password);
        get().setAuth(response.token, response.user);
        await get().loadGroups().catch(() => {});
      },

      logout: () => {
        api.setToken(null);
        set({ user: null, token: null, activeGroupId: null, groups: [] });
        // Drop every cached query so the next user never sees this one's data.
        getQueryClient().clear();
      },

      checkAuth: async () => {
        const { token } = get();
        if (!token) {
          set({ isLoading: false });
          return;
        }

        api.setToken(token);
        try {
          const user = await api.me();
          set({ user });
        } catch {
          set({ user: null, token: null, activeGroupId: null, groups: [], isLoading: false });
          return;
        }
        // A failure to load groups keeps the user signed in; the backend
        // falls back to the default group when X-Group-ID is missing.
        await get().loadGroups().catch(() => {});
        set({ isLoading: false });
      },

      setAuth: (token: string, user: User) => {
        api.setToken(token);
        set({ user, token });
      },

      setActiveGroup: (id: number | null) => {
        set({ activeGroupId: id });
      },

      loadGroups: async () => {
        const { groups } = await api.getGroups();
        set((state) => ({
          groups,
          activeGroupId: pickActiveGroup(state.activeGroupId, groups, state.user),
        }));
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({ token: state.token, activeGroupId: state.activeGroupId }),
    }
  )
);

// The API client reads the active group from here (also after rehydration),
// so the store stays the single source of truth without a circular import.
api.setGroupId(useAuthStore.getState().activeGroupId);
useAuthStore.subscribe((state) => api.setGroupId(state.activeGroupId));
// A 401 on any authenticated request signs the user out (once: logout clears the token).
api.onUnauthorized(() => useAuthStore.getState().logout());
