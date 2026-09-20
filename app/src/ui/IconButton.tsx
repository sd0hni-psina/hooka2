import { StyleSheet } from 'react-native';

import type { Feel } from './haptics';
import { Icon, type IconName } from './Icon';
import { Pressable } from './Pressable';
import { color, hit, radius, type ColorToken } from './tokens';

type Props = {
  icon: IconName;
  onPress: () => void;
  variant?: 'plain' | 'filled' | 'accent';
  accessibilityLabel: string;
  disabled?: boolean;
  tint?: ColorToken;
  haptic?: Feel | 'none';
};

export function IconButton({ icon, onPress, variant = 'plain', accessibilityLabel, disabled, tint, haptic }: Props) {
  return (
    <Pressable
      accessibilityLabel={accessibilityLabel}
      disabled={disabled}
      haptic={disabled ? 'none' : (haptic ?? 'tap')}
      hitSlop={4}
      onPress={onPress}
      style={[
        styles.base,
        variant === 'filled' && styles.filled,
        variant === 'accent' && styles.accent,
        disabled && styles.dim,
      ]}
    >
      <Icon name={icon} color={tint ?? (variant === 'accent' ? 'bg' : 'text')} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: { width: hit.min, height: hit.min, borderRadius: radius.pill, alignItems: 'center', justifyContent: 'center' },
  filled: { backgroundColor: color.surfaceAlt, borderWidth: 1, borderColor: color.border },
  accent: { backgroundColor: color.accent },
  dim: { opacity: 0.4 },
});
