import type { HallState, Hookah, ServiceEvent, Table, TableStatus } from '../api/types';
import { formatShort } from '../domain/countdown';
import { tableStatus } from '../domain/status';
import type { HallData, Optimistic, PendingAction } from './reducer';

// memo1 remembers the last call. Every screen reads the same store, so one
// slot is enough to keep derived objects referentially stable and let
// React.memo do its job, tech.md section 19.15.
function memo1<A extends readonly unknown[], R>(fn: (...args: A) => R): (...args: A) => R {
  let last: { args: A; out: R } | null = null;
  return (...args: A): R => {
    if (last && last.args.length === args.length && last.args.every((a, i) => a === args[i])) return last.out;
    const out = fn(...args);
    last = { args, out };
    return out;
  };
}

export function selectTables(d: HallData): Table[] {
  return view(d)?.tables ?? [];
}

const buildView = memo1((state: HallState | null, pending: Record<string, Optimistic>): HallState | null => {
  if (!state) return null;
  const entries = Object.values(pending);
  if (entries.length === 0) return state;
  const byHookah = new Map<string, Optimistic[]>();
  for (const p of entries) {
    const list = byHookah.get(p.hookahId);
    if (list) list.push(p);
    else byHookah.set(p.hookahId, [p]);
  }
  return {
    ...state,
    hookahs: state.hookahs.map((h) => {
      const local = byHookah.get(h.id);
      if (!local) return h;
      return { ...h, events: h.events.map((e) => local.find((p) => p.event.id === e.id)?.event ?? e) };
    }),
  };
});

// view is the hall as the master sees it: server state with the local
// guesses already painted on.
export function view(d: HallData): HallState | null {
  return buildView(d.state, d.pending);
}

export const hookahByTable = memo1((d: HallData): Map<string, Hookah> => {
  const m = new Map<string, Hookah>();
  for (const h of view(d)?.hookahs ?? []) m.set(h.table_id, h);
  return m;
});

function sameRecord<T>(a: Record<string, T>, b: Record<string, T>): boolean {
  const ka = Object.keys(a);
  if (ka.length !== Object.keys(b).length) return false;
  return ka.every((k) => a[k] === b[k]);
}

let lastStatuses: Record<string, TableStatus> = {};

// statusesAt recomputes colors locally between messages, so a deadline that
// passes without an incoming event still turns the table yellow. The result
// keeps its identity while nothing changed: an idle hall costs no re-render.
export function statusesAt(d: HallData, nowMs: number): Record<string, TableStatus> {
  const out: Record<string, TableStatus> = {};
  const byTable = hookahByTable(d);
  const state = view(d);
  const plan = state?.plan;
  for (const t of state?.tables ?? []) {
    const h = byTable.get(t.id);
    out[t.id] = h && plan ? tableStatus(h, plan, nowMs) : 'free';
  }
  if (sameRecord(out, lastStatuses)) return lastStatuses;
  lastStatuses = out;
  return out;
}

let lastLabels: Record<string, string> = {};

// tableLabels is the line under a table number: who took the task, otherwise
// how long the hookah has been running. Minutes, so the map holds still
// between ticks.
export function tableLabels(d: HallData, nowMs: number): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [tableId, h] of hookahByTable(d)) {
    const claimed = h.events.find((e) => e.state === 'due' && e.claimed_by_name);
    out[tableId] = claimed?.claimed_by_name ?? formatShort(nowMs - Date.parse(h.started_at));
  }
  if (sameRecord(out, lastLabels)) return lastLabels;
  lastLabels = out;
  return out;
}

let lastProgress: Record<string, number> = {};

// progressAt is the share of the hookah resource already burned, 0..1. The
// ring around a table turns it into something readable in one glance.
export function progressAt(d: HallData, nowMs: number): Record<string, number> {
  const out: Record<string, number> = {};
  for (const [tableId, h] of hookahByTable(d)) {
    const start = Date.parse(h.started_at);
    const end = Date.parse(h.expires_at);
    const span = end - start;
    // Rounded to a percent: a smoother ring would redraw the map for nothing.
    out[tableId] = span > 0 ? Math.round(Math.min(1, Math.max(0, (nowMs - start) / span)) * 100) / 100 : 1;
  }
  if (sameRecord(out, lastProgress)) return lastProgress;
  lastProgress = out;
  return out;
}

export type Task = {
  event: ServiceEvent;
  hookah: Hookah;
  table: Table;
  // overdueMs is positive once the deadline has passed.
  overdueMs: number;
  pending: PendingAction | null;
};

// selectTasks lists what needs attention, most urgent first: fired tasks
// before the ones about to fire, tech.md section 12.
export function selectTasks(d: HallData, nowMs: number): Task[] {
  const state = view(d);
  const tables = new Map((state?.tables ?? []).map((t) => [t.id, t]));
  const warnMs = (state?.plan.warn_before_sec ?? 0) * 1000;
  const out: Task[] = [];
  for (const h of state?.hookahs ?? []) {
    const table = tables.get(h.table_id);
    if (!table) continue;
    for (const e of h.events) {
      const due = Date.parse(e.due_at);
      const fired = e.state === 'due';
      const warning = e.state === 'pending' && due - nowMs <= warnMs;
      if (fired || warning) {
        out.push({ event: e, hookah: h, table, overdueMs: nowMs - due, pending: d.pending[e.id]?.action ?? null });
      }
    }
  }
  return out.sort((a, b) => b.overdueMs - a.overdueMs);
}

// countTasks is what the tab badge needs, without building the list.
export function countTasks(d: HallData, nowMs: number): { due: number; soon: number } {
  let due = 0;
  let soon = 0;
  for (const task of selectTasks(d, nowMs)) {
    if (task.event.state === 'due') due += 1;
    else soon += 1;
  }
  return { due, soon };
}
