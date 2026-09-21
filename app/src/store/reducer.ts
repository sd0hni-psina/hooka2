import type { HallState, Hookah, ServiceEvent } from '../api/types';
import type { ParsedMessage } from '../ws/envelope';

// Banner is the in-app notice of a task that just fired, tech.md section 10.
export type Banner = { eventId: string; hookahId: string; tableId: string; tableLabel: string; title: string };

// A local action the master already sees but the server has not confirmed,
// tech.md section 19.15. The patched event is what screens render until the
// server answers or a WS event overwrites it.
export type PendingAction = 'claim' | 'confirm';
export type Optimistic = { action: PendingAction; hookahId: string; event: ServiceEvent; since: number };

export type HallData = {
  hallId: string | null;
  // seq of the last applied message; null until the first snapshot.
  seq: number | null;
  state: HallState | null;
  banner: Banner | null;
  // stale marks a hall raised from the snapshot cache, before the socket
  // confirmed anything. Mutations wait for a live snapshot.
  stale: boolean;
  // savedAt is when the cached snapshot was written, null for live data.
  savedAt: string | null;
  pending: Record<string, Optimistic>;
};

export const emptyHall: HallData = {
  hallId: null, seq: null, state: null, banner: null, stale: false, savedAt: null, pending: {},
};

export type Reduced = {
  data: HallData;
  // resync asks the WS client to subscribe again and replace the store.
  resync: boolean;
};

// reduce is the only place that interprets WS messages, tech.md section 15.
// A snapshot always replaces the store. Any seq other than last + 1 is a gap,
// including a lower seq after a server restart. Every event applies
// idempotently: the same message twice leaves the same store.
export function reduce(data: HallData, msg: ParsedMessage): Reduced {
  switch (msg.kind) {
    case 'snapshot':
      // A late snapshot of the previous hall after a switch is stale.
      if (msg.env.hall_id !== data.hallId) return { data, resync: false };
      // The snapshot is the truth: every local guess goes with it.
      return {
        data: {
          ...data,
          seq: msg.env.seq,
          state: msg.state,
          banner: bannerStillDue(data.banner, msg.state),
          stale: false,
          savedAt: null,
          pending: {},
        },
        resync: false,
      };
    case 'pong':
    case 'error':
      return { data, resync: false };
    default: {
      const state = data.state;
      if (data.seq === null || state === null) return { data, resync: false };
      if (msg.env.seq !== data.seq + 1) return { data, resync: true };
      const next = { ...data, seq: msg.env.seq };
      // A transfer concerns both halls, so it applies whatever hall it names.
      const ours = msg.env.hall_id === null || msg.env.hall_id === data.hallId || msg.kind === 'hookah.transferred';
      if (!ours) return { data: next, resync: false };
      const applied = applyEvent(state, data.hallId, msg);
      return {
        data: {
          ...next,
          state: applied,
          banner: applyBanner(data.banner, applied, msg),
          pending: settlePending(data.pending, msg),
        },
        resync: false,
      };
    }
  }
}

type EventMessage = Exclude<ParsedMessage, { kind: 'snapshot' | 'pong' | 'error' }>;

function applyEvent(state: HallState, hallId: string | null, msg: EventMessage): HallState {
  switch (msg.kind) {
    case 'layout':
      return { ...state, tables: msg.tables };
    case 'plan':
      // Live hookahs keep their snapshot; the venue plan only drives new ones.
      return { ...state, plan: msg.plan };
    case 'hookah.started':
      return withHookah(state, msg.hookah);
    case 'hookah.transferred': {
      const without = { ...state, hookahs: state.hookahs.filter((h) => h.id !== msg.hookah.id) };
      // The hookah left this hall: the old table is free now.
      return msg.hookah.hall_id === hallId ? withHookah(without, msg.hookah) : without;
    }
    case 'hookah.expired':
      return mapHookah(state, msg.hookahId, (h) => ({ ...h, state: 'expired', status: msg.status }));
    case 'hookah.finished':
      return { ...state, hookahs: state.hookahs.filter((h) => h.id !== msg.hookahId) };
    case 'service.due':
      return mapHookah(state, msg.hookahId, (h) => ({ ...h, status: msg.status, events: withEvent(h.events, msg.event) }));
    case 'service.missed':
      return mapHookah(state, msg.hookahId, (h) => ({
        ...h,
        status: msg.status,
        events: h.events.map((e) => (e.id === msg.eventId ? { ...e, state: 'missed' } : e)),
      }));
    case 'service.claimed':
      return mapEvent(state, msg.hookahId, msg.eventId, (e) => ({
        ...e,
        claimed_by: msg.claimedBy,
        claimed_by_name: msg.claimedByName,
        claimed_at: msg.claimedAt,
      }));
    case 'service.confirmed':
      return mapHookah(state, msg.hookahId, (h) => ({
        ...h,
        status: msg.status,
        events: h.events.map((e) =>
          e.id === msg.eventId
            ? {
                ...e,
                state: 'confirmed',
                confirmed_by: msg.confirmedBy,
                confirmed_by_name: msg.confirmedByName,
                confirmed_at: msg.confirmedAt,
                confirmed_early: msg.confirmedEarly,
              }
            : e,
        ),
      }));
    case 'event':
      return state;
  }
}

// withHookah replaces the hookah by id and frees any table it now occupies.
function withHookah(state: HallState, hookah: Hookah): HallState {
  const rest = state.hookahs.filter((h) => h.id !== hookah.id && h.table_id !== hookah.table_id);
  return { ...state, hookahs: [...rest, hookah] };
}

function mapHookah(state: HallState, hookahId: string, fn: (h: Hookah) => Hookah): HallState {
  if (!state.hookahs.some((h) => h.id === hookahId)) return state;
  return { ...state, hookahs: state.hookahs.map((h) => (h.id === hookahId ? fn(h) : h)) };
}

function mapEvent(state: HallState, hookahId: string, eventId: string, fn: (e: ServiceEvent) => ServiceEvent): HallState {
  return mapHookah(state, hookahId, (h) => ({ ...h, events: h.events.map((e) => (e.id === eventId ? fn(e) : e)) }));
}

// settlePending drops a local guess as soon as the server speaks about the
// same event. The server version always wins, tech.md section 19.15.
function settlePending(pending: Record<string, Optimistic>, msg: EventMessage): Record<string, Optimistic> {
  const ids = pendingIdsFor(pending, msg);
  if (ids.length === 0) return pending;
  const next = { ...pending };
  for (const id of ids) delete next[id];
  return next;
}

function pendingIdsFor(pending: Record<string, Optimistic>, msg: EventMessage): string[] {
  switch (msg.kind) {
    case 'service.claimed':
    case 'service.confirmed':
    case 'service.missed':
      return pending[msg.eventId] ? [msg.eventId] : [];
    case 'service.due':
      return pending[msg.event.id] ? [msg.event.id] : [];
    case 'hookah.finished':
      return Object.entries(pending).filter(([, p]) => p.hookahId === msg.hookahId).map(([id]) => id);
    default:
      return [];
  }
}

// bannerStillDue keeps a notice across a snapshot only when its task is still
// due. The events that would have taken it down may have died with the link,
// and a button on a closed task answers event_not_actionable.
function bannerStillDue(banner: Banner | null, state: HallState): Banner | null {
  if (!banner) return null;
  const hookah = state.hookahs.find((h) => h.id === banner.hookahId);
  const event = hookah?.events.find((e) => e.id === banner.eventId);
  return event?.state === 'due' ? banner : null;
}

// applyBanner raises the in-app notice on a fired task and takes it down as
// soon as the task leaves the due state, whoever closed it.
function applyBanner(current: Banner | null, state: HallState, msg: EventMessage): Banner | null {
  if (msg.kind === 'service.due') {
    const hookah = state.hookahs.find((h) => h.id === msg.hookahId);
    const table = state.tables.find((t) => t.id === hookah?.table_id);
    if (!hookah || !table) return current;
    return { eventId: msg.event.id, hookahId: hookah.id, tableId: table.id, tableLabel: table.label, title: msg.event.title };
  }
  if (!current) return null;
  const closed =
    (msg.kind === 'service.confirmed' && msg.eventId === current.eventId) ||
    (msg.kind === 'service.missed' && msg.eventId === current.eventId) ||
    (msg.kind === 'hookah.finished' && msg.hookahId === current.hookahId);
  return closed ? null : current;
}

function withEvent(events: ServiceEvent[], event: ServiceEvent): ServiceEvent[] {
  return events.some((e) => e.id === event.id) ? events.map((e) => (e.id === event.id ? event : e)) : [...events, event];
}
