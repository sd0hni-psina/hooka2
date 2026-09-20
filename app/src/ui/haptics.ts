import * as Haptics from 'expo-haptics';

// Haptics replace a glance at the screen: the master holds the phone in one
// hand and looks at the hall, tech.md section 19.15. Every call is
// fire-and-forget, an unsupported device just stays silent.
export type Feel = 'tap' | 'select' | 'success' | 'warning' | 'error';

const run: Record<Feel, () => Promise<void>> = {
  tap: () => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light),
  select: () => Haptics.selectionAsync(),
  success: () => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success),
  warning: () => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning),
  error: () => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error),
};

export function feel(kind: Feel): void {
  void run[kind]().catch(() => undefined);
}
