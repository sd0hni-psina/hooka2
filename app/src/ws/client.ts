import { parseMessage, clientMessage, type ParsedMessage } from './envelope';
import { localClock, syncFromServer } from './clock';
import { reconnectDelay } from './reconnect';

const PING_EVERY_MS = 20_000;

export type WsStatus = 'connecting' | 'online' | 'offline';

type SocketLike = {
  onopen: (() => void) | null;
  onclose: (() => void) | null;
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
  socket?: (url: string) => SocketLike;
  setTimer?: (fn: () => void, ms: number) => unknown;
  clearTimer?: (t: unknown) => void;
  random?: () => number;
};

// WsClient keeps one connection alive: reconnects with backoff, pings every
// 20 s, subscribes after each open and whenever onMessage asks to resync.
export class WsClient {
  private sock: SocketLike | null = null;
  private hallId: string | null = null;
  private attempt = 0;
  private stopped = true;
  private retryTimer: unknown = null;
  private pingTimer: unknown = null;
  private pingSentAt: number | null = null;
  private subscribeSentAt: number | null = null;
  private readonly d: Required<WsDeps>;

  constructor(deps: WsDeps) {
    this.d = {
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
  resume(): void {
    if (this.stopped) return;
    this.attempt = 0;
    if (this.retryTimer !== null) {
      this.d.clearTimer(this.retryTimer);
      this.retryTimer = null;
    }
    if (this.sock) {
      this.sendSubscribe();
      return;
    }
    void this.open();
  }

  subscribe(hallId: string | null): void {
    this.hallId = hallId;
    this.sendSubscribe();
  }

  private async open(): Promise<void> {
    if (this.stopped) return;
    this.d.onStatus('connecting');
    let url: string | null = null;
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
    s.onopen = () => {
      if (this.sock !== s) return;
      this.attempt = 0;
      this.d.onStatus('online');
      this.sendSubscribe();
      this.ping();
    };
    s.onmessage = (e) => {
      if (this.sock !== s || typeof e.data !== 'string') return;
      this.handle(e.data);
    };
    s.onerror = () => s.close();
    s.onclose = () => {
      if (this.sock !== s) return;
      this.sock = null;
      this.clearTimers();
      this.d.onStatus('offline');
      this.scheduleReconnect();
    };
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
    if (msg.kind === 'snapshot' && this.subscribeSentAt !== null) {
      syncFromServer(msg.state.server_time, this.subscribeSentAt, received);
      this.subscribeSentAt = null;
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

  private scheduleReconnect(): void {
    if (this.stopped) return;
    const delay = reconnectDelay(this.attempt, this.d.random);
    this.attempt += 1;
    this.retryTimer = this.d.setTimer(() => {
      this.retryTimer = null;
      void this.open();
    }, delay);
  }

  private clearTimers(): void {
    if (this.pingTimer !== null) this.d.clearTimer(this.pingTimer);
    if (this.retryTimer !== null) this.d.clearTimer(this.retryTimer);
    this.pingTimer = null;
    this.retryTimer = null;
  }
}
