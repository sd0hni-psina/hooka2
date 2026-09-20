import type { TableStatus } from '../api/types';
import { t } from '../i18n/ru';
import type { IconName } from './Icon';
import { tone, type ColorToken } from './tokens';

// Color never carries meaning alone, tech.md section 12: every status also
// has an icon and a word. The palette itself lives in tokens.tone.
export const statusStyle: Record<TableStatus, { color: ColorToken; icon: IconName; label: string }> = {
  free: { color: tone.free.fg, icon: 'dot', label: t.status.free },
  green: { color: tone.green.fg, icon: 'flame', label: t.status.green },
  yellow: { color: tone.yellow.fg, icon: 'clock', label: t.status.yellow },
  red: { color: tone.red.fg, icon: 'alert', label: t.status.red },
};
