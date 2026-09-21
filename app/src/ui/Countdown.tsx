import type { TableStatus } from '../api/types';
import { countdownMs, formatDuration, formatShort, type CountdownMode } from '../domain/countdown';
import { Text, type TextVariant } from './Text';
import { tone } from './tokens';
import { useServerNow } from './useServerNow';
import type { Granularity } from '../ws/ticker';

type Props = {
  target: string;
  // from freezes the counter at a moment, e.g. finished_at of a stopped hookah.
  from?: string;
  mode: CountdownMode;
  status?: TableStatus;
  variant?: Extract<TextVariant, 'display' | 'mono' | 'label'>;
  // granularity 'minute' keeps a slow counter from waking the screen every
  // second, tech.md section 19.15.
  granularity?: Granularity;
};

export function Countdown({ target, from, mode, status, variant = 'mono', granularity = 'second' }: Props) {
  const live = useServerNow(granularity);
  const now = from ? Date.parse(from) : live;
  const ms = countdownMs(target, mode, now);
  return (
    <Text variant={variant} color={status ? tone[status].fg : 'text'}>
      {granularity === 'minute' ? formatShort(ms) : formatDuration(ms)}
    </Text>
  );
}
