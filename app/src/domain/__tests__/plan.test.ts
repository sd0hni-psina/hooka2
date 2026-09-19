import fixture from '../../../../contracts/fixtures/plan_cases.json';
import { decodePlan } from '../../api/decode';
import { materializePlan } from '../plan';

describe('materializePlan against the shared fixture', () => {
  it.each(fixture.cases.map((c) => [c.name, c] as const))('%s', (_name, c) => {
    const got = materializePlan(decodePlan(c.plan), c.started_at);
    expect(got).toEqual(
      c.expected.map((e) => ({ ...e, due_at: new Date(Date.parse(e.due_at)).toISOString() })),
    );
  });
});
