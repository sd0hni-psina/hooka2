import {
  arr, decodeEnvelope, decodeEvent, decodeHallState, decodeHookah, decodeStatus, decodeTable, obj, str,
} from '../api/decode';
import type { Envelope, HallState, Hookah, ServiceEvent, Table, TableStatus } from '../api/types';

export type ParsedMessage =
  | { kind: 'snapshot'; env: Envelope; state: HallState }
  | { kind: 'pong'; env: Envelope; serverTime: string }
  | { kind: 'error'; env: Envelope; code: string; message: string }
  | { kind: 'layout'; env: Envelope; hallId: string; tables: Table[] }
  | { kind: 'hookah.started'; env: Envelope; hookah: Hookah }
  | { kind: 'hookah.transferred'; env: Envelope; hookah: Hookah; fromTableId: string }
  | { kind: 'hookah.expired'; env: Envelope; hookahId: string; tableId: string; status: TableStatus }
  | { kind: 'hookah.finished'; env: Envelope; hookahId: string; tableId: string }
  | { kind: 'service.due'; env: Envelope; event: ServiceEvent; hookahId: string; status: TableStatus }
  | { kind: 'service.missed'; env: Envelope; eventId: string; hookahId: string; status: TableStatus }
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
    case 'hookah.started': {
      const d = obj(env.data, 'data');
      return { kind: 'hookah.started', env, hookah: decodeHookah(d.hookah, 'data.hookah') };
    }
    case 'hookah.transferred': {
      const d = obj(env.data, 'data');
      return {
        kind: 'hookah.transferred', env,
        hookah: decodeHookah(d.hookah, 'data.hookah'),
        fromTableId: str(d, 'from_table_id', 'data'),
      };
    }
    case 'hookah.expired': {
      const d = obj(env.data, 'data');
      return {
        kind: 'hookah.expired', env,
        hookahId: str(d, 'hookah_id', 'data'), tableId: str(d, 'table_id', 'data'), status: decodeStatus(d, 'status', 'data'),
      };
    }
    case 'hookah.finished': {
      const d = obj(env.data, 'data');
      return { kind: 'hookah.finished', env, hookahId: str(d, 'hookah_id', 'data'), tableId: str(d, 'table_id', 'data') };
    }
    case 'service.due': {
      const d = obj(env.data, 'data');
      return {
        kind: 'service.due', env, event: decodeEvent(d.event, 'data.event'),
        hookahId: str(d, 'hookah_id', 'data'), status: decodeStatus(d, 'status', 'data'),
      };
    }
    case 'service.missed': {
      const d = obj(env.data, 'data');
      return {
        kind: 'service.missed', env, eventId: str(d, 'event_id', 'data'),
        hookahId: str(d, 'hookah_id', 'data'), status: decodeStatus(d, 'status', 'data'),
      };
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
