import fixture from '../../../../contracts/fixtures/ws_events.json';
import { parseMessage } from '../envelope';

describe('WS envelopes against the shared fixture', () => {
  it.each(fixture.events.map((e) => [e.type, e] as const))('%s parses without losing fields', (_type, e) => {
    const msg = parseMessage(JSON.stringify(e));
    expect(msg.env).toEqual(e);
    if (msg.kind === 'snapshot') expect(msg.state).toEqual(e.data);
    if (msg.kind === 'pong') expect(msg.serverTime).toBe((e.data as { server_time: string }).server_time);
    if (msg.kind === 'error') expect(msg.code).toBe((e.data as { code: string }).code);
  });

  it('rejects an envelope without seq', () => {
    expect(() => parseMessage(JSON.stringify({ v: 1, type: 'pong', ts: 'x', venue_id: 'v', hall_id: null, data: {} }))).toThrow(
      /seq/,
    );
  });
});
