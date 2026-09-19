import { Pressable, StyleSheet } from 'react-native';

import { Icon, type IconName } from './Icon';
import { color, hit, radius } from './tokens';

type Props = {
  icon: IconName;
  onPress: () => void;
  variant?: 'plain' | 'filled';
  accessibilityLabel: string;
};

export function IconButton({ icon, onPress, variant = 'plain', accessibilityLabel }: Props) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      onPress={onPress}
      hitSlop={4}
      style={({ pressed }) => [styles.base, variant === 'filled' && styles.filled, pressed && styles.pressed]}
    >
      <Icon name={icon} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  base: { width: hit.min, height: hit.min, borderRadius: radius.pill, alignItems: 'center', justifyContent: 'center' },
  filled: { backgroundColor: color.surfaceAlt },
  pressed: { opacity: 0.6 },
});
