import type { EventKind, Plan, PlanStep } from '../api/types';

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

export type PlanProblem = { field: string; reason: string };

// validatePlan mirrors domain.ValidatePlan on the server, tech.md section 7,
// so the editor can refuse a plan before the request.
export function validatePlan(plan: Plan): PlanProblem | null {
  const fail = (field: string, reason: string): PlanProblem => ({ field, reason });
  if (plan.version !== 1) return fail('version', 'unsupported_version');
  if (plan.steps.length < 1 || plan.steps.length > 12) return fail('steps', 'steps_count_out_of_range');

  let expireIndex = -1;
  for (let i = 0; i < plan.steps.length; i++) {
    const s = plan.steps[i] as PlanStep;
    const field = `steps[${i}]`;
    if (!kinds.includes(s.kind)) return fail(`${field}.kind`, 'unknown_kind');
    const title = s.title.trim();
    if (title.length === 0 || [...title].length > 120) return fail(`${field}.title`, 'title_length_out_of_range');
    if (s.offset_sec < 60 || s.offset_sec > 86400) return fail(`${field}.offset_sec`, 'offset_out_of_range');
    if (s.repeat_every_sec !== undefined && (s.repeat_every_sec < 120 || s.repeat_every_sec > 86400)) {
      return fail(`${field}.repeat_every_sec`, 'repeat_out_of_range');
    }
    if (s.kind === 'expire') {
      if (expireIndex >= 0) return fail(`${field}.kind`, 'multiple_expire');
      expireIndex = i;
      if (s.repeat_every_sec !== undefined) return fail(`${field}.repeat_every_sec`, 'expire_repeats');
    }
  }
  if (expireIndex < 0) return fail('steps', 'missing_expire');

  const expire = plan.steps[expireIndex] as PlanStep;
  for (let i = 0; i < plan.steps.length; i++) {
    if (i !== expireIndex && (plan.steps[i] as PlanStep).offset_sec >= expire.offset_sec) {
      return fail(`steps[${i}].offset_sec`, 'offset_after_expire');
    }
  }
  if (plan.grace_sec < 30 || plan.grace_sec > 900) return fail('grace_sec', 'grace_out_of_range');
  if (plan.warn_before_sec < 0 || plan.warn_before_sec > 300) return fail('warn_before_sec', 'warn_out_of_range');
  if (plan.missed_after_sec <= plan.grace_sec) return fail('missed_after_sec', 'missed_not_after_grace');
  return null;
}

const kinds: EventKind[] = ['check', 'shake', 'replace', 'expire', 'custom'];

// previewPlan shows the first tasks of a plan before it is saved,
// tech.md slice 5.
export function previewPlan(plan: Plan, startedAtIso: string, limit = 10): PlannedEvent[] {
  return materializePlan(plan, startedAtIso).slice(0, limit);
}
