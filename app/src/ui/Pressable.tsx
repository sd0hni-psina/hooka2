import { useState, type ReactNode } from 'react';
import {
  Animated, Pressable as RNPressable, type AccessibilityRole, type StyleProp, type ViewStyle,
} from 'react-native';

import { feel, type Feel } from './haptics';
import { duration, pressScale } from './motion';

type Props = {
  children: ReactNode;
  onPress?: () => void;
  onLongPress?: () => void;
  disabled?: boolean;
  style?: StyleProp<ViewStyle>;
  accessibilityRole?: AccessibilityRole;
  accessibilityLabel?: string;
  accessibilityState?: { disabled?: boolean; busy?: boolean; selected?: boolean };
  // haptic fires on press; 'none' keeps a purely visual control quiet.
  haptic?: Feel | 'none';
  hitSlop?: number;
};

// Touch is the whole interface here: the master looks at the hall, not at the
// screen. Every pressable answers with a scale and a tap in the hand,
// tech.md section 19.15. The scale runs on the native driver, so a busy hall
// never makes a button feel late.
export function Pressable({
  children, onPress, onLongPress, disabled, style, accessibilityRole = 'button',
  accessibilityLabel, accessibilityState, haptic = 'tap', hitSlop,
}: Props) {
  // useState holds the value: a ref may not be read while rendering.
  const [scale] = useState(() => new Animated.Value(1));

  const squeeze = (to: number, ms: number) =>
    Animated.timing(scale, { toValue: to, duration: ms, useNativeDriver: true }).start();

  return (
    <RNPressable
      accessibilityRole={accessibilityRole}
      accessibilityLabel={accessibilityLabel}
      accessibilityState={accessibilityState ?? { disabled: !!disabled }}
      disabled={disabled}
      hitSlop={hitSlop}
      onPressIn={() => squeeze(pressScale, duration.fast)}
      onPressOut={() => squeeze(1, duration.base)}
      onPress={() => {
        if (haptic !== 'none') feel(haptic);
        onPress?.();
      }}
      onLongPress={onLongPress ? () => {
        feel('select');
        onLongPress();
      } : undefined}
    >
      <Animated.View style={[style, { transform: [{ scale }] }]}>{children}</Animated.View>
    </RNPressable>
  );
}
