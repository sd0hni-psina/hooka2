import type { Hookah, ServiceEvent, Table, TableStatus } from '../api/types';
import { tableStatus } from '../domain/status';
import type { HallData } from './reducer';

export function selectTables(d: HallData): Table[] {
  return d.state?.tables ?? [];
}

export function hookahByTable(d: HallData): Map<string, Hookah> {
  const m = new Map<string, Hookah>();
  for (const h of d.state?.hookahs ?? []) m.set(h.table_id, h);
  return m;
}

export function hookahById(d: HallData, hookahId: string | null): Hookah | null {
  if (!hookahId) return null;
  return d.state?.hookahs.find((h) => h.id === hookahId) ?? null;
}

// statusesAt recomputes colors locally between messages, so a deadline that
// passes without an incoming event still turns the table yellow.
export function statusesAt(d: HallData, nowMs: number): Record<string, TableStatus> {
  const out: Record<string, TableStatus> = {};
  const byTable = hookahByTable(d);
  const plan = d.state?.plan;
  for (const t of selectTables(d)) {
    const h = byTable.get(t.id);
    out[t.id] = h && plan ? tableStatus(h, plan, nowMs) : 'free';
  }
  return out;
}

export type Task = {
  event: ServiceEvent;
  hookah: Hookah;
  table: Table;
  // overdueMs is positive once the deadline has passed.
  overdueMs: number;
};

// selectTasks lists what needs attention, most urgent first: fired tasks
// before the ones about to fire, tech.md section 12.
export function selectTasks(d: HallData, nowMs: number): Task[] {
  const tables = new Map(selectTables(d).map((t) => [t.id, t]));
  const warnMs = (d.state?.plan.warn_before_sec ?? 0) * 1000;
  const out: Task[] = [];
  for (const h of d.state?.hookahs ?? []) {
    const table = tables.get(h.table_id);
    if (!table) continue;
    for (const e of h.events) {
      const due = Date.parse(e.due_at);
      const fired = e.state === 'due';
      const warning = e.state === 'pending' && due - nowMs <= warnMs;
      if (fired || warning) out.push({ event: e, hookah: h, table, overdueMs: nowMs - due });
    }
  }
  return out.sort((a, b) => b.overdueMs - a.overdueMs);
}
