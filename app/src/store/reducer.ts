import type { HallState } from '../api/types';
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
// including a lower seq after a server restart.
export function reduce(data: HallData, msg: ParsedMessage): Reduced {
  switch (msg.kind) {
    case 'snapshot':
      // A late snapshot of the previous hall after a switch is stale.
      if (msg.env.hall_id !== data.hallId) return { data, resync: false };
      return { data: { ...data, seq: msg.env.seq, state: msg.state }, resync: false };
    case 'pong':
    case 'error':
      return { data, resync: false };
    case 'event': {
      if (data.seq === null || data.state === null) return { data, resync: false };
      if (msg.env.seq !== data.seq + 1) return { data, resync: true };
      const next = { ...data, seq: msg.env.seq };
      const ours = msg.env.hall_id === null || msg.env.hall_id === data.hallId;
      if (!ours) return { data: next, resync: false };
      return { data: { ...next, state: applyEvent(next.state as HallState, msg) }, resync: false };
    }
  }
}

// applyEvent grows with the slices that introduce each event type. Unknown
// types leave the state untouched.
function applyEvent(state: HallState, _msg: Extract<ParsedMessage, { kind: 'event' }>): HallState {
  return state;
}
