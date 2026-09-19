// Mirrors tech.md sections 11, 9 and 19.1. Field names stay snake_case.

export type Role = 'owner' | 'admin' | 'master';
export type Shape = 'rect' | 'circle';
export type TableStatus = 'free' | 'green' | 'yellow' | 'red';
export type HookahState = 'active' | 'expired' | 'finished';
export type EventKind = 'check' | 'shake' | 'replace' | 'expire' | 'custom';
export type EventState = 'pending' | 'due' | 'confirmed' | 'missed' | 'cancelled';
export type FinishReason = 'guest_left' | 'replaced' | 'mistake' | 'shift_end';
export type Platform = 'ios' | 'android';

export type PlanStep = {
  kind: EventKind;
  title: string;
  offset_sec: number;
  repeat_every_sec?: number;
};

export type Plan = {
  version: number;
  steps: PlanStep[];
  warn_before_sec: number;
  grace_sec: number;
  missed_after_sec: number;
  push_repeat_sec: number;
  push_max_attempts: number;
};

export type Table = {
  id: string; hall_id: string; label: string; shape: Shape;
  x: number; y: number; w: number; h: number; rotation: number; seats: number;
};

export type ServiceEvent = {
  id: string; hookah_id: string; seq: number; kind: EventKind; title: string;
  due_at: string; state: EventState;
  claimed_by: string | null; claimed_by_name: string | null; claimed_at: string | null;
  confirmed_by: string | null; confirmed_by_name: string | null; confirmed_at: string | null;
  confirmed_early: boolean;
};

export type Hookah = {
  id: string; table_id: string; hall_id: string; state: HookahState;
  started_at: string; expires_at: string; finished_at: string | null;
  started_by: string; started_by_name: string;
  status: TableStatus;
  events: ServiceEvent[];
};

export type Venue = {
  id: string; name: string; timezone: string;
  join_code: string | null;
};

export type Hall = { id: string; venue_id: string; name: string; sort_order: number };

export type Staff = {
  id: string; venue_id: string; display_name: string; role: Role;
  blocked: boolean; created_at: string; last_seen_at: string | null;
};

export type Tokens = { access_token: string; refresh_token: string; access_expires_at: string };

export type Me = { staff: Staff; venue: Venue; halls: Hall[] };

export type OnlineStaff = { id: string; display_name: string };

export type HallState = {
  server_time: string;
  hall: { id: string; name: string };
  plan: Plan;
  tables: Table[];
  hookahs: Hookah[];
  staff_online: OnlineStaff[];
};

export type Envelope = {
  v: number;
  type: string;
  seq: number;
  ts: string;
  venue_id: string;
  hall_id: string | null;
  data: unknown;
};
