import type { HallState, Hookah, ServiceEvent } from '../api/types';
import type { ParsedMessage } from '../ws/envelope';

export type HallData = {
  hallId: string | null;
  // seq of the last applied message; null until the first snapshot.
  seq: number | null;
  state: HallState | null;
};

export const emptyHall: HallData = { hallId: null, seq: null, state: null };

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
      return { data: { ...data, seq: msg.env.seq, state: msg.state }, resync: false };
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
      return { data: { ...next, state: applyEvent(state, data.hallId, msg) }, resync: false };
    }
  }
}

type EventMessage = Exclude<ParsedMessage, { kind: 'snapshot' | 'pong' | 'error' }>;

function applyEvent(state: HallState, hallId: string | null, msg: EventMessage): HallState {
  switch (msg.kind) {
    case 'layout':
      return { ...state, tables: msg.tables };
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

function withEvent(events: ServiceEvent[], event: ServiceEvent): ServiceEvent[] {
  return events.some((e) => e.id === event.id) ? events.map((e) => (e.id === event.id ? event : e)) : [...events, event];
}
