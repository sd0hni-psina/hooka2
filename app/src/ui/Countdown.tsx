import type { TableStatus } from '../api/types';
import { countdownMs, formatDuration, type CountdownMode } from '../domain/countdown';
import { statusStyle } from './status';
import { Text } from './Text';
import { useServerNow } from './useServerNow';

type Props = {
  target: string;
  // from freezes the counter at a moment, e.g. finished_at of a stopped hookah.
  from?: string;
  mode: CountdownMode;
  status?: TableStatus;
};

export function Countdown({ target, from, mode, status }: Props) {
  const live = useServerNow();
  const now = from ? Date.parse(from) : live;
  return (
    <Text variant="mono" color={status ? statusStyle[status].color : 'text'}>
      {formatDuration(countdownMs(target, mode, now))}
    </Text>
  );
}
