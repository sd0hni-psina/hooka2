import type { EventKind, Plan } from '../api/types';

export type PlannedEvent = { seq: number; kind: EventKind; title: string; due_at: string };

const priority: Record<EventKind, number> = { expire: 5, replace: 4, shake: 3, check: 2, custom: 1 };

// materializePlan mirrors domain.MaterializePlan, tech.md section 7. The
// client uses it for the schedule preview before a plan is saved.
export function materializePlan(plan: Plan, startedAtIso: string): PlannedEvent[] {
  const expire = plan.steps.find((s) => s.kind === 'expire');
  if (!expire) return [];
  const start = Date.parse(startedAtIso);

  type Slot = { step: number; kind: EventKind; title: string };
  const slots = new Map<number, Slot>();
  const put = (offset: number, c: Slot) => {
    const cur = slots.get(offset);
    if (!cur || priority[c.kind] > priority[cur.kind] || (priority[c.kind] === priority[cur.kind] && c.step < cur.step)) {
      slots.set(offset, c);
    }
  };

  plan.steps.forEach((s, i) => {
    if (s.kind === 'expire') {
      put(s.offset_sec, { step: i, kind: s.kind, title: s.title });
      return;
    }
    for (let off = s.offset_sec; off < expire.offset_sec; ) {
      put(off, { step: i, kind: s.kind, title: s.title });
      if (!s.repeat_every_sec || s.repeat_every_sec <= 0) break;
      off += s.repeat_every_sec;
    }
  });

  return [...slots.keys()]
    .sort((a, b) => a - b)
    .map((off, i) => {
      const slot = slots.get(off) as Slot;
      return { seq: i + 1, kind: slot.kind, title: slot.title, due_at: new Date(start + off * 1000).toISOString() };
    });
}
