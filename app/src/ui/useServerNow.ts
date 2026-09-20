import { useEffect, useState } from 'react';

import { serverNow } from '../ws/clock';
import { subscribeTick, type Granularity } from '../ws/ticker';

// useServerNow re-renders on the shared ticker. Granularity 'minute' keeps
// the hall map still between minutes, tech.md section 19.15.
export function useServerNow(granularity: Granularity = 'second'): number {
  const [now, setNow] = useState(serverNow);
  useEffect(() => subscribeTick(granularity, setNow), [granularity]);
  return now;
}
