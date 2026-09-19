import fixture from '../../../../contracts/fixtures/ws_events.json';
import type { Envelope } from '../../api/types';
import { parseMessage, type ParsedMessage } from '../../ws/envelope';
import { emptyHall, reduce, type HallData } from '../reducer';

const snapshotFixture = fixture.events.find((e) => e.type === 'state.snapshot') as Envelope;
const hallId = snapshotFixture.hall_id as string;

function snapshot(seq: number, hall = hallId): ParsedMessage {
  return parseMessage(JSON.stringify({ ...snapshotFixture, seq, hall_id: hall }));
}

// A type this build does not know: the reducer must still track its seq.
function event(seq: number, hall: string | null = hallId, type = 'venue.pinged'): ParsedMessage {
  const env: Envelope = { v: 1, type, seq, ts: '2026-09-19T18:05:00Z', venue_id: snapshotFixture.venue_id, hall_id: hall, data: {} };
  return parseMessage(JSON.stringify(env));
}

function run(start: HallData, msgs: ParsedMessage[]): { data: HallData; resyncs: number[] } {
  let data = start;
  const resyncs: number[] = [];
  msgs.forEach((m, i) => {
    const r = reduce(data, m);
    data = r.data;
    if (r.resync) resyncs.push(i);
  });
  return { data, resyncs };
}

describe('hall reducer', () => {
  const selected: HallData = { ...emptyHall, hallId };

  it('replaces the store with a snapshot and takes its seq as base', () => {
    const { data, resyncs } = run(selected, [snapshot(41)]);
    expect(data.seq).toBe(41);
    expect(data.state?.hall.name).toBe('Второй зал');
    expect(resyncs).toEqual([]);
  });

  it('advances seq on consecutive events of any hall', () => {
    const { data, resyncs } = run(selected, [snapshot(41), event(42), event(43, 'other-hall'), event(44, null)]);
    expect(data.seq).toBe(44);
    expect(resyncs).toEqual([]);
  });

  it('asks for a resync on a seq gap and keeps the old base', () => {
    const { data, resyncs } = run(selected, [snapshot(41), event(43)]);
    expect(resyncs).toEqual([1]);
    expect(data.seq).toBe(41);
  });

  it('treats a lower seq after a server restart as a gap', () => {
    const { resyncs } = run(selected, [snapshot(41), event(1)]);
    expect(resyncs).toEqual([1]);
  });

  it('recovers from a gap once the new snapshot arrives', () => {
    const { data, resyncs } = run(selected, [snapshot(41), event(45), snapshot(45), event(46)]);
    expect(resyncs).toEqual([1]);
    expect(data.seq).toBe(46);
  });

  it('ignores events before the first snapshot', () => {
    const { data, resyncs } = run(selected, [event(7)]);
    expect(data.seq).toBeNull();
    expect(resyncs).toEqual([]);
  });

  it('ignores a late snapshot of the previously selected hall', () => {
    const { data } = run(selected, [snapshot(41, 'previous-hall')]);
    expect(data.state).toBeNull();
  });
});

const layoutFixture = fixture.events.find((e) => e.type === 'layout.updated') as Envelope;

describe('layout.updated', () => {
  const selected: HallData = { ...emptyHall, hallId };

  function layout(seq: number, hall: string | null = hallId): ParsedMessage {
    return parseMessage(JSON.stringify({ ...layoutFixture, seq, hall_id: hall }));
  }

  it('replaces the tables of the hall', () => {
    const { data } = run(selected, [snapshot(41), layout(42)]);
    expect(data.state?.tables.map((t) => t.label)).toEqual(['15', '16']);
    expect(data.state?.hookahs).toHaveLength(1);
    expect(data.seq).toBe(42);
  });

  it('ignores a layout of another hall but keeps seq', () => {
    const { data, resyncs } = run(selected, [snapshot(41), layout(42, 'other-hall')]);
    expect(data.state?.tables.map((t) => t.label)).toEqual(['15']);
    expect(data.seq).toBe(42);
    expect(resyncs).toEqual([]);
  });
});
