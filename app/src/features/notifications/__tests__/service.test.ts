import { actionFor, parseTarget } from '../service';

describe('notification handling', () => {
  it('reads the target of a push', () => {
    const data = {
      event_id: 'e1', hookah_id: 'h1', table_id: 't1', hall_id: 'hall1', kind: 'shake', due_at: '2026-09-19T18:00:00Z',
    };
    expect(parseTarget(data)).toEqual({ eventId: 'e1', tableId: 't1', hallId: 'hall1' });
  });

  it('ignores a push without the fields it needs', () => {
    expect(parseTarget({ event_id: 'e1' })).toBeNull();
    expect(parseTarget(null)).toBeNull();
  });

  it('maps the notification buttons to actions', () => {
    expect(actionFor('claim')).toBe('claim');
    expect(actionFor('confirm')).toBe('confirm');
    expect(actionFor('expo.modules.notifications.actions.DEFAULT')).toBe('open');
  });
});
