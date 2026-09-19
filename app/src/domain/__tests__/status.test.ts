import fixture from '../../../../contracts/fixtures/status_cases.json';
import type { HookahState, EventState } from '../../api/types';
import { tableStatus } from '../status';

type Case = {
  name: string;
  hookah: { state: string; events: { due_at: string; state: string }[] } | null;
  expected: string;
};

describe('tableStatus against the shared fixture', () => {
  const now = Date.parse(fixture.now);
  it.each((fixture.cases as Case[]).map((c) => [c.name, c] as const))('%s', (_name, c) => {
    const hookah = c.hookah
      ? {
          state: c.hookah.state as HookahState,
          events: c.hookah.events.map((e) => ({ due_at: e.due_at, state: e.state as EventState })),
        }
      : null;
    expect(tableStatus(hookah, fixture.plan, now)).toBe(c.expected);
  });
});
