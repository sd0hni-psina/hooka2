import fixture from '../../../../contracts/fixtures/ws_events.json';
import { __setLocalClockForTests, serverNow } from '../clock';
import { WsClient, type WsStatus } from '../client';
import type { ParsedMessage } from '../envelope';

type Frame = { type: string; data: Record<string, string> };

class FakeSocket {
  onopen: (() => void) | null = null;
  onclose: ((e: { code?: number }) => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  sent: Frame[] = [];
  closed = false;
  constructor(readonly url: string) {}
  send(data: string) {
    this.sent.push(JSON.parse(data) as Frame);
  }
  // close is what the client calls: the peer never answers a dead socket, so
  // nothing happens until the test says the server closed it.
  close() {
    this.closed = true;
  }
  // serverClose is the server, or the network, ending the connection.
  serverClose(code = 1006) {
    this.closed = true;
    this.onclose?.({ code });
  }
  receive(msg: object) {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }
}

// Virtual time: the client reads localClock() and sets timers through the
// injected functions, so a test moves both in one call.
let now = 0;
type Timer = { fn: () => void; ms: number; at: number };

function setup(onMessage: (m: ParsedMessage) => boolean | void = () => undefined, extra: { onRevoked?: () => void } = {}) {
  const sockets: FakeSocket[] = [];
  const timers: Timer[] = [];
  const statuses: WsStatus[] = [];
  const client = new WsClient({
    url: async () => 'ws://api/api/v1/ws?access_token=t',
    onMessage,
    onStatus: (s) => statuses.push(s),
    socket: (url) => {
      const s = new FakeSocket(url);
      sockets.push(s);
      return s;
    },
    setTimer: (fn, ms) => {
      const t = { fn, ms, at: now + ms };
      timers.push(t);
      return t;
    },
    clearTimer: (t) => {
      const i = timers.indexOf(t as Timer);
      if (i >= 0) timers.splice(i, 1);
    },
    random: () => 0,
    ...extra,
  });
  // advance runs every timer due inside the span, in order, with the clock
  // standing at each timer's moment.
  const advance = async (ms: number) => {
    const target = now + ms;
    for (;;) {
      const due = timers.filter((t) => t.at <= target).sort((a, b) => a.at - b.at)[0];
      if (!due) break;
      timers.splice(timers.indexOf(due), 1);
      now = due.at;
      due.fn();
      await flush();
    }
    now = target;
  };
  return { client, sockets, timers, statuses, advance };
}

const flush = () => new Promise<void>((r) => setImmediate(() => r()));

const envelope = (type: string, seq: number, data: object, hall: string | null = null) => ({
  v: 1, type, seq, ts: '2026-09-19T18:00:00Z', venue_id: 'v', hall_id: hall, data,
});

const snapshotFixture = fixture.events.find((e) => e.type === 'state.snapshot')!;
const snapshotFrame = () => ({ ...snapshotFixture, seq: 41 });

const types = (s: FakeSocket) => s.sent.map((m) => m.type);

// open starts the client on hall-1 and lets the first socket open.
async function open(s: ReturnType<typeof setup>) {
  s.client.subscribe('hall-1');
  s.client.start();
  await flush();
  s.sockets[0]!.onopen?.();
}

beforeEach(() => {
  now = Date.parse('2026-09-19T18:10:00Z');
  __setLocalClockForTests(() => now);
});

describe('WsClient', () => {
  it('subscribes and pings as soon as the socket opens', async () => {
    const s = setup();
    await open(s);
    expect(types(s.sockets[0]!)).toEqual(['subscribe', 'ping']);
    expect(s.sockets[0]!.sent[0]!.data).toEqual({ hall_id: 'hall-1' });
    expect(s.statuses).toEqual(['connecting', 'online']);
  });

  it('reconnects with growing delays and resubscribes after reopening', async () => {
    const { client, sockets, timers } = setup();
    client.subscribe('hall-1');
    client.start();
    await flush();
    sockets[0]!.serverClose();
    expect(timers.map((t) => t.ms)).toEqual([1000]);
    timers.shift()!.fn();
    await flush();
    sockets[1]!.serverClose();
    expect(timers.map((t) => t.ms)).toEqual([2000]);
    timers.shift()!.fn();
    await flush();
    sockets[2]!.onopen?.();
    expect(sockets[2]!.sent[0]).toEqual({ v: 1, type: 'subscribe', data: { hall_id: 'hall-1' } });
  });

  it('resubscribes when the store reports a gap', async () => {
    const s = setup((m) => m.kind === 'event');
    await open(s);
    s.sockets[0]!.receive(envelope('hookah.expired', 9, {}, 'hall-1'));
    expect(types(s.sockets[0]!)).toEqual(['subscribe', 'ping', 'subscribe']);
  });

  it('resubscribes after a message it cannot parse', async () => {
    const s = setup();
    await open(s);
    s.sockets[0]!.receive({ type: 'service.due', data: {} });
    expect(types(s.sockets[0]!)).toEqual(['subscribe', 'ping', 'subscribe']);
  });

  it('syncs the clock from pong', async () => {
    const s = setup();
    s.client.start();
    await flush();
    s.sockets[0]!.onopen?.();
    s.sockets[0]!.receive(envelope('pong', 0, { server_time: '2026-09-19T18:00:00Z' }));
    expect(serverNow()).toBe(Date.parse('2026-09-19T18:00:00Z'));
  });

  it('stops reconnecting after stop', async () => {
    const { client, sockets, timers } = setup();
    client.start();
    await flush();
    client.stop();
    expect(sockets[0]!.closed).toBe(true);
    expect(timers).toEqual([]);
  });
});

describe('silent socket', () => {
  it('drops a socket that stayed silent for 45 seconds and opens a new one', async () => {
    const s = setup();
    await open(s);
    await s.advance(44_000);
    expect(s.sockets).toHaveLength(1);
    expect(s.statuses[s.statuses.length - 1]).toBe('online');

    await s.advance(2_000);
    expect(s.sockets[0]!.closed).toBe(true);
    expect(s.statuses).toContain('offline');
    // The backoff of section 9 applies: the new socket comes after its first pause.
    await s.advance(2_000);
    expect(s.sockets).toHaveLength(2);
  });

  it('counts silence from the last frame, whatever its type', async () => {
    const s = setup();
    await open(s);
    await s.advance(30_000);
    s.sockets[0]!.receive(envelope('pong', 0, { server_time: '2026-09-19T18:10:30Z' }));
    await s.advance(30_000);
    expect(s.sockets).toHaveLength(1);
    s.sockets[0]!.receive(snapshotFrame());
    await s.advance(30_000);
    expect(s.sockets).toHaveLength(1);
    await s.advance(20_000);
    expect(s.sockets[0]!.closed).toBe(true);
  });

  it('drops a socket that never opened', async () => {
    const s = setup();
    s.client.subscribe('hall-1');
    s.client.start();
    await flush();
    await s.advance(46_000);
    expect(s.sockets[0]!.closed).toBe(true);
    expect(s.statuses).toContain('offline');
  });

  it('ignores the late close of a socket it already dropped', async () => {
    const s = setup();
    await open(s);
    // The watchdog fires at 45 s, the first retry is due at 46 s.
    await s.advance(45_500);
    expect(s.timers.map((t) => t.ms)).toEqual([1000]);
    s.sockets[0]!.serverClose();
    // Still one retry: the late close belongs to a socket nobody listens to.
    expect(s.timers.map((t) => t.ms)).toEqual([1000]);
    await s.advance(2_000);
    expect(s.sockets).toHaveLength(2);
  });

  it('opens one socket when resumed while still connecting', async () => {
    const s = setup();
    s.client.subscribe('hall-1');
    s.client.start();
    s.client.resume();
    await flush();
    expect(s.sockets).toHaveLength(1);
  });
});

describe('return from background', () => {
  it('opens at once instead of waiting out the backoff', async () => {
    const s = setup();
    s.client.subscribe('hall-1');
    s.client.start();
    await flush();
    s.sockets[0]!.serverClose();

    s.client.resume();
    await flush();
    expect(s.sockets).toHaveLength(2);
    // The pending retry is cancelled, so it never opens a third one.
    await s.advance(5_000);
    expect(s.sockets).toHaveLength(2);
  });

  it('asks for a fresh snapshot when the socket survived the background', async () => {
    const s = setup();
    await open(s);
    s.sockets[0]!.receive(snapshotFrame());
    s.sockets[0]!.sent.length = 0;

    s.client.resume();
    await flush();
    expect(s.sockets).toHaveLength(1);
    expect(types(s.sockets[0]!)).toEqual(['subscribe']);
  });

  it('does not vouch for the hall between the return and the fresh snapshot', async () => {
    const s = setup();
    await open(s);
    s.sockets[0]!.receive(snapshotFrame());
    s.statuses.length = 0;

    s.client.resume();
    // Two minutes of sleep are behind the screen: until the snapshot lands the
    // link is "connecting", not "online".
    expect(s.statuses).toEqual(['connecting']);
    s.sockets[0]!.receive(snapshotFrame());
    expect(s.statuses).toEqual(['connecting', 'online']);
  });

  it('listens again after stop and start following a 1008', async () => {
    const s = setup();
    await open(s);
    s.sockets[0]!.serverClose(1008);
    s.client.stop();
    s.client.start();
    await flush();
    s.sockets[1]!.serverClose(1006);
    await s.advance(2_000);
    expect(s.sockets).toHaveLength(3);
  });

  it('replaces a socket that stays silent for 3 seconds after resume, without a backoff', async () => {
    const s = setup();
    await open(s);
    s.sockets[0]!.receive(snapshotFrame());
    await s.advance(10_000);

    s.client.resume();
    await s.advance(2_900);
    expect(s.sockets).toHaveLength(1);
    await s.advance(200);
    expect(s.sockets[0]!.closed).toBe(true);
    expect(s.sockets).toHaveLength(2);
  });

  it('keeps a socket that answers after resume', async () => {
    const s = setup();
    await open(s);
    await s.advance(10_000);

    s.client.resume();
    await s.advance(1_000);
    s.sockets[0]!.receive(snapshotFrame());
    await s.advance(10_000);
    expect(s.sockets).toHaveLength(1);
    expect(s.sockets[0]!.closed).toBe(false);
  });
});

describe('close codes', () => {
  it('does not reconnect after 1008 and tells the app the session may be revoked', async () => {
    const onRevoked = jest.fn();
    const s = setup(() => undefined, { onRevoked });
    await open(s);
    s.sockets[0]!.serverClose(1008);
    expect(onRevoked).toHaveBeenCalledTimes(1);
    expect(s.statuses[s.statuses.length - 1]).toBe('offline');

    // No loop: minutes pass and nobody knocks on the door again.
    await s.advance(5 * 60_000);
    expect(s.sockets).toHaveLength(1);
    expect(onRevoked).toHaveBeenCalledTimes(1);
  });

  it('reconnects after 1008 only when the app comes back to the foreground', async () => {
    const s = setup();
    await open(s);
    s.sockets[0]!.serverClose(1008);
    s.client.resume();
    await flush();
    expect(s.sockets).toHaveLength(2);
  });

  it('reconnects after 1011 with a growing pause while no snapshot arrives', async () => {
    const s = setup();
    s.client.subscribe('hall-1');
    s.client.start();
    const delays: number[] = [];
    for (let i = 0; i < 3; i++) {
      await flush();
      s.sockets[i]!.onopen?.();
      s.timers.length = 0;
      s.sockets[i]!.serverClose(1011);
      delays.push(s.timers[0]!.ms);
      s.timers.shift()!.fn();
    }
    expect(delays).toEqual([1000, 2000, 4000]);
  });

  it('resets the backoff on the first snapshot, not on open', async () => {
    const s = setup();
    s.client.subscribe('hall-1');
    s.client.start();
    await flush();
    s.sockets[0]!.serverClose(1011);
    s.timers.shift()!.fn();
    await flush();
    s.sockets[1]!.serverClose(1011);
    s.timers.shift()!.fn();
    await flush();

    s.sockets[2]!.onopen?.();
    s.sockets[2]!.receive(snapshotFrame());
    s.timers.length = 0;
    s.sockets[2]!.serverClose(1011);
    expect(s.timers.map((t) => t.ms)).toEqual([1000]);
  });

  it('treats an unknown close code as a plain drop', async () => {
    const onRevoked = jest.fn();
    const s = setup(() => undefined, { onRevoked });
    await open(s);
    s.sockets[0]!.serverClose(1006);
    await s.advance(2_000);
    expect(onRevoked).not.toHaveBeenCalled();
    expect(s.sockets).toHaveLength(2);
  });
});
