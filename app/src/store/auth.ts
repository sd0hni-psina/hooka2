import { create } from 'zustand';

import { setTokenSource } from '../api/client';
import { getMe } from '../api/endpoints';
import { ApiError } from '../api/errors';
import { clearSession, loadSession, type Session } from '../api/session';
import type { Me } from '../api/types';

export type AuthStatus = 'loading' | 'signedOut' | 'signedIn';

type AuthStore = {
  status: AuthStatus;
  session: Session | null;
  me: Me | null;
  error: unknown;
  bootstrap: () => Promise<void>;
  refreshMe: () => Promise<void>;
  signOut: () => Promise<void>;
};

export const useAuthStore = create<AuthStore>((set, get) => ({
  status: 'loading',
  session: null,
  me: null,
  error: null,

  bootstrap: async () => {
    const session = await loadSession();
    if (!session) {
      set({ status: 'signedOut', session: null, me: null });
      return;
    }
    set({ session });
    await get().refreshMe();
  },

  refreshMe: async () => {
    try {
      const me = await getMe();
      set({ me, status: 'signedIn', error: null });
    } catch (e) {
      if (e instanceof ApiError && (e.code === 'unauthorized' || e.code === 'forbidden')) {
        await get().signOut();
        return;
      }
      // Network trouble keeps the session: the app shows the error and retries.
      set({ error: e, status: get().me ? 'signedIn' : 'signedOut' });
    }
  },

  signOut: async () => {
    await clearSession();
    set({ status: 'signedOut', session: null, me: null });
  },
}));

setTokenSource(async () => useAuthStore.getState().session?.access_token ?? null);
