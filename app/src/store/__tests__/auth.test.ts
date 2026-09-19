// jest.mock is hoisted above the imports by babel-jest.
import { getMe } from '../../api/endpoints';
import { __setLocalClockForTests } from '../../ws/clock';
import { useAuthStore } from '../auth';

jest.mock('expo-secure-store', () => {
  const mem = new Map<string, string>();
  return {
    getItemAsync: jest.fn(async (k: string) => mem.get(k) ?? null),
    setItemAsync: jest.fn(async (k: string, v: string) => void mem.set(k, v)),
    deleteItemAsync: jest.fn(async (k: string) => void mem.delete(k)),
    __mem: mem,
  };
});


type Call = { url: string; auth: string | null; body: unknown };

const me = {
  staff: { id: 's', venue_id: 'v', display_name: 'Ерлан', role: 'master', blocked: false, created_at: '2026-09-19T17:00:00Z', last_seen_at: null },
  venue: { id: 'v', name: 'Абая', timezone: 'Asia/Atyrau', join_code: null },
  halls: [{ id: 'h1', venue_id: 'v', name: 'Зал', sort_order: 0 }],
};

const json = (status: number, body: unknown) =>
  ({ status, ok: status >= 200 && status < 300, json: async () => body }) as unknown as Response;

const unauthorized = () => json(401, { error: { code: 'unauthorized', message: 'x' } });

function mockServer(handler: (c: Call) => Response) {
  const calls: Call[] = [];
  globalThis.fetch = jest.fn(async (url: RequestInfo | URL, init?: RequestInit) => {
    const headers = (init?.headers ?? {}) as Record<string, string>;
    const c = { url: String(url), auth: headers.Authorization ?? null, body: init?.body ? JSON.parse(String(init.body)) : null };
    calls.push(c);
    return handler(c);
  }) as unknown as typeof fetch;
  return calls;
}

const now = Date.parse('2026-09-19T18:00:00Z');
const session = (access: string, refresh: string, expiresInMs: number) => ({
  access_token: access,
  refresh_token: refresh,
  access_expires_at: new Date(now + expiresInMs).toISOString(),
});

beforeEach(() => {
  __setLocalClockForTests(() => now);
  useAuthStore.setState({ status: 'signedIn', session: session('old-access', 'r1', 20 * 60_000), me: null, hallId: null, error: null });
});

describe('auth store', () => {
  it('refreshes an expired access token transparently and repeats the call', async () => {
    const calls = mockServer((c) => {
      if (c.url.endsWith('/auth/refresh')) return json(200, { tokens: session('new-access', 'r2', 30 * 60_000) });
      return c.auth === 'Bearer new-access' ? json(200, me) : unauthorized();
    });
    await expect(getMe()).resolves.toEqual(me);
    expect(calls.map((c) => c.url.replace(/.*\/api\/v1/, ''))).toEqual(['/me', '/auth/refresh', '/me']);
    expect(useAuthStore.getState().session?.refresh_token).toBe('r2');
  });

  it('refreshes ahead of expiry without a failed call', async () => {
    useAuthStore.setState({ session: session('old-access', 'r1', 30_000) });
    const calls = mockServer((c) => {
      if (c.url.endsWith('/auth/refresh')) return json(200, { tokens: session('new-access', 'r2', 30 * 60_000) });
      return json(200, me);
    });
    await getMe();
    expect(calls.map((c) => c.url.replace(/.*\/api\/v1/, ''))).toEqual(['/auth/refresh', '/me']);
    expect(calls[1]!.auth).toBe('Bearer new-access');
  });

  it('shares one refresh between parallel calls', async () => {
    let refreshes = 0;
    mockServer((c) => {
      if (c.url.endsWith('/auth/refresh')) {
        refreshes += 1;
        return json(200, { tokens: session('new-access', 'r2', 30 * 60_000) });
      }
      return c.auth === 'Bearer new-access' ? json(200, me) : unauthorized();
    });
    await Promise.all([getMe(), getMe(), getMe()]);
    expect(refreshes).toBe(1);
  });

  it('signs out when the refresh token was already rotated', async () => {
    mockServer((c) => (c.url.endsWith('/auth/refresh') ? unauthorized() : unauthorized()));
    await expect(getMe()).rejects.toMatchObject({ code: 'unauthorized' });
    expect(useAuthStore.getState().status).toBe('signedOut');
    expect(useAuthStore.getState().session).toBeNull();
  });

  it('forgets a saved hall that no longer exists', async () => {
    useAuthStore.setState({ hallId: 'gone' });
    mockServer(() => json(200, me));
    await useAuthStore.getState().refreshMe();
    expect(useAuthStore.getState().hallId).toBeNull();
    expect(useAuthStore.getState().status).toBe('signedIn');
  });
});
