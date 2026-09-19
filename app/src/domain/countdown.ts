// formatDuration renders whole seconds as M:SS below an hour and H:MM:SS above.
export function formatDuration(totalMs: number): string {
  const sign = totalMs < 0 ? '-' : '';
  const total = Math.floor(Math.abs(totalMs) / 1000);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const ss = String(s).padStart(2, '0');
  if (h > 0) return `${sign}${h}:${String(m).padStart(2, '0')}:${ss}`;
  return `${sign}${m}:${ss}`;
}

export type CountdownMode = 'up' | 'down';

// countdownMs: 'up' measures time since target, 'down' time left until target.
// nowMs comes from serverNow(), or from a frozen moment for stopped timers.
export function countdownMs(targetIso: string, mode: CountdownMode, nowMs: number): number {
  const target = Date.parse(targetIso);
  return mode === 'up' ? nowMs - target : target - nowMs;
}
