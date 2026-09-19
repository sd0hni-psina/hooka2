import * as SecureStore from 'expo-secure-store';

import type { Tokens } from './types';

const KEY = 'hookah.session.v1';

export type Session = Tokens;

// The dev token lets slice 0 reach GET /me before the join flow exists,
// tech.md section 19.2. Release builds never read it.
function devSession(): Session | null {
  const token = process.env.EXPO_PUBLIC_DEV_ACCESS_TOKEN;
  if (!__DEV__ || !token) return null;
  return { access_token: token, refresh_token: '', access_expires_at: '' };
}

export async function loadSession(): Promise<Session | null> {
  const raw = await SecureStore.getItemAsync(KEY);
  if (raw) {
    try {
      const parsed: unknown = JSON.parse(raw);
      if (typeof parsed === 'object' && parsed !== null && 'access_token' in parsed && typeof parsed.access_token === 'string') {
        return parsed as Session;
      }
    } catch {
      await SecureStore.deleteItemAsync(KEY);
    }
  }
  return devSession();
}

export async function saveSession(s: Session): Promise<void> {
  await SecureStore.setItemAsync(KEY, JSON.stringify(s));
}

export async function clearSession(): Promise<void> {
  await SecureStore.deleteItemAsync(KEY);
}
