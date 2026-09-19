import { Pressable, StyleSheet, View } from 'react-native';

import { Icon, type IconName } from './Icon';
import { Text } from './Text';
import { color, radius, space, type ColorToken } from './tokens';

export type ToastTone = 'neutral' | 'success' | 'warning' | 'danger';

type Props = { message: string; tone: ToastTone; actionTitle?: string; onAction?: () => void };

const toneStyle: Record<ToastTone, { color: ColorToken; icon: IconName }> = {
  neutral: { color: 'accent', icon: 'info' },
  success: { color: 'green', icon: 'check' },
  warning: { color: 'yellow', icon: 'clock' },
  danger: { color: 'red', icon: 'alert' },
};

export function Toast({ message, tone, actionTitle, onAction }: Props) {
  const s = toneStyle[tone];
  return (
    <View accessibilityRole="alert" style={[styles.toast, { borderLeftColor: color[s.color] }]}>
      <Icon name={s.icon} color={s.color} />
      <View style={styles.message}>
        <Text variant="body">{message}</Text>
      </View>
      {actionTitle && onAction ? (
        <Pressable accessibilityRole="button" onPress={onAction} style={styles.action}>
          <Text variant="label" color="accent">
            {actionTitle}
          </Text>
        </Pressable>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  toast: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: space[3],
    padding: space[3],
    borderRadius: radius.md,
    backgroundColor: color.surfaceAlt,
    borderLeftWidth: space[1],
  },
  message: { flex: 1 },
  action: { padding: space[2] },
});
