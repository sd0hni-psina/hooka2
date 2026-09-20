// Parses unknown JSON at the network boundary into the typed contract.
// A mismatch throws DecodeError, which callers treat like a server error.
import type {
  EventKind, EventState, Envelope, Hall, HallState, Hookah, HookahState, Me, OnlineStaff,
  Plan, PlanStep, Role, Shape, ServiceEvent, Staff, Table, TableStatus, Tokens, Venue,
} from './types';

export class DecodeError extends Error {
  constructor(path: string, expected: string) {
    super(`decode: ${path} is not ${expected}`);
    this.name = 'DecodeError';
  }
}

type Obj = Record<string, unknown>;

export function obj(u: unknown, path: string): Obj {
  if (typeof u !== 'object' || u === null || Array.isArray(u)) throw new DecodeError(path, 'object');
  return u as Obj;
}

export function str(o: Obj, k: string, path: string): string {
  const v = o[k];
  if (typeof v !== 'string') throw new DecodeError(`${path}.${k}`, 'string');
  return v;
}

export function num(o: Obj, k: string, path: string): number {
  const v = o[k];
  if (typeof v !== 'number' || !Number.isFinite(v)) throw new DecodeError(`${path}.${k}`, 'number');
  return v;
}

export function bool(o: Obj, k: string, path: string): boolean {
  const v = o[k];
  if (typeof v !== 'boolean') throw new DecodeError(`${path}.${k}`, 'boolean');
  return v;
}

export function nstr(o: Obj, k: string, path: string): string | null {
  const v = o[k];
  if (v === null || v === undefined) return null;
  if (typeof v !== 'string') throw new DecodeError(`${path}.${k}`, 'string or null');
  return v;
}

function oneOf<T extends string>(o: Obj, k: string, path: string, allowed: readonly T[]): T {
  const v = str(o, k, path);
  if (!(allowed as readonly string[]).includes(v)) throw new DecodeError(`${path}.${k}`, allowed.join('|'));
  return v as T;
}

export function arr<T>(o: Obj, k: string, path: string, item: (u: unknown, p: string) => T): T[] {
  const v = o[k];
  if (!Array.isArray(v)) throw new DecodeError(`${path}.${k}`, 'array');
  return v.map((x, i) => item(x, `${path}.${k}[${i}]`));
}

const roles = ['owner', 'admin', 'master'] as const satisfies readonly Role[];
const shapes = ['rect', 'circle'] as const satisfies readonly Shape[];
const statuses = ['free', 'green', 'yellow', 'red'] as const satisfies readonly TableStatus[];
const hookahStates = ['active', 'expired', 'finished'] as const satisfies readonly HookahState[];
const kinds = ['check', 'shake', 'replace', 'expire', 'custom'] as const satisfies readonly EventKind[];
const eventStates = ['pending', 'due', 'confirmed', 'missed', 'cancelled'] as const satisfies readonly EventState[];

export function decodeStaff(u: unknown, p = 'staff'): Staff {
  const o = obj(u, p);
  return {
    id: str(o, 'id', p), venue_id: str(o, 'venue_id', p), display_name: str(o, 'display_name', p),
    role: oneOf(o, 'role', p, roles), blocked: bool(o, 'blocked', p),
    created_at: str(o, 'created_at', p), last_seen_at: nstr(o, 'last_seen_at', p),
  };
}

export function decodeVenue(u: unknown, p = 'venue'): Venue {
  const o = obj(u, p);
  return { id: str(o, 'id', p), name: str(o, 'name', p), timezone: str(o, 'timezone', p), join_code: nstr(o, 'join_code', p) };
}

export function decodeHall(u: unknown, p = 'hall'): Hall {
  const o = obj(u, p);
  return { id: str(o, 'id', p), venue_id: str(o, 'venue_id', p), name: str(o, 'name', p), sort_order: num(o, 'sort_order', p) };
}

export function decodeMe(u: unknown): Me {
  const o = obj(u, 'me');
  return { staff: decodeStaff(o.staff), venue: decodeVenue(o.venue), halls: arr(o, 'halls', 'me', decodeHall) };
}

export function decodeTable(u: unknown, p = 'table'): Table {
  const o = obj(u, p);
  return {
    id: str(o, 'id', p), hall_id: str(o, 'hall_id', p), label: str(o, 'label', p), shape: oneOf(o, 'shape', p, shapes),
    x: num(o, 'x', p), y: num(o, 'y', p), w: num(o, 'w', p), h: num(o, 'h', p),
    rotation: num(o, 'rotation', p), seats: num(o, 'seats', p),
  };
}

function decodeStep(u: unknown, p: string): PlanStep {
  const o = obj(u, p);
  const step: PlanStep = { kind: oneOf(o, 'kind', p, kinds), title: str(o, 'title', p), offset_sec: num(o, 'offset_sec', p) };
  if (o.repeat_every_sec !== undefined && o.repeat_every_sec !== null) step.repeat_every_sec = num(o, 'repeat_every_sec', p);
  return step;
}

export function decodePlan(u: unknown, p = 'plan'): Plan {
  const o = obj(u, p);
  return {
    version: num(o, 'version', p), steps: arr(o, 'steps', p, decodeStep),
    warn_before_sec: num(o, 'warn_before_sec', p), grace_sec: num(o, 'grace_sec', p),
    missed_after_sec: num(o, 'missed_after_sec', p), push_repeat_sec: num(o, 'push_repeat_sec', p),
    push_max_attempts: num(o, 'push_max_attempts', p),
  };
}

export function decodeEvent(u: unknown, p = 'event'): ServiceEvent {
  const o = obj(u, p);
  return {
    id: str(o, 'id', p), hookah_id: str(o, 'hookah_id', p), seq: num(o, 'seq', p),
    kind: oneOf(o, 'kind', p, kinds), title: str(o, 'title', p), due_at: str(o, 'due_at', p),
    state: oneOf(o, 'state', p, eventStates),
    claimed_by: nstr(o, 'claimed_by', p), claimed_by_name: nstr(o, 'claimed_by_name', p), claimed_at: nstr(o, 'claimed_at', p),
    confirmed_by: nstr(o, 'confirmed_by', p), confirmed_by_name: nstr(o, 'confirmed_by_name', p),
    confirmed_at: nstr(o, 'confirmed_at', p), confirmed_early: bool(o, 'confirmed_early', p),
  };
}

export function decodeHookah(u: unknown, p = 'hookah'): Hookah {
  const o = obj(u, p);
  return {
    id: str(o, 'id', p), table_id: str(o, 'table_id', p), hall_id: str(o, 'hall_id', p),
    state: oneOf(o, 'state', p, hookahStates), started_at: str(o, 'started_at', p), expires_at: str(o, 'expires_at', p),
    finished_at: nstr(o, 'finished_at', p), started_by: str(o, 'started_by', p), started_by_name: str(o, 'started_by_name', p),
    status: oneOf(o, 'status', p, statuses), events: arr(o, 'events', p, decodeEvent),
  };
}

export function decodeStatus(o: Obj, k: string, p: string): TableStatus {
  return oneOf(o, k, p, statuses);
}

function decodeOnline(u: unknown, p: string): OnlineStaff {
  const o = obj(u, p);
  return { id: str(o, 'id', p), display_name: str(o, 'display_name', p) };
}

export function decodeHallState(u: unknown, p = 'state'): HallState {
  const o = obj(u, p);
  const hall = obj(o.hall, `${p}.hall`);
  return {
    server_time: str(o, 'server_time', p),
    hall: { id: str(hall, 'id', `${p}.hall`), name: str(hall, 'name', `${p}.hall`) },
    plan: decodePlan(o.plan, `${p}.plan`),
    tables: arr(o, 'tables', p, decodeTable),
    hookahs: arr(o, 'hookahs', p, decodeHookah),
    staff_online: arr(o, 'staff_online', p, decodeOnline),
  };
}

export function decodeEnvelope(u: unknown): Envelope {
  const o = obj(u, 'envelope');
  return {
    v: num(o, 'v', 'envelope'), type: str(o, 'type', 'envelope'), seq: num(o, 'seq', 'envelope'),
    ts: str(o, 'ts', 'envelope'), venue_id: str(o, 'venue_id', 'envelope'),
    hall_id: nstr(o, 'hall_id', 'envelope'), data: o.data,
  };
}

export function decodeTokens(u: unknown, p = 'tokens'): Tokens {
  const o = obj(u, p);
  return { access_token: str(o, 'access_token', p), refresh_token: str(o, 'refresh_token', p), access_expires_at: str(o, 'access_expires_at', p) };
}

export type AuthResult = { tokens: Tokens; venue: Venue };
export type JoinResult = AuthResult & { halls: Hall[] };

export function decodeAuthResult(u: unknown): AuthResult {
  const o = obj(u, 'auth');
  return { tokens: decodeTokens(o.tokens), venue: decodeVenue(o.venue) };
}

export function decodeJoinResult(u: unknown): JoinResult {
  const o = obj(u, 'join');
  return { ...decodeAuthResult(u), halls: arr(o, 'halls', 'join', decodeHall) };
}

export function decodeRefresh(u: unknown): Tokens {
  return decodeTokens(obj(u, 'refresh').tokens);
}

export function decodeNothing(): void {
  return undefined;
}

export function decodeJoinCode(u: unknown): string {
  return str(obj(u, 'rotate'), 'join_code', 'rotate');
}
