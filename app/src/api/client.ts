import { ApiError, NetworkError, parseApiError } from './errors';

export const apiUrl = (process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080').replace(/\/$/, '');
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
  try {
    return await fetch(`${apiBase}${path}`, {
      method,
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    });
  } catch (e) {
    throw new NetworkError(e);
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
