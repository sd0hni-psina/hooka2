import type { TableStatus } from '../api/types';
import { t } from '../i18n/ru';
import type { IconName } from './Icon';
import type { ColorToken } from './tokens';

// Color never carries meaning alone, tech.md section 12: every status also
// has an icon and a word.
export const statusStyle: Record<TableStatus, { color: ColorToken; icon: IconName; label: string }> = {
  free: { color: 'textMuted', icon: 'dot', label: t.status.free },
  green: { color: 'green', icon: 'flame', label: t.status.green },
  yellow: { color: 'yellow', icon: 'clock', label: t.status.yellow },
  red: { color: 'red', icon: 'alert', label: t.status.red },
};
