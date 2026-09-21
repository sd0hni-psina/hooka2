import { parseMessage, clientMessage, type ParsedMessage } from './envelope';
import { localClock, syncFromServer } from './clock';
import { reconnectDelay } from './reconnect';

const PING_EVERY_MS = 20_000;
// The server answers every ping, so a live socket cannot stay quiet for the
// span of two pings and a margin, tech.md section 19.17.
const SILENCE_LIMIT_MS = 45_000;
// After the app returns from background the socket must speak within this
// span or it is replaced.
const RESUME_PROBE_MS = 3_000;
// The server closes with this code when the session is revoked or blocked.
const CLOSE_POLICY = 1008;

export type WsStatus = 'connecting' | 'online' | 'offline';

type SocketLike = {
  onopen: (() => void) | null;
  onclose: ((e: { code?: number }) => void) | null;
  onerror: (() => void) | null;
  onmessage: ((e: { data: unknown }) => void) | null;
  send: (data: string) => void;
  close: () => void;
};

export type WsDeps = {
  // url resolves the address with a fresh access token, or null when signed out.
  url: () => Promise<string | null>;
  onMessage: (msg: ParsedMessage) => boolean | void;
  onStatus: (s: WsStatus) => void;
  // onRevoked fires when the server closed the socket with 1008. The client
  // stays closed: the app checks the session and decides.
  onRevoked?: () => void;
  socket?: (url: string) => SocketLike;
  setTimer?: (fn: () => void, ms: number) => unknown;
  clearTimer?: (t: unknown) => void;
  random?: () => number;
};

// WsClient keeps one connection alive: reconnects with backoff, pings every
// 20 s, drops a socket that went silent, subscribes after each open and
// whenever onMessage asks to resync.
export class WsClient {
  private sock: SocketLike | null = null;
  private hallId: string | null = null;
  private attempt = 0;
  private stopped = true;
  private retryTimer: unknown = null;
  private pingTimer: unknown = null;
  private pingSentAt: number | null = null;
  private subscribeSentAt: number | null = null;
  private watchdogTimer: unknown = null;
  private probeTimer: unknown = null;
  private lastFrameAt = 0;
  // frames counts every frame received, so a probe can tell that the socket
  // spoke without reading the clock.
  private frames = 0;
  private opening = false;
  // revoked keeps the client quiet after a 1008 until the app returns.
  private revoked = false;
  private readonly d: Required<WsDeps>;

  constructor(deps: WsDeps) {
    this.d = {
      onRevoked: () => undefined,
      socket: (url) => new WebSocket(url) as unknown as SocketLike,
      setTimer: (fn, ms) => setTimeout(fn, ms),
      clearTimer: (t) => clearTimeout(t as ReturnType<typeof setTimeout>),
      random: Math.random,
      ...deps,
    };
  }

  start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    this.revoked = false;
    void this.open();
  }

  stop(): void {
    this.stopped = true;
    this.clearTimers();
    const s = this.sock;
    this.sock = null;
    s?.close();
    this.d.onStatus('offline');
  }

  // resume brings the socket back at once instead of waiting out the
  // backoff. The app returning from background calls it: a master who
  // unlocks the phone must not read a hall from two minutes ago,
  // tech.md section 19.15.
  //
  // A socket that looks alive after a long sleep may be dead without a FIN, so
  // it gets a subscribe and a few seconds to answer before it is replaced.
  resume(): void {
    if (this.stopped) return;
    this.revoked = false;
    this.attempt = 0;
    if (this.retryTimer !== null) {
      this.d.clearTimer(this.retryTimer);
      this.retryTimer = null;
    }
    if (this.sock) {
      // The screen may be minutes old: say so until the snapshot lands.
      this.d.onStatus('connecting');
      this.sendSubscribe();
      this.armProbe();
      return;
    }
    void this.open();
  }

  subscribe(hallId: string | null): void {
    this.hallId = hallId;
    this.sendSubscribe();
  }

  private async open(): Promise<void> {
    // A second open while the url is still resolving would leave two live sockets.
    if (this.stopped || this.opening) return;
    this.opening = true;
    let url: string | null = null;
    try {
      this.d.onStatus('connecting');
      try {
        url = await this.d.url();
      } catch {
        url = null;
      }
      if (this.stopped) return;
      if (!url) {
        this.scheduleReconnect();
        return;
      }
      const s = this.d.socket(url);
      this.sock = s;
      // A socket that never opens is as silent as one that died: the clock
      // starts at creation.
      this.lastFrameAt = localClock();
      this.armWatchdog();
      s.onopen = () => {
        if (this.sock !== s) return;
        this.d.onStatus('online');
        this.sendSubscribe();
        this.ping();
      };
      s.onmessage = (e) => {
        if (this.sock !== s || typeof e.data !== 'string') return;
        this.frames += 1;
        this.lastFrameAt = localClock();
        this.handle(e.data);
      };
      s.onerror = () => s.close();
      s.onclose = (e) => {
        if (this.sock !== s) return;
        this.sock = null;
        this.clearTimers();
        this.d.onStatus('offline');
        if (e?.code === CLOSE_POLICY) {
          this.revoked = true;
          this.d.onRevoked();
          return;
        }
        this.scheduleReconnect();
      };
    } finally {
      this.opening = false;
    }
  }

  private handle(raw: string): void {
    let msg: ParsedMessage;
    try {
      msg = parseMessage(raw);
    } catch {
      // A message this build cannot read leaves a hole in seq, so rebase on a
      // fresh snapshot instead of drifting.
      this.sendSubscribe();
      return;
    }
    const received = localClock();
    if (msg.kind === 'pong' && this.pingSentAt !== null) {
      syncFromServer(msg.serverTime, this.pingSentAt, received);
      this.pingSentAt = null;
    }
    if (msg.kind === 'snapshot') {
      // The backoff restarts on a snapshot, not on open: a socket the server
      // opens and closes at once with 1011 must keep growing its pause.
      this.attempt = 0;
      // The link is live again once the hall is fresh. Repeating online after
      // a plain subscribe is harmless: the store ignores a repeat.
      this.d.onStatus('online');
      if (this.subscribeSentAt !== null) {
        syncFromServer(msg.state.server_time, this.subscribeSentAt, received);
        this.subscribeSentAt = null;
      }
    }
    if (this.d.onMessage(msg) === true) this.sendSubscribe();
  }

  private sendSubscribe(): void {
    if (!this.sock || !this.hallId) return;
    this.subscribeSentAt = localClock();
    this.safeSend(clientMessage('subscribe', { hall_id: this.hallId }));
  }

  private ping(): void {
    this.pingSentAt = localClock();
    this.safeSend(clientMessage('ping', {}));
    this.pingTimer = this.d.setTimer(() => this.ping(), PING_EVERY_MS);
  }

  private safeSend(data: string): void {
    try {
      this.sock?.send(data);
    } catch {
      this.sock?.close();
    }
  }

  // armWatchdog sleeps until the silence limit would run out. Frames move
  // lastFrameAt only, so the timer checks and sleeps again instead of being
  // reset on every message.
  private armWatchdog(): void {
    if (this.watchdogTimer !== null) this.d.clearTimer(this.watchdogTimer);
    const wait = Math.min(SILENCE_LIMIT_MS, Math.max(0, this.lastFrameAt + SILENCE_LIMIT_MS - localClock()));
    this.watchdogTimer = this.d.setTimer(() => {
      this.watchdogTimer = null;
      if (localClock() - this.lastFrameAt >= SILENCE_LIMIT_MS) this.dropSocket(false);
      else this.armWatchdog();
    }, wait);
  }

  private armProbe(): void {
    if (this.probeTimer !== null) this.d.clearTimer(this.probeTimer);
    const seen = this.frames;
    this.probeTimer = this.d.setTimer(() => {
      this.probeTimer = null;
      if (this.frames === seen) this.dropSocket(true);
    }, RESUME_PROBE_MS);
  }

  // dropSocket abandons a socket that stopped answering. It does not wait for
  // onclose: without a FIN the event may never come, and the late one is
  // ignored because this.sock no longer points at it.
  private dropSocket(immediately: boolean): void {
    const dead = this.sock;
    this.sock = null;
    this.clearTimers();
    try {
      dead?.close();
    } catch {
      // Closing a broken socket may throw; it is abandoned either way.
    }
    this.d.onStatus('offline');
    if (immediately) {
      this.attempt = 0;
      void this.open();
      return;
    }
    this.scheduleReconnect();
  }

  private scheduleReconnect(): void {
    if (this.stopped || this.revoked) return;
    const delay = reconnectDelay(this.attempt, this.d.random);
    this.attempt += 1;
    this.retryTimer = this.d.setTimer(() => {
      this.retryTimer = null;
      void this.open();
    }, delay);
  }

  private clearTimers(): void {
    for (const t of [this.pingTimer, this.retryTimer, this.watchdogTimer, this.probeTimer]) {
      if (t !== null) this.d.clearTimer(t);
    }
    this.pingTimer = null;
    this.retryTimer = null;
    this.watchdogTimer = null;
    this.probeTimer = null;
  }
}
