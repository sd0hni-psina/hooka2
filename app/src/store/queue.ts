import { NetworkError } from '../api/errors';
import type { PendingAction } from './reducer';

// Actions that survive a dead network, tech.md section 19.15. Only claim and
// confirm qualify: both are idempotent on the server, so a late replay costs
// nothing. Start, transfer and finish still demand a live connection.
const MAX_ITEMS = 32;
const MAX_AGE_MS = 10 * 60 * 1000;
const MIN_RUN_GAP_MS = 3_000;

export type Queued = { kind: PendingAction; eventId: string; queuedAt: number };

export class RetryQueue {
  private items: Queued[] = [];
  private lastRun = Number.NEGATIVE_INFINITY;
  private running = false;

  push(kind: PendingAction, eventId: string, nowMs: number): void {
    this.items = this.items.filter((i) => !(i.eventId === eventId && i.kind === kind));
    this.items.push({ kind, eventId, queuedAt: nowMs });
    if (this.items.length > MAX_ITEMS) this.items = this.items.slice(-MAX_ITEMS);
  }

  drop(eventId: string): void {
    this.items = this.items.filter((i) => i.eventId !== eventId);
  }

  size(): number {
    return this.items.length;
  }

  clear(): void {
    this.items = [];
  }

  // run sends what is waiting. A network failure keeps the entry, any answer
  // from the server retires it: there is nothing left to argue about.
  async run(
    send: (item: Queued) => Promise<void>,
    onFail: (item: Queued, err: unknown) => void,
    nowMs: number,
  ): Promise<void> {
    if (this.running || nowMs - this.lastRun < MIN_RUN_GAP_MS) return;
    this.running = true;
    this.lastRun = nowMs;
    try {
      const expired = this.items.filter((i) => nowMs - i.queuedAt > MAX_AGE_MS);
      for (const item of expired) onFail(item, new Error('expired'));
      const live = this.items.filter((i) => nowMs - i.queuedAt <= MAX_AGE_MS);
      this.items = [];
      for (const item of live) {
        try {
          await send(item);
        } catch (e) {
          if (e instanceof NetworkError) this.items.push(item);
          else onFail(item, e);
        }
      }
    } finally {
      this.running = false;
    }
  }
}
