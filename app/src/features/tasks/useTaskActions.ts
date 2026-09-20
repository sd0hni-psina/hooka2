import { useCallback, useMemo } from 'react';

import { t } from '../../i18n/ru';
import { useAuthStore } from '../../store/auth';
import { useHallStore } from '../../store/hall';
import type { PendingAction } from '../../store/reducer';
import { feel, toast } from '../../ui';

// One place decides what a task action says out loud. The store already
// painted the result, so a success is silent on purpose.
export function useTaskActions() {
  const act = useHallStore((s) => s.act);
  const staffId = useAuthStore((s) => s.me?.staff.id ?? null);
  const staffName = useAuthStore((s) => s.me?.staff.display_name ?? null);

  const run = useCallback(
    async (kind: PendingAction, eventId: string) => {
      const actor = staffId && staffName ? { id: staffId, name: staffName } : null;
      const result = await act(kind, eventId, actor);
      if (result.status === 'queued') {
        toast({ key: 'queued', message: t.tasks.queued, tone: 'warning' });
        return;
      }
      if (result.status === 'rejected') {
        feel('error');
        toast({ key: eventId, message: result.message, tone: 'danger' });
      }
    },
    [act, staffId, staffName],
  );

  return useMemo(
    () => ({
      claim: (eventId: string) => void run('claim', eventId),
      confirm: (eventId: string) => void run('confirm', eventId),
    }),
    [run],
  );
}
