import fixture from '../../../../contracts/fixtures/ws_events.json';
import type { Envelope } from '../../api/types';
import { __setLocalClockForTests } from '../../ws/clock';
import { parseMessage } from '../../ws/envelope';
import { useHallStore } from '../hall';

const byType = (type: string) => fixture.events.find((e) => e.type === type) as Envelope;
const snapshotEnv = byType('state.snapshot');
const hallId = snapshotEnv.hall_id as string;

const snapshot = (seq = 41) => parseMessage(JSON.stringify({ ...snapshotEnv, seq }));
const pong = () =>
  parseMessage(JSON.stringify({ v: 1, type: 'pong', seq: 41, ts: '2026-09-19T18:00:00Z', venue_id: 'v', hall_id: null, data: { server_time: '2026-09-19T18:00:00Z' } }));

let clock = 0;
const at = (iso: string) => {
  clock = Date.parse(iso);
};
const hall = () => useHallStore.getState();

// The master must never read an old hall without being told: the moment the
// link leaves online the store says its data is stale, tech.md section 19.17.
describe('data age on a lost connection', () => {
  beforeEach(() => {
    __setLocalClockForTests(() => clock);
    at('2026-09-19T18:00:00Z');
    hall().selectHall(hallId);
    hall().setConnection('connecting');
    hall().apply(snapshot());
    hall().setConnection('online');
  });

  it('is fresh while online after a snapshot', () => {
    expect(hall().stale).toBe(false);
    expect(hall().savedAt).toBeNull();
  });

  it('marks the hall stale when the socket drops, dated by the last frame', () => {
    at('2026-09-19T18:05:00Z');
    hall().apply(pong());
    at('2026-09-19T18:09:00Z');
    hall().setConnection('offline');

    expect(hall().stale).toBe(true);
    expect(hall().savedAt).toBe('2026-09-19T18:05:00.000Z');
  });

  it('keeps the hall stale after the socket reopens until a snapshot arrives', () => {
    hall().setConnection('offline');
    at('2026-09-19T18:03:00Z');
    hall().setConnection('connecting');
    hall().setConnection('online');
    expect(hall().stale).toBe(true);

    hall().apply(snapshot(50));
    expect(hall().stale).toBe(false);
    expect(hall().savedAt).toBeNull();
  });

  it('does not move the date of data that is already stale', () => {
    hall().setConnection('offline');
    const first = hall().savedAt;
    at('2026-09-19T18:20:00Z');
    hall().setConnection('connecting');
    expect(hall().savedAt).toBe(first);
  });

  it('marks nothing stale when there is no hall to be wrong about', () => {
    hall().selectHall(null);
    hall().setConnection('offline');
    expect(hall().stale).toBe(false);
  });
});
