import { create } from 'zustand';

import { claimEvent, confirmEvent } from '../api/endpoints';
import { explain, NetworkError } from '../api/errors';
import type { HallState, ServiceEvent } from '../api/types';
import { serverNow } from '../ws/clock';
import type { ParsedMessage } from '../ws/envelope';
import { loadSnapshot, saveSnapshot } from './cache';
import { RetryQueue, type Queued } from './queue';
import { emptyHall, reduce, type HallData, type Optimistic, type PendingAction } from './reducer';

export type Connection = 'idle' | 'connecting' | 'online' | 'offline';

export type Actor = { id: string; name: string };

// ActResult tells the screen what to say. 'sent' needs no message at all:
// the store already painted the result.
export type ActResult = { status: 'sent' | 'queued' } | { status: 'rejected'; message: string };

type HallStore = HallData & {
  connection: Connection;
  // queued counts actions waiting for the network to come back.
  queued: number;
  selectHall: (hallId: string | null) => void;
  apply: (msg: ParsedMessage) => boolean;
  setConnection: (c: Connection) => void;
  dismissBanner: () => void;
  act: (kind: PendingAction, eventId: string, actor: Actor | null) => Promise<ActResult>;
};

const queue = new RetryQueue();

// heardAt is when the last frame of any kind arrived. It dates the hall the
// moment the link drops, tech.md section 19.17. It is not state: nothing
// renders it, and a write per pong must not wake a subscriber.
let heardAt: number | null = null;

// patch is what the master sees the instant the finger lifts, before the
// server has said anything, tech.md section 19.15.
function patch(event: ServiceEvent, kind: PendingAction, actor: Actor, nowIso: string): ServiceEvent {
  if (kind === 'claim') {
    return { ...event, claimed_by: actor.id, claimed_by_name: actor.name, claimed_at: nowIso };
  }
  return {
    ...event,
    state: 'confirmed',
    confirmed_by: actor.id,
    confirmed_by_name: actor.name,
    confirmed_at: nowIso,
    confirmed_early: event.state !== 'due',
  };
}

function findEvent(state: HallState | null, eventId: string): { hookahId: string; event: ServiceEvent } | null {
  for (const h of state?.hookahs ?? []) {
    const event = h.events.find((e) => e.id === eventId);
    if (event) return { hookahId: h.id, event };
  }
  return null;
}

const send: Record<PendingAction, (eventId: string) => Promise<ServiceEvent>> = {
  claim: claimEvent,
  confirm: confirmEvent,
};

// One store per hall: switching halls drops the previous state entirely.
export const useHallStore = create<HallStore>((set, get) => {
  const setPending = (eventId: string, next: Optimistic | null) => {
    const pending = { ...get().pending };
    if (next) pending[eventId] = next;
    else delete pending[eventId];
    set({ pending });
  };

  // hydrate raises the last known hall out of the cache. It gives up the
  // moment live data exists: a guess never overwrites the truth.
  const hydrate = (hallId: string) => {
    void loadSnapshot(hallId, serverNow()).then((cached) => {
      if (!cached) return;
      const s = get();
      if (s.hallId !== hallId || s.state !== null) return;
      set({ state: cached.state, stale: true, savedAt: cached.savedAt });
    });
  };

  const flush = () => {
    void queue.run(
      async (item: Queued) => {
        const event = await send[item.kind](item.eventId);
        const found = findEvent(get().state, item.eventId);
        setPending(item.eventId, found ? { action: item.kind, hookahId: found.hookahId, event, since: serverNow() } : null);
      },
      (item) => setPending(item.eventId, null),
      serverNow(),
    );
    set({ queued: queue.size() });
  };

  return {
    ...emptyHall,
    connection: 'idle',
    queued: 0,

    selectHall: (hallId) => {
      queue.clear();
      heardAt = null;
      set({ ...emptyHall, hallId, queued: 0 });
      if (hallId) hydrate(hallId);
    },

    apply: (msg) => {
      heardAt = serverNow();
      const r = reduce(get(), msg);
      if (r.data !== get()) set(r.data);
      if (msg.kind === 'snapshot' && r.data.state && r.data.hallId) {
        void saveSnapshot(r.data.hallId, r.data.state, serverNow());
      }
      return r.resync;
    },

    setConnection: (connection) => {
      const was = get().connection;
      // Leaving online, the screen can no longer be trusted: date it. The
      // flag stays until a snapshot of the new connection lands, so a reopened
      // socket that has not spoken yet does not pass for fresh.
      const { state, stale } = get();
      const date = connection !== 'online' && state !== null && !stale;
      set(date ? { connection, stale: true, savedAt: new Date(heardAt ?? serverNow()).toISOString() } : { connection });
      if (connection === 'online' && was !== 'online' && queue.size() > 0) flush();
    },

    dismissBanner: () => set({ banner: null }),

    // act paints the result first and asks the server after. The server
    // answer, or the WS event that follows it, replaces the guess.
    act: async (kind, eventId, actor) => {
      const found = findEvent(get().state, eventId);
      const nowIso = new Date(serverNow()).toISOString();
      if (found && actor) {
        setPending(eventId, {
          action: kind,
          hookahId: found.hookahId,
          event: patch(found.event, kind, actor, nowIso),
          since: serverNow(),
        });
      }
      try {
        const event = await send[kind](eventId);
        queue.drop(eventId);
        setPending(eventId, found ? { action: kind, hookahId: found.hookahId, event, since: serverNow() } : null);
        set({ queued: queue.size() });
        return { status: 'sent' };
      } catch (e) {
        if (e instanceof NetworkError) {
          queue.push(kind, eventId, serverNow());
          set({ queued: queue.size() });
          return { status: 'queued' };
        }
        // The server answered no: drop the guess and let its state stand.
        setPending(eventId, null);
        return { status: 'rejected', message: explain(e) };
      }
    },
  };
});
