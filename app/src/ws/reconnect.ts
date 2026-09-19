// Backoff of tech.md section 9: 1, 2, 4, 8, 16, then 30 seconds, with up to
// 30% jitter. attempt starts at 0.
const steps = [1000, 2000, 4000, 8000, 16000];
const cap = 30000;

export function reconnectDelay(attempt: number, random: () => number = Math.random): number {
  const base = steps[attempt] ?? cap;
  return Math.round(base * (1 + 0.3 * random()));
}
