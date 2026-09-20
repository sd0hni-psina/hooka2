import { create } from 'zustand';

import type { ParsedMessage } from '../ws/envelope';
import { emptyHall, reduce, type HallData } from './reducer';

export type Connection = 'idle' | 'connecting' | 'online' | 'offline';

type HallStore = HallData & {
  connection: Connection;
  selectHall: (hallId: string | null) => void;
  apply: (msg: ParsedMessage) => boolean;
  setConnection: (c: Connection) => void;
  dismissBanner: () => void;
};

// One store per hall: switching halls drops the previous state entirely.
export const useHallStore = create<HallStore>((set, get) => ({
  ...emptyHall,
  connection: 'idle',
  selectHall: (hallId) => set({ ...emptyHall, hallId }),
  apply: (msg) => {
    const r = reduce(get(), msg);
    if (r.data !== get()) set(r.data);
    return r.resync;
  },
  setConnection: (connection) => set({ connection }),
  dismissBanner: () => set({ banner: null }),
}));
