import { __setLocalClockForTests, serverNow } from '../clock';
import { __setTickerTimersForTests, setTickerActive, subscribeTick } from '../ticker';

type Scheduled = { fn: () => void; ms: number };

describe('shared ticker', () => {
  let queue: Scheduled[];
  let clock: number;

  const runNext = (advanceBy?: number) => {
    const next = queue.shift();
    if (!next) throw new Error('nothing scheduled');
    clock += advanceBy ?? next.ms;
    next.fn();
  };

  beforeEach(() => {
    clock = 10_000;
    queue = [];
    __setLocalClockForTests(() => clock);
    __setTickerTimersForTests({
      schedule: (fn, ms) => {
        queue.push({ fn, ms });
        return queue.length;
      },
      cancel: () => {
        queue.length = 0;
      },
    });
  });

  it('wakes on the next server second boundary', () => {
    clock = 10_400;
    const seen: number[] = [];
    subscribeTick('second', (n) => seen.push(n));
    expect(queue[0].ms).toBe(600);
    runNext();
    expect(seen).toEqual([11_000]);
  });

  it('serves every second subscriber from one timer', () => {
    const a: number[] = [];
    const b: number[] = [];
    subscribeTick('second', (n) => a.push(n));
    subscribeTick('second', (n) => b.push(n));
    expect(queue).toHaveLength(1);
    runNext();
    expect(a).toEqual([11_000]);
    expect(b).toEqual([11_000]);
  });

  it('calls a minute subscriber only when the minute changes', () => {
    clock = 59_000;
    const minutes: number[] = [];
    subscribeTick('minute', (n) => minutes.push(n));
    runNext();
    runNext();
    runNext();
    expect(minutes).toEqual([60_000]);
  });

  it('stops the timer when the last subscriber leaves', () => {
    const off = subscribeTick('second', () => undefined);
    expect(queue).toHaveLength(1);
    off();
    expect(queue).toHaveLength(0);
  });

  it('pauses in background and fires one tick on return', () => {
    const seen: number[] = [];
    subscribeTick('second', (n) => seen.push(n));
    setTickerActive(false);
    expect(queue).toHaveLength(0);
    clock = 25_000;
    setTickerActive(true);
    expect(seen).toEqual([25_000]);
    expect(serverNow()).toBe(25_000);
  });
});
