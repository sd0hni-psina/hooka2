import { countdownMs, formatDuration } from '../countdown';

describe('formatDuration', () => {
  it.each([
    [0, '0:00'],
    [999, '0:00'],
    [59_000, '0:59'],
    [60_000, '1:00'],
    [20 * 60_000 + 5_000, '20:05'],
    [3_600_000, '1:00:00'],
    [80 * 60_000, '1:20:00'],
    [-65_000, '-1:05'],
  ])('%d ms renders as %s', (ms, want) => {
    expect(formatDuration(ms)).toBe(want);
  });
});

describe('countdownMs', () => {
  const target = '2026-09-19T18:00:00Z';
  const at = (iso: string) => Date.parse(iso);
  it('counts up from the target', () => {
    expect(countdownMs(target, 'up', at('2026-09-19T18:20:00Z'))).toBe(20 * 60_000);
  });
  it('counts down to the target', () => {
    expect(countdownMs(target, 'down', at('2026-09-19T17:55:00Z'))).toBe(5 * 60_000);
  });
});
