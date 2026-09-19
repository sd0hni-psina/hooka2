import { reconnectDelay } from '../reconnect';

describe('reconnectDelay', () => {
  it('follows 1, 2, 4, 8, 16 then 30 seconds without jitter', () => {
    const delays = [0, 1, 2, 3, 4, 5, 6, 20].map((a) => reconnectDelay(a, () => 0));
    expect(delays).toEqual([1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000]);
  });

  it('adds at most 30% jitter', () => {
    expect(reconnectDelay(0, () => 0.999999)).toBeLessThanOrEqual(1300);
    expect(reconnectDelay(5, () => 0.999999)).toBeLessThanOrEqual(39000);
    expect(reconnectDelay(5, () => 0.5)).toBe(34500);
  });
});
