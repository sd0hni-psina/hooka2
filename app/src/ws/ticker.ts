import { serverNow } from './clock';

// One ticker for the whole app, tech.md section 19.15. Before it every
// countdown owned a setInterval and the hall map redrew 40 tables a second.
export type Granularity = 'second' | 'minute';

type Listener = (nowMs: number) => void;

const listeners: Record<Granularity, Set<Listener>> = { second: new Set(), minute: new Set() };

let timer: unknown = null;
let active = true;
let lastMinute = -1;

// Timers are injectable so tests drive the ticker without real time.
let schedule: (fn: () => void, ms: number) => unknown = (fn, ms) => setTimeout(fn, ms);
let cancel: (t: unknown) => void = (t) => clearTimeout(t as ReturnType<typeof setTimeout>);

function emit(now: number): void {
  for (const fn of [...listeners.second]) fn(now);
  const minute = Math.floor(now / 60_000);
  if (minute !== lastMinute) {
    lastMinute = minute;
    for (const fn of [...listeners.minute]) fn(now);
  }
}

function stop(): void {
  if (timer !== null) cancel(timer);
  timer = null;
}

// run aligns the next wake-up with the server second boundary, so a counter
// flips at the same moment on every phone in the hall.
function run(): void {
  stop();
  if (!active || listeners.second.size + listeners.minute.size === 0) return;
  const now = serverNow();
  const delay = 1000 - (((now % 1000) + 1000) % 1000);
  timer = schedule(() => {
    timer = null;
    emit(serverNow());
    run();
  }, delay);
}

export function subscribeTick(granularity: Granularity, fn: Listener): () => void {
  listeners[granularity].add(fn);
  run();
  return () => {
    listeners[granularity].delete(fn);
    if (listeners.second.size + listeners.minute.size === 0) stop();
  };
}

// setTickerActive follows AppState: a backgrounded app burns no timer, and
// the return fires one immediate tick so nothing shows a stale second.
export function setTickerActive(next: boolean): void {
  if (active === next) return;
  active = next;
  if (!active) {
    stop();
    return;
  }
  lastMinute = -1;
  emit(serverNow());
  run();
}

export function isTickerActive(): boolean {
  return active;
}

export function __setTickerTimersForTests(deps: {
  schedule: (fn: () => void, ms: number) => unknown;
  cancel: (t: unknown) => void;
}): void {
  stop();
  listeners.second.clear();
  listeners.minute.clear();
  schedule = deps.schedule;
  cancel = deps.cancel;
  active = true;
  lastMinute = -1;
}
