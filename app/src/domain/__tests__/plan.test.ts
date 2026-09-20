import fixture from '../../../../contracts/fixtures/plan_cases.json';
import { decodePlan } from '../../api/decode';
import type { Plan, PlanStep } from '../../api/types';
import { materializePlan, previewPlan, validatePlan } from '../plan';

describe('materializePlan against the shared fixture', () => {
  it.each(fixture.cases.map((c) => [c.name, c] as const))('%s', (_name, c) => {
    const got = materializePlan(decodePlan(c.plan), c.started_at);
    expect(got).toEqual(
      c.expected.map((e) => ({ ...e, due_at: new Date(Date.parse(e.due_at)).toISOString() })),
    );
  });
});

describe('validatePlan at the boundaries', () => {
  const base = (): Plan => JSON.parse(JSON.stringify(fixture.cases[0]?.plan)) as Plan;

  it('accepts the default plan', () => {
    expect(validatePlan(base())).toBeNull();
  });

  it.each([
    ['two expire steps', (p: Plan) => p.steps.push({ kind: 'expire', title: 'x', offset_sec: 5000 }), 'steps[4].kind', 'multiple_expire'],
    ['a step after expire', (p: Plan) => ((p.steps[0] as PlanStep).offset_sec = 5000), 'steps[0].offset_sec', 'offset_after_expire'],
    ['a ten second step', (p: Plan) => ((p.steps[0] as PlanStep).offset_sec = 10), 'steps[0].offset_sec', 'offset_out_of_range'],
    ['a repeat under two minutes', (p: Plan) => ((p.steps[1] as PlanStep).repeat_every_sec = 119), 'steps[1].repeat_every_sec', 'repeat_out_of_range'],
    ['grace under thirty seconds', (p: Plan) => (p.grace_sec = 29), 'grace_sec', 'grace_out_of_range'],
    ['missed not after grace', (p: Plan) => (p.missed_after_sec = p.grace_sec), 'missed_after_sec', 'missed_not_after_grace'],
    ['warn over five minutes', (p: Plan) => (p.warn_before_sec = 301), 'warn_before_sec', 'warn_out_of_range'],
  ])('rejects %s', (_name, mutate, field, reason) => {
    const p = base();
    mutate(p);
    expect(validatePlan(p)).toEqual({ field, reason });
  });

  it('accepts a shake every twelve minutes', () => {
    const p = base();
    (p.steps[1] as PlanStep).offset_sec = 720;
    (p.steps[1] as PlanStep).repeat_every_sec = 720;
    expect(validatePlan(p)).toBeNull();
  });
});

describe('previewPlan', () => {
  it('shows the first ten tasks by time', () => {
    const p = JSON.parse(JSON.stringify(fixture.cases[0]?.plan)) as Plan;
    (p.steps[1] as PlanStep).offset_sec = 120;
    (p.steps[1] as PlanStep).repeat_every_sec = 120;
    const preview = previewPlan(p, '2026-09-19T18:00:00Z');
    expect(preview).toHaveLength(10);
    expect(preview[0]?.due_at).toBe('2026-09-19T18:02:00.000Z');
    expect(preview.map((e) => Date.parse(e.due_at))).toEqual([...preview.map((e) => Date.parse(e.due_at))].sort((a, b) => a - b));
  });
});
