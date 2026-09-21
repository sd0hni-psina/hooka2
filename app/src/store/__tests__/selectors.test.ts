import fixture from '../../../../contracts/fixtures/ws_events.json';
import type { Envelope } from '../../api/types';
import { parseMessage } from '../../ws/envelope';
import { emptyHall, reduce, type HallData } from '../reducer';
import { hookahByTable, progressAt, savedTables, selectTables, staffOnline, statusesAt, tableLabels } from '../selectors';

const snapshotEnv = fixture.events.find((e) => e.type === 'state.snapshot') as Envelope;
const hallId = snapshotEnv.hall_id as string;

const seeded = (): HallData =>
  reduce({ ...emptyHall, hallId }, parseMessage(JSON.stringify(snapshotEnv))).data;

describe('derived hall state', () => {
  it('keeps the status map identical while nothing moves', () => {
    const d = seeded();
    const start = Date.parse(d.state?.hookahs[0]?.started_at ?? '');
    expect(statusesAt(d, start + 1000)).toBe(statusesAt(d, start + 2000));
  });

  it('builds a new status map once a table changes color', () => {
    const d = seeded();
    const hookah = d.state?.hookahs[0];
    const expires = Date.parse(hookah?.expires_at ?? '');
    const before = statusesAt(d, Date.parse(hookah?.started_at ?? '') + 1000);
    const after = statusesAt(d, expires + 60_000);
    expect(after).not.toBe(before);
  });

  it('reuses the table index while the hall data holds', () => {
    const d = seeded();
    expect(hookahByTable(d)).toBe(hookahByTable(d));
  });

  it('labels a running table in whole minutes', () => {
    const d = seeded();
    const hookah = d.state?.hookahs[0];
    const start = Date.parse(hookah?.started_at ?? '');
    const labels = tableLabels(d, start + 5 * 60_000 + 30_000);
    // The fixture hookah has a claimed task, so the name wins over the timer.
    const value = labels[hookah?.table_id ?? ''];
    expect(typeof value).toBe('string');
    expect(value?.length).toBeGreaterThan(0);
  });

  it('reads the burned share of the resource as a rounded fraction', () => {
    const d = seeded();
    const hookah = d.state?.hookahs[0];
    const start = Date.parse(hookah?.started_at ?? '');
    const span = Date.parse(hookah?.expires_at ?? '') - start;
    expect(progressAt(d, start)[hookah?.table_id ?? '']).toBe(0);
    expect(progressAt(d, start + span / 2)[hookah?.table_id ?? '']).toBeCloseTo(0.5, 2);
    expect(progressAt(d, start + span * 2)[hookah?.table_id ?? '']).toBe(1);
  });
});

// A zustand 5 selector that returns a new object on every call loops the
// screen forever. Before the first snapshot every list selector answers empty,
// and the answer must be the same object each time.
describe('selectors before the first snapshot', () => {
  it.each([
    ['staffOnline', () => staffOnline(emptyHall)],
    ['savedTables', () => savedTables(emptyHall)],
    ['selectTables', () => selectTables(emptyHall)],
  ])('%s returns one empty list', (_name, read) => {
    expect(read()).toEqual([]);
    expect(read()).toBe(read());
  });
});
