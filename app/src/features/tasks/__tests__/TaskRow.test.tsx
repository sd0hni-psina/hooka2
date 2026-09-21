import { fireEvent, render } from '@testing-library/react-native';

import type { Hookah, ServiceEvent, Table } from '../../../api/types';
import { t } from '../../../i18n/ru';
import { taskAffordance, type Task } from '../../../store/selectors';
import { TaskRow } from '../TaskRow';

const table: Table = {
  id: 'tbl', hall_id: 'h', label: '15', shape: 'rect', x: 0, y: 0, w: 10, h: 8, rotation: 0, seats: 4,
};

const event: ServiceEvent = {
  id: 'ev', hookah_id: 'hk', seq: 2, kind: 'shake', title: 'Стряхните угли',
  due_at: '2026-09-21T18:00:00Z', state: 'due',
  claimed_by: null, claimed_by_name: null, claimed_at: null,
  confirmed_by: null, confirmed_by_name: null, confirmed_at: null, confirmed_early: false,
};

const hookah: Hookah = {
  id: 'hk', table_id: 'tbl', hall_id: 'h', state: 'active',
  started_at: '2026-09-21T17:30:00Z', expires_at: '2026-09-21T18:50:00Z', finished_at: null,
  started_by: 's', started_by_name: 'Ерлан', status: 'yellow', events: [event],
};

const task = (patch: Partial<Task> = {}): Task => ({
  event, hookah, table, overdueMs: 65_000, pending: null, ...patch,
});

describe('what a task row offers', () => {
  it('offers both actions on a fired task nobody took', () => {
    const a = taskAffordance(task());
    expect(a).toMatchObject({ canClaim: true, canConfirm: true, busy: false, overdue: true, waitMs: 65_000 });
  });

  it('drops the claim button once somebody took the task', () => {
    const a = taskAffordance(task({ event: { ...event, claimed_by: 'other', claimed_by_name: 'Аскар' } }));
    expect(a.canClaim).toBe(false);
    expect(a.claimedBy).toBe('Аскар');
  });

  it('offers no claim on a task that has not fired yet', () => {
    const a = taskAffordance(task({ event: { ...event, state: 'pending' }, overdueMs: -30_000 }));
    expect(a).toMatchObject({ canClaim: false, canConfirm: true, overdue: false, waitMs: 30_000 });
  });

  it('offers nothing while an action is in flight', () => {
    const a = taskAffordance(task({ pending: 'confirm' }));
    expect(a).toMatchObject({ canClaim: false, canConfirm: false, busy: true });
  });
});

describe('TaskRow', () => {
  it('lets a master take and close the task without leaving the list', async () => {
    const onClaim = jest.fn();
    const onConfirm = jest.fn();
    const view = await render(
      <TaskRow task={task()} variant="row" onOpen={() => undefined} onClaim={onClaim} onConfirm={onConfirm} />,
    );
    fireEvent.press(view.getByLabelText(t.tasks.claim));
    fireEvent.press(view.getByLabelText(t.tasks.confirm));
    expect(onClaim).toHaveBeenCalledTimes(1);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });
});
