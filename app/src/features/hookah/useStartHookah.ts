import * as Crypto from 'expo-crypto';
import { useCallback, useState } from 'react';

import { startHookah } from '../../api/endpoints';
import { explain } from '../../api/errors';
import type { Table } from '../../api/types';
import { serverNow } from '../../ws/clock';
import { toast } from '../../ui';
import { StartKeys } from './startKeys';

// One registry for the whole app: the map screen and the table screen start
// the same tables, and a retry from the other screen must carry the same key.
const keys = new StartKeys(() => Crypto.randomUUID());

// useStartHookah is the only way to put a hookah on a table. The result is
// silent on success: the WS event paints the table. A failure speaks once.
export function useStartHookah() {
  const [starting, setStarting] = useState(false);

  const start = useCallback(async (table: Table): Promise<boolean> => {
    setStarting(true);
    try {
      await keys.run(table.id, serverNow(), (key) => startHookah(table.id, key));
      return true;
    } catch (e) {
      toast({ key: 'start', message: explain(e), tone: 'danger' });
      return false;
    } finally {
      setStarting(false);
    }
  }, []);

  return { start, starting };
}
