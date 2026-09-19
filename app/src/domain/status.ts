import type { EventState, HookahState, TableStatus } from '../api/types';

export type StatusPlan = { warn_before_sec: number; grace_sec: number };
export type StatusEvent = { due_at: string; state: EventState };
export type StatusHookah = { state: HookahState; events: StatusEvent[] };

// tableStatus mirrors domain.TableStatus on the server, tech.md section 7.
// The first matching rule wins. Both implementations run against
// contracts/fixtures/status_cases.json.
export function tableStatus(hookah: StatusHookah | null, plan: StatusPlan, nowMs: number): TableStatus {
  if (!hookah || hookah.state === 'finished') return 'free';
  const due = hookah.events.filter((e) => e.state === 'due');
  if (due.some((e) => nowMs >= Date.parse(e.due_at) + plan.grace_sec * 1000)) return 'red';
  if (hookah.state === 'expired') return 'red';
  if (due.length > 0) return 'yellow';
  if (hookah.events.some((e) => e.state === 'pending' && Date.parse(e.due_at) - nowMs <= plan.warn_before_sec * 1000)) {
    return 'yellow';
  }
  return 'green';
}
