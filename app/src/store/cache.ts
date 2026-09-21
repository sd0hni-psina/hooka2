import AsyncStorage from '@react-native-async-storage/async-storage';

import { decodeHallState, obj, str } from '../api/decode';
import type { HallState } from '../api/types';

// Cache of the last snapshot, tech.md section 19.15. It exists for one
// moment: the app opens and the master sees the hall before the socket says
// a word.
const PREFIX = 'hall.snapshot.v1.';
const MAX_AGE_MS = 12 * 60 * 60 * 1000;
const MIN_WRITE_GAP_MS = 5_000;

export type Cached = { state: HallState; savedAt: string };

export type SnapshotStorage = {
  getItem: (key: string) => Promise<string | null>;
  setItem: (key: string, value: string) => Promise<void>;
  removeItem: (key: string) => Promise<void>;
};

let storage: SnapshotStorage = AsyncStorage;
const lastWrite: Record<string, number> = {};

export async function loadSnapshot(hallId: string, nowMs: number): Promise<Cached | null> {
  try {
    const raw = await storage.getItem(PREFIX + hallId);
    if (!raw) return null;
    const box = obj(JSON.parse(raw) as unknown, 'cache');
    const savedAt = str(box, 'saved_at', 'cache');
    const age = nowMs - Date.parse(savedAt);
    if (!Number.isFinite(age) || age > MAX_AGE_MS || age < 0) throw new Error('snapshot out of date');
    return { state: decodeHallState(box.state), savedAt };
  } catch {
    // A cache this build cannot read is not worth repairing: the socket is
    // about to deliver the real thing.
    await storage.removeItem(PREFIX + hallId).catch(() => undefined);
    return null;
  }
}

export async function saveSnapshot(hallId: string, state: HallState, nowMs: number): Promise<void> {
  if (nowMs - (lastWrite[hallId] ?? -Infinity) < MIN_WRITE_GAP_MS) return;
  lastWrite[hallId] = nowMs;
  try {
    await storage.setItem(PREFIX + hallId, JSON.stringify({ saved_at: new Date(nowMs).toISOString(), state }));
  } catch {
    // A full disk must not break a running shift.
  }
}

export async function dropSnapshots(hallIds: string[]): Promise<void> {
  await Promise.all(hallIds.map((id) => storage.removeItem(PREFIX + id).catch(() => undefined)));
  for (const id of hallIds) delete lastWrite[id];
}

export function __setSnapshotStorageForTests(s: SnapshotStorage): void {
  storage = s;
  for (const k of Object.keys(lastWrite)) delete lastWrite[k];
}
