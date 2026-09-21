import { __setLocalClockForTests, serverNow } from '../clock';
import { WsClient, type WsStatus } from '../client';
import type { ParsedMessage } from '../envelope';

class FakeSocket {
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  sent: { type: string; data: Record<string, string> }[] = [];
  closed = false;
  constructor(readonly url: string) {}
  send(data: string) {
    this.sent.push(JSON.parse(data) as { type: string; data: Record<string, string> });
  }
  close() {
    if (this.closed) return;
    this.closed = true;
    this.onclose?.();
  }
  receive(msg: object) {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }
}

type Timer = { fn: () => void; ms: number };

function setup(onMessage: (m: ParsedMessage) => boolean | void = () => undefined) {
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
      const t = { fn, ms };
      timers.push(t);
      return t;
    },
    clearTimer: (t) => {
      const i = timers.indexOf(t as Timer);
      if (i >= 0) timers.splice(i, 1);
    },
    random: () => 0,
  });
  return { client, sockets, timers, statuses };
}

const flush = () => new Promise<void>((r) => setImmediate(() => r()));

const envelope = (type: string, seq: number, data: object, hall: string | null = null) => ({
  v: 1, type, seq, ts: '2026-09-19T18:00:00Z', venue_id: 'v', hall_id: hall, data,
});

describe('WsClient', () => {
  beforeEach(() => __setLocalClockForTests(() => Date.parse('2026-09-19T18:10:00Z')));

  it('subscribes and pings as soon as the socket opens', async () => {
    const { client, sockets, statuses } = setup();
    client.subscribe('hall-1');
    client.start();
    await flush();
    sockets[0]!.onopen?.();
    expect(sockets[0]!.sent.map((m) => m.type)).toEqual(['subscribe', 'ping']);
    expect(sockets[0]!.sent[0]!.data).toEqual({ hall_id: 'hall-1' });
    expect(statuses).toEqual(['connecting', 'online']);
  });

  it('reconnects with growing delays and resubscribes after reopening', async () => {
    const { client, sockets, timers } = setup();
    client.subscribe('hall-1');
    client.start();
    await flush();
    sockets[0]!.close();
    expect(timers.map((t) => t.ms)).toEqual([1000]);
    timers.shift()!.fn();
    await flush();
    sockets[1]!.close();
    expect(timers.map((t) => t.ms)).toEqual([2000]);
    timers.shift()!.fn();
    await flush();
    sockets[2]!.onopen?.();
    expect(sockets[2]!.sent[0]).toEqual({ v: 1, type: 'subscribe', data: { hall_id: 'hall-1' } });
  });

  it('resets the backoff after a successful open', async () => {
    const { client, sockets, timers } = setup();
    client.start();
    await flush();
    sockets[0]!.close();
    timers.shift()!.fn();
    await flush();
    sockets[1]!.onopen?.();
    timers.length = 0;
    sockets[1]!.close();
    expect(timers.map((t) => t.ms)).toEqual([1000]);
  });

  it('resubscribes when the store reports a gap', async () => {
    const { client, sockets } = setup((m) => m.kind === 'event');
    client.subscribe('hall-1');
    client.start();
    await flush();
    sockets[0]!.onopen?.();
    sockets[0]!.receive(envelope('hookah.expired', 9, {}, 'hall-1'));
    expect(sockets[0]!.sent.map((m) => m.type)).toEqual(['subscribe', 'ping', 'subscribe']);
  });

  it('resubscribes after a message it cannot parse', async () => {
    const { client, sockets } = setup();
    client.subscribe('hall-1');
    client.start();
    await flush();
    sockets[0]!.onopen?.();
    sockets[0]!.receive({ type: 'service.due', data: {} });
    expect(sockets[0]!.sent.map((m) => m.type)).toEqual(['subscribe', 'ping', 'subscribe']);
  });

  it('syncs the clock from pong', async () => {
    const { client, sockets } = setup();
    client.start();
    await flush();
    sockets[0]!.onopen?.();
    sockets[0]!.receive(envelope('pong', 0, { server_time: '2026-09-19T18:00:00Z' }));
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

describe('return from background', () => {
  it('opens at once instead of waiting out the backoff', async () => {
    const { client, sockets, timers } = setup();
    client.subscribe('hall-1');
    client.start();
    await flush();
    sockets[0]?.close();
    // A pending retry is what the master would otherwise wait for.
    expect(timers.some((t) => t.ms >= 1000)).toBe(true);

    client.resume();
    await flush();
    expect(sockets).toHaveLength(2);
    expect(timers.some((t) => t.ms >= 1000)).toBe(false);
  });

  it('asks for a fresh snapshot when the socket survived the background', async () => {
    const { client, sockets } = setup();
    client.subscribe('hall-1');
    client.start();
    await flush();
    const socket = sockets[0];
    socket?.sent.splice(0, socket.sent.length);

    client.resume();
    await flush();
    expect(sockets).toHaveLength(1);
    expect(socket?.sent.map((m) => m.type)).toEqual(['subscribe']);
  });
});
