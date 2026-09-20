import { StyleSheet, View } from 'react-native';

import type { Feel } from './haptics';
import { Icon, type IconName } from './Icon';
import { Pressable } from './Pressable';
import { Spinner } from './Spinner';
import { Text } from './Text';
import { color, hit, radius, space, type ColorToken } from './tokens';

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost';

type Props = {
  title: string;
  onPress: () => void;
  variant: ButtonVariant;
  size: 'md' | 'lg';
  loading?: boolean;
  disabled?: boolean;
  icon?: IconName;
  haptic?: Feel | 'none';
};

const bg: Record<ButtonVariant, string | undefined> = {
  primary: color.accent,
  secondary: color.surfaceAlt,
  danger: color.danger,
  ghost: undefined,
};

const fg: Record<ButtonVariant, ColorToken> = { primary: 'bg', secondary: 'text', danger: 'bg', ghost: 'accent' };

const defaultFeel: Record<ButtonVariant, Feel> = {
  primary: 'success', secondary: 'tap', danger: 'warning', ghost: 'tap',
};

export function Button({ title, onPress, variant, size, loading, disabled, icon, haptic }: Props) {
  const inactive = disabled || loading;
  return (
    <Pressable
      accessibilityState={{ disabled: !!inactive, busy: !!loading }}
      disabled={inactive}
      haptic={inactive ? 'none' : (haptic ?? defaultFeel[variant])}
      onPress={onPress}
      style={[
        styles.base,
        size === 'lg' ? styles.lg : styles.md,
        { backgroundColor: bg[variant] },
        variant === 'secondary' && styles.bordered,
        inactive && styles.dim,
      ]}
    >
      <View style={styles.row}>
        {loading ? <Spinner /> : icon ? <Icon name={icon} size={20} color={fg[variant]} /> : null}
        <Text variant={size === 'lg' ? 'body' : 'label'} color={fg[variant]}>
          {title}
        </Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: { borderRadius: radius.md, alignItems: 'center', justifyContent: 'center', paddingHorizontal: space[4] },
  md: { minHeight: hit.min },
  lg: { minHeight: hit.min + space[3] },
  bordered: { borderWidth: 1, borderColor: color.border },
  dim: { opacity: 0.5 },
  row: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
});
