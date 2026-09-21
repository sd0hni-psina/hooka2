import { ApiError, NetworkError } from '../../api/errors';

// The server stores an Idempotency-Key for good, so a key that outlives its
// attempt turns the next start on that table into a replay of the old one.
// Two minutes cover a retry after a bad network and nothing more, tech.md
// section 19.17.
const KEY_TTL_MS = 2 * 60_000;

type Live = { key: string; since: number };

// retryable tells whether the request may have reached the server. Then the
// same key on the next try returns the hookah that was created, if any.
function retryable(e: unknown): boolean {
  return e instanceof NetworkError || (e instanceof ApiError && e.status >= 500);
}

// StartKeys hands out one Idempotency-Key per table for the life of a start.
export class StartKeys {
  private readonly live = new Map<string, Live>();

  constructor(private readonly newKey: () => string) {}

  // run sends with the table's key. Success and any answer of the server that
  // is not a 5xx end the key; a network error or a 5xx keeps it for the retry.
  async run<T>(tableId: string, now: number, send: (key: string) => Promise<T>): Promise<T> {
    const key = this.keyFor(tableId, now);
    try {
      const result = await send(key);
      this.live.delete(tableId);
      return result;
    } catch (e) {
      if (!retryable(e)) this.live.delete(tableId);
      throw e;
    }
  }

  private keyFor(tableId: string, now: number): string {
    const held = this.live.get(tableId);
    if (held && now - held.since < KEY_TTL_MS) return held.key;
    const key = this.newKey();
    this.live.set(tableId, { key, since: now });
    return key;
  }
}
