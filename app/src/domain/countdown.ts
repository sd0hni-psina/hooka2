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

// formatShort renders a running hookah in the shortest honest form: minutes
// below an hour, hours and minutes above. The hall map uses it, so it must
// change no more than once a minute.
export function formatShort(totalMs: number): string {
  const total = Math.max(0, Math.floor(totalMs / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  if (h > 0) return `${h}ч ${m}м`;
  return `${m}м`;
}
