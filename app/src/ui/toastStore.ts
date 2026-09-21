import { create } from 'zustand';

export type ToastTone = 'neutral' | 'success' | 'warning' | 'danger';

export type ToastInput = {
  message: string;
  tone: ToastTone;
  // key collapses repeats: the same failure retried ten times shows once.
  key?: string;
  actionTitle?: string;
  onAction?: () => void;
  ttlMs?: number;
};

export type ToastItem = {
  id: string;
  message: string;
  tone: ToastTone;
  actionTitle?: string;
  onAction?: () => void;
};

const MAX_VISIBLE = 3;
const DEFAULT_TTL_MS = 4_000;

type ToastStore = {
  items: ToastItem[];
  show: (input: ToastInput) => void;
  dismiss: (id: string) => void;
  clear: () => void;
};

const timers = new Map<string, ReturnType<typeof setTimeout>>();

// Messages float above the screen instead of pushing the layout around, the
// way the inline toasts of v1 did, tech.md section 19.15.
export const useToastStore = create<ToastStore>((set, get) => ({
  items: [],
  show: (input) => {
    const id = input.key ?? input.message;
    const existing = timers.get(id);
    if (existing) clearTimeout(existing);
    const item: ToastItem = {
      id,
      message: input.message,
      tone: input.tone,
      actionTitle: input.actionTitle,
      onAction: input.onAction,
    };
    set({ items: [...get().items.filter((i) => i.id !== id), item].slice(-MAX_VISIBLE) });
    timers.set(id, setTimeout(() => get().dismiss(id), input.ttlMs ?? DEFAULT_TTL_MS));
  },
  dismiss: (id) => {
    const timer = timers.get(id);
    if (timer) clearTimeout(timer);
    timers.delete(id);
    set({ items: get().items.filter((i) => i.id !== id) });
  },
  clear: () => {
    for (const timer of timers.values()) clearTimeout(timer);
    timers.clear();
    set({ items: [] });
  },
}));

// toast is callable outside React, so a store or an api layer can speak too.
export function toast(input: ToastInput): void {
  useToastStore.getState().show(input);
}
