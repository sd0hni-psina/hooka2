import { countdownMs, formatDuration } from '../../domain/countdown';
import { __setLocalClockForTests, isClockSynced, serverNow, syncFromServer } from '../clock';

describe('clock', () => {
  const realNow = Date.parse('2026-09-19T18:20:00Z');

  it('shows the right counter on a phone whose clock runs 10 minutes fast', () => {
    const skew = 10 * 60_000;
    let local = realNow + skew;
    __setLocalClockForTests(() => local);
    expect(isClockSynced()).toBe(false);

    // Ping sent at local t, pong stamped by the server 100 ms later, received 200 ms after sending.
    const sent = local;
    local += 200;
    syncFromServer(new Date(realNow + 100).toISOString(), sent, local);

    expect(isClockSynced()).toBe(true);
    expect(Math.abs(serverNow() - (realNow + 200))).toBeLessThanOrEqual(1);

    const startedAt = '2026-09-19T18:00:00Z';
    expect(formatDuration(countdownMs(startedAt, 'up', serverNow()))).toBe('20:00');
  });

  it('ignores an unparsable server time', () => {
    __setLocalClockForTests(() => realNow);
    syncFromServer('not a date', realNow, realNow);
    expect(isClockSynced()).toBe(false);
    expect(serverNow()).toBe(realNow);
  });
});
