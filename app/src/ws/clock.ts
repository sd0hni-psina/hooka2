// The server is the only source of time, tech.md section 7. Every countdown
// and status computation reads serverNow(); Date.now() lives only here.

let offsetMs = 0;
let synced = false;

// localNow is injectable for tests.
let localNow: () => number = () => Date.now();

export function serverNow(): number {
  return localNow() + offsetMs;
}

export function clockOffset(): number {
  return offsetMs;
}

export function isClockSynced(): boolean {
  return synced;
}

// syncFromServer takes a server timestamp and the local send and receive
// moments of the exchange. Half the round trip approximates when the server
// stamped the reply.
export function syncFromServer(serverTimeIso: string, sentAtLocal: number, receivedAtLocal: number): void {
  const server = Date.parse(serverTimeIso);
  if (Number.isNaN(server)) return;
  const rtt = Math.max(0, receivedAtLocal - sentAtLocal);
  offsetMs = server - (sentAtLocal + rtt / 2);
  synced = true;
}

export function localClock(): number {
  return localNow();
}

export function __setLocalClockForTests(fn: () => number): void {
  localNow = fn;
  offsetMs = 0;
  synced = false;
}
