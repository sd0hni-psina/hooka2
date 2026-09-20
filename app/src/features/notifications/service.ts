import * as Notifications from 'expo-notifications';

import { claimEvent, confirmEvent } from '../../api/endpoints';
import { obj, str } from '../../api/decode';
import { t } from '../../i18n/ru';

// The notification category of tech.md section 10: two buttons on the push.
export const CATEGORY = 'service_action';
export const ACTION_CLAIM = 'claim';
export const ACTION_CONFIRM = 'confirm';

export type PushTarget = { eventId: string; tableId: string; hallId: string };

// parseTarget reads the data the server puts on the push, tech.md section 10.
export function parseTarget(data: unknown): PushTarget | null {
  try {
    const d = obj(data, 'push');
    return { eventId: str(d, 'event_id', 'push'), tableId: str(d, 'table_id', 'push'), hallId: str(d, 'hall_id', 'push') };
  } catch {
    return null;
  }
}

// actionFor decides what a tap on the notification does: a button calls its
// endpoint, the body only opens the table.
export function actionFor(actionIdentifier: string): 'claim' | 'confirm' | 'open' {
  if (actionIdentifier === ACTION_CLAIM) return 'claim';
  if (actionIdentifier === ACTION_CONFIRM) return 'confirm';
  return 'open';
}

export async function registerCategory(): Promise<void> {
  await Notifications.setNotificationCategoryAsync(CATEGORY, [
    { identifier: ACTION_CLAIM, buttonTitle: t.tasks.claim, options: { opensAppToForeground: false } },
    { identifier: ACTION_CONFIRM, buttonTitle: t.tasks.confirm, options: { opensAppToForeground: false } },
  ]);
}

// runAction performs what the tapped button promised.
export async function runAction(action: 'claim' | 'confirm' | 'open', target: PushTarget): Promise<void> {
  if (action === 'claim') await claimEvent(target.eventId);
  if (action === 'confirm') await confirmEvent(target.eventId);
}
