import type { Hookah, Table, TableStatus } from '../api/types';
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
