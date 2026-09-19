import { Pressable, StyleSheet, View } from 'react-native';

import { Icon, type IconName } from './Icon';
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
};

const bg: Record<ButtonVariant, string | undefined> = {
  primary: color.accent,
  secondary: color.surfaceAlt,
  danger: color.danger,
  ghost: undefined,
};

const fg: Record<ButtonVariant, ColorToken> = { primary: 'text', secondary: 'text', danger: 'text', ghost: 'accent' };

export function Button({ title, onPress, variant, size, loading, disabled, icon }: Props) {
  const inactive = disabled || loading;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: !!inactive, busy: !!loading }}
      disabled={inactive}
      onPress={onPress}
      style={({ pressed }) => [
        styles.base,
        size === 'lg' ? styles.lg : styles.md,
        { backgroundColor: bg[variant] },
        variant === 'secondary' && styles.bordered,
        (pressed || inactive) && styles.dim,
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
  dim: { opacity: 0.6 },
  row: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
});
