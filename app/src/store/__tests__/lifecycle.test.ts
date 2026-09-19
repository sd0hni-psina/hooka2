import fixture from '../../../../contracts/fixtures/ws_events.json';
import type { Envelope, HallState } from '../../api/types';
import { parseMessage, type ParsedMessage } from '../../ws/envelope';
import { emptyHall, reduce, type HallData } from '../reducer';
import { selectTasks, statusesAt } from '../selectors';

const byType = (type: string) => fixture.events.find((e) => e.type === type) as Envelope;
const snapshotFixture = byType('state.snapshot');
const hallId = snapshotFixture.hall_id as string;
const tableId = 'a1b2c3d4-0000-4000-8000-000000000015';

function message(type: string, seq: number, patch: (data: Record<string, unknown>) => void = () => undefined, hall = hallId): ParsedMessage {
  const env = byType(type);
  const data = structuredClone(env.data) as Record<string, unknown>;
  patch(data);
  return parseMessage(JSON.stringify({ ...env, seq, hall_id: hall, data }));
}

function play(msgs: ParsedMessage[], start: HallData = { ...emptyHall, hallId }): HallData {
  return msgs.reduce((data, m) => reduce(data, m).data, start);
}

const snapshot = (seq = 41) => message('state.snapshot', seq);
const state = (d: HallData) => d.state as HallState;

describe('hookah lifecycle in the store', () => {
  it('starts from the snapshot with one hookah on table 15', () => {
    const d = play([snapshot()]);
    expect(state(d).hookahs).toHaveLength(1);
    expect(state(d).hookahs[0]?.table_id).toBe(tableId);
  });

  it('adds a started hookah and keeps one hookah per table', () => {
    const d = play([snapshot(), message('hookah.started', 42, (data) => {
      const h = data.hookah as Record<string, unknown>;
      h.id = 'new-hookah';
      h.table_id = tableId;
    })]);
    expect(state(d).hookahs.map((h) => h.id)).toEqual(['new-hookah']);
  });

  it('applies the same started event twice with the same result', () => {
    const once = play([snapshot(), message('hookah.started', 42)]);
    const twice = play([snapshot(), message('hookah.started', 42), message('hookah.started', 43)]);
    expect(state(twice).hookahs).toEqual(state(once).hookahs);
  });

  it('moves a hookah to another table of the same hall', () => {
    const otherTable = 'a1b2c3d4-0000-4000-8000-000000000022';
    const d = play([snapshot(), message('hookah.transferred', 42, (data) => {
      (data.hookah as Record<string, unknown>).table_id = otherTable;
    })]);
    expect(state(d).hookahs).toHaveLength(1);
    expect(state(d).hookahs[0]?.table_id).toBe(otherTable);
  });

  it('drops a hookah that moved to another hall, even though the event names that hall', () => {
    const d = play([snapshot(), message('hookah.transferred', 42, (data) => {
      const h = data.hookah as Record<string, unknown>;
      h.hall_id = 'other-hall';
      h.table_id = 'other-table';
    }, 'other-hall')]);
    expect(state(d).hookahs).toHaveLength(0);
    expect(d.seq).toBe(42);
  });

  it('marks a due task and turns the table yellow, then red when expired', () => {
    const due = play([snapshot(), message('service.due', 42)]);
    const event = state(due).hookahs[0]?.events.find((e) => e.state === 'due');
    expect(event?.kind).toBe('shake');
    expect(state(due).hookahs[0]?.status).toBe('yellow');

    const expired = play([snapshot(), message('service.due', 42), message('hookah.expired', 43)]);
    expect(state(expired).hookahs[0]?.state).toBe('expired');
    expect(state(expired).hookahs[0]?.status).toBe('red');
  });

  it('closes a missed task without dropping the hookah', () => {
    const d = play([snapshot(), message('service.due', 42), message('service.missed', 43)]);
    const events = state(d).hookahs[0]?.events ?? [];
    expect(events.find((e) => e.id === 'e0000000-0000-4000-8000-000000000002')?.state).toBe('missed');
    expect(state(d).hookahs).toHaveLength(1);
  });

  it('frees the table when the hookah is finished', () => {
    const d = play([snapshot(), message('hookah.finished', 42)]);
    expect(state(d).hookahs).toHaveLength(0);
    const now = Date.parse('2026-09-19T18:05:00Z');
    expect(statusesAt(d, now)[tableId]).toBe('free');
  });

  it('ignores events of another hall but keeps seq', () => {
    const d = play([snapshot(), message('hookah.expired', 42, () => undefined, 'other-hall')]);
    expect(state(d).hookahs[0]?.state).toBe('active');
    expect(d.seq).toBe(42);
  });

  it('asks for a snapshot when an event of the lifecycle skips a seq', () => {
    const { resync } = reduce(play([snapshot()]), message('hookah.finished', 44));
    expect(resync).toBe(true);
  });
});

describe('task strip', () => {
  const now = Date.parse('2026-09-19T18:05:00Z');

  it('lists fired tasks before the ones about to fire', () => {
    const d = play([snapshot(), message('service.due', 42)]);
    const tasks = selectTasks(d, now);
    expect(tasks).toHaveLength(1);
    expect(tasks[0]?.event.kind).toBe('shake');
    expect(tasks[0]?.table.label).toBe('15');
    expect(tasks[0]?.overdueMs).toBeGreaterThan(0);
  });

  it('is empty while every task is still far away', () => {
    const calm = message('state.snapshot', 41, (data) => {
      const hookahs = data.hookahs as Record<string, unknown>[];
      const events = hookahs[0]?.events as Record<string, unknown>[];
      const event = events[0] as Record<string, unknown>;
      event.state = 'pending';
      event.due_at = '2026-09-19T18:30:00Z';
    });
    expect(selectTasks(play([calm]), now)).toHaveLength(0);
  });
});
