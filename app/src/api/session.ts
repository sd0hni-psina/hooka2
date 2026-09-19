import * as SecureStore from 'expo-secure-store';

import { decodeTokens } from './decode';
import type { Tokens } from './types';

const SESSION_KEY = 'hookah.session.v1';
const HALL_KEY = 'hookah.hall.v1';

export type Session = Tokens;

// The dev token lets the app reach the API without the join flow, tech.md
// section 19.2. Release builds never read it.
function devSession(): Session | null {
  const token = process.env.EXPO_PUBLIC_DEV_ACCESS_TOKEN;
  if (!__DEV__ || !token) return null;
  return { access_token: token, refresh_token: '', access_expires_at: '' };
}

export async function loadSession(): Promise<Session | null> {
  const raw = await SecureStore.getItemAsync(SESSION_KEY);
  if (raw) {
    try {
      return decodeTokens(JSON.parse(raw) as unknown);
    } catch {
      await SecureStore.deleteItemAsync(SESSION_KEY);
    }
  }
  return devSession();
}

export async function saveSession(s: Session): Promise<void> {
  await SecureStore.setItemAsync(SESSION_KEY, JSON.stringify(s));
}

export async function clearSession(): Promise<void> {
  await SecureStore.deleteItemAsync(SESSION_KEY);
  await SecureStore.deleteItemAsync(HALL_KEY);
}

export async function loadHallId(): Promise<string | null> {
  return SecureStore.getItemAsync(HALL_KEY);
}

export async function saveHallId(id: string): Promise<void> {
  await SecureStore.setItemAsync(HALL_KEY, id);
}
