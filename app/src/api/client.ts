import { ApiError, NetworkError, parseApiError } from './errors';

export const apiUrl = (process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080').replace(/\/$/, '');
export const apiBase = `${apiUrl}/api/v1`;

type TokenSource = () => Promise<string | null>;
let tokenSource: TokenSource = async () => null;

// The auth store registers where access tokens come from, so this module
// stays free of React and storage details.
export function setTokenSource(src: TokenSource) {
  tokenSource = src;
}

type Options = {
  body?: unknown;
  headers?: Record<string, string>;
  auth?: boolean;
};

export async function request<T>(method: string, path: string, decode: (u: unknown) => T, opts: Options = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json', ...opts.headers };
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
  if (opts.auth !== false) {
    const token = await tokenSource();
    if (token) headers.Authorization = `Bearer ${token}`;
  }

  let res: Response;
  try {
    res = await fetch(`${apiBase}${path}`, {
      method,
      headers,
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    });
  } catch (e) {
    throw new NetworkError(e);
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
