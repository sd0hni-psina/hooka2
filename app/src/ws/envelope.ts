import { arr, decodeEnvelope, decodeHallState, decodeTable, obj, str } from '../api/decode';
import type { Envelope, HallState, Table } from '../api/types';

export type ParsedMessage =
  | { kind: 'snapshot'; env: Envelope; state: HallState }
  | { kind: 'pong'; env: Envelope; serverTime: string }
  | { kind: 'error'; env: Envelope; code: string; message: string }
  | { kind: 'layout'; env: Envelope; hallId: string; tables: Table[] }
  | { kind: 'event'; env: Envelope };

// parseMessage decodes one raw WS frame. Unknown event types pass through as
// 'event' so the reducer can still track their seq.
export function parseMessage(raw: string): ParsedMessage {
  const env = decodeEnvelope(JSON.parse(raw) as unknown);
  switch (env.type) {
    case 'state.snapshot':
      return { kind: 'snapshot', env, state: decodeHallState(env.data, 'data') };
    case 'pong':
      return { kind: 'pong', env, serverTime: str(obj(env.data, 'data'), 'server_time', 'data') };
    case 'layout.updated': {
      const d = obj(env.data, 'data');
      return { kind: 'layout', env, hallId: str(d, 'hall_id', 'data'), tables: arr(d, 'tables', 'data', decodeTable) };
    }
    case 'error': {
      const d = obj(env.data, 'data');
      return { kind: 'error', env, code: str(d, 'code', 'data'), message: str(d, 'message', 'data') };
    }
    default:
      return { kind: 'event', env };
  }
}

export function clientMessage(type: 'subscribe' | 'ping', data: Record<string, string>): string {
  return JSON.stringify({ v: 1, type, data });
}
