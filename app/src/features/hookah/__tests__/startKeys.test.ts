import { ApiError, NetworkError } from '../../../api/errors';
import { StartKeys } from '../startKeys';

const network = () => new NetworkError(new Error('timeout'));
const occupied = () => new ApiError('table_occupied', 409, 'занято', null);
const proxyDown = () => new ApiError('internal', 502, 'HTTP 502', null);

function setup() {
  let n = 0;
  const minted: string[] = [];
  const keys = new StartKeys(() => {
    n += 1;
    minted.push(`key-${n}`);
    return `key-${n}`;
  });
  // sent records the key of every request, in order.
  const sent: string[] = [];
  const attempt = (table: string, now: number, outcome: () => Promise<string>) =>
    keys.run(table, now, async (key) => {
      sent.push(key);
      return outcome();
    });
  return { attempt, sent, minted };
}

const ok = async () => 'hookah';
const fail = (e: () => Error) => async (): Promise<string> => {
  throw e();
};

describe('start key of a hookah', () => {
  it('reuses the key after a network error', async () => {
    const { attempt, sent } = setup();
    await expect(attempt('t1', 0, fail(network))).rejects.toBeInstanceOf(NetworkError);
    await attempt('t1', 5_000, ok);
    expect(sent).toEqual(['key-1', 'key-1']);
  });

  it('reuses the key after a 5xx, because the request may have landed', async () => {
    const { attempt, sent } = setup();
    await expect(attempt('t1', 0, fail(proxyDown))).rejects.toBeInstanceOf(ApiError);
    await attempt('t1', 1_000, ok);
    expect(sent).toEqual(['key-1', 'key-1']);
  });

  it('mints a new key after a success', async () => {
    const { attempt, sent } = setup();
    await attempt('t1', 0, ok);
    await attempt('t1', 1_000, ok);
    expect(sent).toEqual(['key-1', 'key-2']);
  });

  it('mints a new key after the server answered no', async () => {
    const { attempt, sent } = setup();
    await expect(attempt('t1', 0, fail(occupied))).rejects.toMatchObject({ code: 'table_occupied' });
    await attempt('t1', 1_000, ok);
    expect(sent).toEqual(['key-1', 'key-2']);
  });

  it('gives every table its own key', async () => {
    const { attempt, sent } = setup();
    await expect(attempt('t1', 0, fail(network))).rejects.toBeDefined();
    await expect(attempt('t2', 0, fail(network))).rejects.toBeDefined();
    await attempt('t1', 1_000, ok);
    await attempt('t2', 1_000, ok);
    expect(sent).toEqual(['key-1', 'key-2', 'key-1', 'key-2']);
  });

  it('lets a key retire after 2 minutes', async () => {
    // The server keeps a key forever. Reused an hour later, it would hand back
    // a hookah that finished long ago instead of starting a new one.
    const { attempt, sent } = setup();
    await expect(attempt('t1', 0, fail(network))).rejects.toBeDefined();
    await expect(attempt('t1', 119_999, fail(network))).rejects.toBeDefined();
    await attempt('t1', 120_000, ok);
    expect(sent).toEqual(['key-1', 'key-1', 'key-2']);
  });

  it('counts the 2 minutes from the first send, not from the last retry', async () => {
    const { attempt, sent } = setup();
    await expect(attempt('t1', 0, fail(network))).rejects.toBeDefined();
    await expect(attempt('t1', 100_000, fail(network))).rejects.toBeDefined();
    await attempt('t1', 121_000, ok);
    expect(sent).toEqual(['key-1', 'key-1', 'key-2']);
  });

  it('sends the same key for a double tap while the first request is in flight', async () => {
    const { attempt, sent } = setup();
    let release: (v: string) => void = () => undefined;
    const slow = () => new Promise<string>((r) => (release = r));
    const first = attempt('t1', 0, slow);
    const second = attempt('t1', 200, async () => 'hookah');
    release('hookah');
    await Promise.all([first, second]);
    expect(sent).toEqual(['key-1', 'key-1']);
  });
});
