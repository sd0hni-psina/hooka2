import { create } from 'zustand';

import { setAuthProvider } from '../api/client';
import { getMe, logout as logoutRequest, refreshTokens } from '../api/endpoints';
import { ApiError } from '../api/errors';
import { clearSession, loadHallId, loadSession, saveHallId, saveSession, type Session } from '../api/session';
import type { Me } from '../api/types';
import { serverNow } from '../ws/clock';

export type AuthStatus = 'loading' | 'signedOut' | 'signedIn';

// Refresh when less than this is left on the access token, tech.md section 19.4.
const REFRESH_AHEAD_MS = 60_000;

type AuthStore = {
  status: AuthStatus;
  session: Session | null;
  me: Me | null;
  hallId: string | null;
  error: unknown;
  bootstrap: () => Promise<void>;
  signIn: (s: Session) => Promise<void>;
  refreshMe: () => Promise<void>;
  selectHall: (id: string) => Promise<void>;
  signOut: () => Promise<void>;
  accessToken: () => Promise<string | null>;
  refreshSession: () => Promise<boolean>;
};

let inflight: Promise<boolean> | null = null;

export const useAuthStore = create<AuthStore>((set, get) => ({
  status: 'loading',
  session: null,
  me: null,
  hallId: null,
  error: null,

  bootstrap: async () => {
    const session = await loadSession();
    if (!session) {
      set({ status: 'signedOut', session: null, me: null });
      return;
    }
    set({ session, hallId: await loadHallId() });
    await get().refreshMe();
  },

  signIn: async (session) => {
    await saveSession(session);
    set({ session, error: null });
    await get().refreshMe();
  },

  refreshMe: async () => {
    try {
      const me = await getMe();
      const hallId = get().hallId;
      const known = hallId !== null && me.halls.some((h) => h.id === hallId);
      set({ me, status: 'signedIn', error: null, hallId: known ? hallId : null });
    } catch (e) {
      if (e instanceof ApiError && (e.code === 'unauthorized' || e.code === 'forbidden')) {
        await get().signOut();
        return;
      }
      // Network trouble keeps the session: the screen shows the error and retries.
      set({ error: e, status: get().me ? 'signedIn' : 'signedOut' });
    }
  },

  selectHall: async (id) => {
    await saveHallId(id);
    set({ hallId: id });
  },

  signOut: async () => {
    if (get().session) {
      try {
        await logoutRequest();
      } catch {
        // The session is dropped locally either way; the server revokes it on expiry.
      }
    }
    await clearSession();
    set({ status: 'signedOut', session: null, me: null, hallId: null });
  },

  accessToken: async () => {
    const s = get().session;
    if (!s) return null;
    const expires = Date.parse(s.access_expires_at);
    if (s.refresh_token && !Number.isNaN(expires) && expires - serverNow() < REFRESH_AHEAD_MS) {
      try {
        await get().refreshSession();
      } catch {
        // The refresh is early insurance. The current token still works for
        // up to a minute, so a proxy hiccup must not cost the request.
      }
    }
    return get().session?.access_token ?? null;
  },

  // Concurrent callers share one /auth/refresh call. Resolves true with a new
  // token, false when the session is gone, and rejects when the server could
  // not be asked: the caller sees trouble, the session survives.
  refreshSession: () => {
    if (inflight) return inflight;
    const run = async (): Promise<boolean> => {
      const s = get().session;
      if (!s?.refresh_token) return false;
      try {
        const tokens = await refreshTokens(s.refresh_token);
        await saveSession(tokens);
        set({ session: tokens });
        return true;
      } catch (e) {
        // Only a verdict of the server ends the session. A 502 from the proxy
        // during a deploy or a dead network says nothing about the token,
        // tech.md section 19.16.
        if (e instanceof ApiError && (e.code === 'unauthorized' || e.code === 'forbidden')) {
          await clearSession();
          set({ status: 'signedOut', session: null, me: null, hallId: null });
          return false;
        }
        throw e;
      }
    };
    inflight = run().finally(() => {
      inflight = null;
    });
    return inflight;
  },
}));

setAuthProvider({
  access: () => useAuthStore.getState().accessToken(),
  refresh: () => useAuthStore.getState().refreshSession(),
});
