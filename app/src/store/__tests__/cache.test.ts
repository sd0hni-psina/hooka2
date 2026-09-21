import fixture from '../../../../contracts/fixtures/ws_events.json';
import { decodeHallState } from '../../api/decode';
import type { Envelope } from '../../api/types';
import { __setSnapshotStorageForTests, loadSnapshot, saveSnapshot } from '../cache';

const snapshotEnv = fixture.events.find((e) => e.type === 'state.snapshot') as Envelope;
const hallState = decodeHallState(snapshotEnv.data);
const hallId = snapshotEnv.hall_id as string;
const now = 1_700_000_000_000;

function memory() {
  const box: Record<string, string> = {};
  __setSnapshotStorageForTests({
    getItem: async (k) => box[k] ?? null,
    setItem: async (k, v) => {
      box[k] = v;
    },
    removeItem: async (k) => {
      delete box[k];
    },
  });
  return box;
}

describe('snapshot cache', () => {
  it('returns the hall it stored', async () => {
    memory();
    await saveSnapshot(hallId, hallState, now);
    const cached = await loadSnapshot(hallId, now + 1000);
    expect(cached?.state.tables).toEqual(hallState.tables);
    expect(cached?.savedAt).toBe(new Date(now).toISOString());
  });

  it('writes no more than once every five seconds', async () => {
    const box = memory();
    await saveSnapshot(hallId, hallState, now);
    await saveSnapshot(hallId, { ...hallState, tables: [] }, now + 2000);
    const cached = await loadSnapshot(hallId, now + 3000);
    expect(cached?.state.tables).toHaveLength(hallState.tables.length);
    expect(Object.keys(box)).toHaveLength(1);
  });

  it('ignores a snapshot older than twelve hours', async () => {
    memory();
    await saveSnapshot(hallId, hallState, now);
    expect(await loadSnapshot(hallId, now + 13 * 60 * 60 * 1000)).toBeNull();
  });

  it('drops a cache this build cannot read', async () => {
    const box = memory();
    box[`hall.snapshot.v1.${hallId}`] = '{"saved_at":"nonsense"}';
    expect(await loadSnapshot(hallId, now)).toBeNull();
    expect(box[`hall.snapshot.v1.${hallId}`]).toBeUndefined();
  });

  it('has nothing to give for a hall it never saw', async () => {
    memory();
    expect(await loadSnapshot('unknown-hall', now)).toBeNull();
  });
});
