import Constants from 'expo-constants';

import { ApiError, NetworkError, parseApiError } from './errors';

// A request that never answers must not hold a screen, tech.md section 19.12.
const TIMEOUT_MS = 10_000;
const DEV_API_PORT = 8080;

// devHost reuses the machine that serves the JS bundle: in development the
// API runs on the same laptop, and its address changes with every network.
function devHost(): string | null {
  const host = Constants.expoConfig?.hostUri?.split(':')[0];
  return host ? `http://${host}:${DEV_API_PORT}` : null;
}

function resolveApiUrl(): string {
  const configured = process.env.EXPO_PUBLIC_API_URL?.trim();
  if (configured) return configured;
  return devHost() ?? 'http://localhost:8080';
}

export const apiUrl = resolveApiUrl().replace(/\/$/, '');
export const apiBase = `${apiUrl}/api/v1`;

// AuthProvider is implemented by the auth store. access() returns a token
// that is fresh enough to send; refresh() rotates it after a 401 and reports
// whether a new one is available.
export type AuthProvider = {
  access: () => Promise<string | null>;
  refresh: () => Promise<boolean>;
};

let provider: AuthProvider = { access: async () => null, refresh: async () => false };

export function setAuthProvider(p: AuthProvider) {
  provider = p;
}

type Options = {
  body?: unknown;
  headers?: Record<string, string>;
  auth?: boolean;
};

async function send(method: string, path: string, opts: Options, token: string | null): Promise<Response> {
  const headers: Record<string, string> = { Accept: 'application/json', ...opts.headers };
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
  if (token) headers.Authorization = `Bearer ${token}`;
  const abort = new AbortController();
  const timer = setTimeout(() => abort.abort(), TIMEOUT_MS);
  try {
    return await fetch(`${apiBase}${path}`, {
      method,
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      signal: abort.signal,
    });
  } catch (e) {
    throw new NetworkError(e);
  } finally {
    clearTimeout(timer);
  }
}

// request sends one API call. An expired access token is refreshed once and
// the call repeated, so screens never see the rotation (tech.md section 19.4).
export async function request<T>(method: string, path: string, decode: (u: unknown) => T, opts: Options = {}): Promise<T> {
  const authed = opts.auth !== false;
  let res = await send(method, path, opts, authed ? await provider.access() : null);
  if (authed && res.status === 401 && (await provider.refresh())) {
    res = await send(method, path, opts, await provider.access());
  }

  if (res.status === 204) return decode(undefined);
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    if (res.ok) throw new ApiError('internal', res.status, 'invalid JSON', null);
  }
  if (!res.ok) throw parseApiError(res.status, body);
  return decode(body);
}
