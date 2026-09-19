import { useEffect, useState } from 'react';

import { serverNow } from '../ws/clock';

// useServerNow re-renders every intervalMs with the server clock.
export function useServerNow(intervalMs = 1000): number {
  const [now, setNow] = useState(serverNow);
  useEffect(() => {
    const id = setInterval(() => setNow(serverNow()), intervalMs);
    return () => clearInterval(id);
  }, [intervalMs]);
  return now;
}
