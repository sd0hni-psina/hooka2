import { connectionLine } from '../connectionLine';

const keys = (l: ReturnType<typeof connectionLine>) => l.map((x) => x.key);
const base = { connection: 'online' as const, stale: false, time: null, queued: 0 };

// The line answers two questions: is the hall on screen fresh, and did my
// last tap reach the server.
describe('connection line', () => {
  it('says live only when the connection is up and the data is fresh', () => {
    expect(keys(connectionLine(base))).toEqual(['live']);
  });

  it('does not claim a live link while the reopened socket has no snapshot yet', () => {
    const line = connectionLine({ ...base, stale: true, time: '18:05' });
    expect(keys(line)).toEqual(['stale']);
    expect(line[0]?.title).toContain('18:05');
  });

  it('shows the loss and the age of the data when the link is down', () => {
    const line = connectionLine({ ...base, connection: 'offline', stale: true, time: '18:05' });
    expect(keys(line)).toEqual(['offline', 'stale']);
    expect(line[1]?.title).toContain('18:05');
  });

  it('shows only the age while reconnecting with old data', () => {
    expect(keys(connectionLine({ ...base, connection: 'connecting', stale: true, time: '18:05' }))).toEqual(['stale']);
  });

  it('shows connecting when there is no hall on screen yet', () => {
    expect(keys(connectionLine({ ...base, connection: 'connecting' }))).toEqual(['connecting']);
    expect(keys(connectionLine({ ...base, connection: 'idle' }))).toEqual(['connecting']);
  });

  it('shows the taps that wait for the network, whatever the link says', () => {
    const line = connectionLine({ ...base, connection: 'offline', stale: true, time: '18:05', queued: 2 });
    expect(keys(line)).toEqual(['offline', 'stale', 'queued']);
    expect(line[2]?.title).toContain('2');
    expect(keys(connectionLine({ ...base, queued: 1 }))).toEqual(['live', 'queued']);
  });

  it('shows offline without an age when the data has no date', () => {
    expect(keys(connectionLine({ ...base, connection: 'offline' }))).toEqual(['offline']);
  });
});
