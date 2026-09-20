import { countdownMs, formatDuration, formatShort } from '../countdown';

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

describe('formatShort', () => {
  it('reads minutes below an hour', () => {
    expect(formatShort(0)).toBe('0м');
    expect(formatShort(59_000)).toBe('0м');
    expect(formatShort(5 * 60_000 + 30_000)).toBe('5м');
  });

  it('reads hours and minutes above', () => {
    expect(formatShort(3_600_000)).toBe('1ч 0м');
    expect(formatShort(4_920_000)).toBe('1ч 22м');
  });

  it('never goes negative: a passed deadline reads zero', () => {
    expect(formatShort(-5000)).toBe('0м');
  });

  it('holds still inside one minute, so the map does not redraw', () => {
    expect(formatShort(120_000)).toBe(formatShort(179_000));
  });
});
