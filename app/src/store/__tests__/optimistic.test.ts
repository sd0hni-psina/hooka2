import fixture from '../../../../contracts/fixtures/ws_events.json';
import { ApiError, NetworkError } from '../../api/errors';
import type { Envelope, ServiceEvent } from '../../api/types';
import { __setLocalClockForTests } from '../../ws/clock';
import { parseMessage } from '../../ws/envelope';
import { useHallStore } from '../hall';
import { selectTasks, view } from '../selectors';

jest.mock('../../api/endpoints', () => ({
  claimEvent: jest.fn(),
  confirmEvent: jest.fn(),
}));

import { claimEvent, confirmEvent } from '../../api/endpoints';

const mockClaim = claimEvent as jest.MockedFunction<typeof claimEvent>;
const mockConfirm = confirmEvent as jest.MockedFunction<typeof confirmEvent>;

const byType = (type: string) => fixture.events.find((e) => e.type === type) as Envelope;
const snapshotEnv = byType('state.snapshot');
const hallId = snapshotEnv.hall_id as string;
const actor = { id: 'staff-me', name: 'Ерлан' };

function snapshot(seq = 41) {
  return parseMessage(JSON.stringify({ ...snapshotEnv, seq }));
}

function firstEvent(): ServiceEvent {
  const state = useHallStore.getState().state;
  const event = state?.hookahs[0]?.events[0];
  if (!event) throw new Error('fixture has no service event');
  return event;
}

function shown(eventId: string): ServiceEvent {
  const event = view(useHallStore.getState())?.hookahs[0]?.events.find((e) => e.id === eventId);
  if (!event) throw new Error('event vanished from the view');
  return event;
}

describe('optimistic claim and confirm', () => {
  beforeEach(() => {
    __setLocalClockForTests(() => 1_700_000_000_000);
    mockClaim.mockReset();
    mockConfirm.mockReset();
    useHallStore.getState().selectHall(hallId);
    useHallStore.getState().apply(snapshot());
    useHallStore.getState().setConnection('online');
  });

  it('shows the confirmation before the server answers', async () => {
    const event = firstEvent();
    let release: (e: ServiceEvent) => void = () => undefined;
    mockConfirm.mockReturnValue(new Promise<ServiceEvent>((res) => { release = res; }));

    const inflight = useHallStore.getState().act('confirm', event.id, actor);
    expect(shown(event.id).state).toBe('confirmed');
    expect(shown(event.id).confirmed_by_name).toBe('Ерлан');
    release({ ...event, state: 'confirmed', confirmed_by: 'staff-me', confirmed_by_name: 'Ерлан из ответа' });
    await expect(inflight).resolves.toBe('sent');
    expect(shown(event.id).confirmed_by_name).toBe('Ерлан из ответа');
  });

  it('marks a claimed task as pending until the server answers', () => {
    const event = firstEvent();
    mockClaim.mockReturnValue(new Promise<ServiceEvent>(() => undefined));

    void useHallStore.getState().act('claim', event.id, actor);
    const task = selectTasks(useHallStore.getState(), Date.parse(event.due_at) + 1000)
      .find((t) => t.event.id === event.id);
    expect(task?.pending).toBe('claim');
    expect(task?.event.claimed_by_name).toBe('Ерлан');
  });

  it('leaves no trace of a guess the server refused', async () => {
    const event = firstEvent();
    mockClaim.mockRejectedValue(new ApiError('event_not_actionable', 409, 'занято', null));

    await expect(useHallStore.getState().act('claim', event.id, actor)).resolves.toBe('rejected');
    expect(shown(event.id).claimed_by_name).toBe(event.claimed_by_name);
    expect(useHallStore.getState().pending).toEqual({});
  });

  it('keeps the guess and queues the action when the network is gone', async () => {
    const event = firstEvent();
    mockConfirm.mockRejectedValue(new NetworkError(new Error('offline')));

    await expect(useHallStore.getState().act('confirm', event.id, actor)).resolves.toBe('queued');
    expect(shown(event.id).state).toBe('confirmed');
    expect(useHallStore.getState().queued).toBe(1);
  });

  it('replays the queue when the connection comes back', async () => {
    const event = firstEvent();
    mockConfirm.mockRejectedValueOnce(new NetworkError(new Error('offline')));
    await useHallStore.getState().act('confirm', event.id, actor);

    __setLocalClockForTests(() => 1_700_000_010_000);
    mockConfirm.mockResolvedValueOnce({ ...event, state: 'confirmed', confirmed_by_name: 'Ерлан' });
    useHallStore.getState().setConnection('offline');
    useHallStore.getState().setConnection('online');
    await Promise.resolve();
    await Promise.resolve();

    expect(mockConfirm).toHaveBeenCalledTimes(2);
    expect(useHallStore.getState().queued).toBe(0);
  });

  it('drops the guess as soon as the server event arrives', async () => {
    const event = firstEvent();
    mockConfirm.mockResolvedValue({ ...event, state: 'confirmed' });
    await useHallStore.getState().act('confirm', event.id, actor);
    expect(Object.keys(useHallStore.getState().pending)).toEqual([event.id]);

    const confirmed = byType('service.confirmed');
    const data = structuredClone(confirmed.data) as Record<string, unknown>;
    data.event_id = event.id;
    data.hookah_id = useHallStore.getState().state?.hookahs[0]?.id;
    useHallStore.getState().apply(parseMessage(JSON.stringify({ ...confirmed, seq: 42, data })));

    expect(useHallStore.getState().pending).toEqual({});
  });

  it('forgets every guess when a fresh snapshot lands', async () => {
    const event = firstEvent();
    mockClaim.mockRejectedValue(new NetworkError(new Error('offline')));
    await useHallStore.getState().act('claim', event.id, actor);
    expect(Object.keys(useHallStore.getState().pending)).toHaveLength(1);

    useHallStore.getState().apply(snapshot(50));
    expect(useHallStore.getState().pending).toEqual({});
    expect(useHallStore.getState().stale).toBe(false);
  });
});
