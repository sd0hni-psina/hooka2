import { t } from '../../i18n/ru';
import type { Connection } from '../../store/hall';
import type { IconName } from '../../ui/Icon';

export type Line = {
  key: 'live' | 'offline' | 'connecting' | 'stale' | 'queued';
  title: string;
  tone: 'green' | 'red' | 'neutral' | 'yellow' | 'accent';
  icon: IconName;
};

type Input = {
  connection: Connection;
  stale: boolean;
  // time is the clock time of the data on screen, set when it is stale.
  time: string | null;
  // queued counts taps waiting for the network.
  queued: number;
};

// connectionLine decides what the status line says. A green "connected" never
// stands next to old data: until a snapshot lands, the age is the news.
export function connectionLine({ connection, stale, time, queued }: Input): Line[] {
  const lines: Line[] = [];
  const age: Line | null = stale && time ? { key: 'stale', title: t.hall.stale(time), tone: 'yellow', icon: 'clockOff' } : null;

  if (connection === 'online') {
    if (age) lines.push(age);
    else if (!stale) lines.push({ key: 'live', title: t.hall.online, tone: 'green', icon: 'check' });
  } else if (connection === 'offline') {
    lines.push({ key: 'offline', title: t.hall.offline, tone: 'red', icon: 'wifi-off' });
    if (age) lines.push(age);
  } else if (age) {
    lines.push(age);
  } else {
    lines.push({ key: 'connecting', title: t.hall.connecting, tone: 'neutral', icon: 'refresh' });
  }

  if (queued > 0) lines.push({ key: 'queued', title: t.hall.queued(queued), tone: 'accent', icon: 'refresh' });
  return lines;
}
