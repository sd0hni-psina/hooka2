import type { ReactNode } from 'react';
import { Pressable as RNPressable, type AccessibilityRole, type StyleProp, type ViewStyle } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withSpring, withTiming } from 'react-native-reanimated';

import { feel, type Feel } from './haptics';
import { pressScale, spring, timing } from './motion';

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
// tech.md section 19.15.
export function Pressable({
  children, onPress, onLongPress, disabled, style, accessibilityRole = 'button',
  accessibilityLabel, accessibilityState, haptic = 'tap', hitSlop,
}: Props) {
  const scale = useSharedValue(1);
  const animated = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));

  return (
    <RNPressable
      accessibilityRole={accessibilityRole}
      accessibilityLabel={accessibilityLabel}
      accessibilityState={accessibilityState ?? { disabled: !!disabled }}
      disabled={disabled}
      hitSlop={hitSlop}
      onPressIn={() => {
        scale.value = withTiming(pressScale, timing.fast);
      }}
      onPressOut={() => {
        scale.value = withSpring(1, spring);
      }}
      onPress={() => {
        if (haptic !== 'none') feel(haptic);
        onPress?.();
      }}
      onLongPress={onLongPress ? () => {
        feel('select');
        onLongPress();
      } : undefined}
    >
      <Animated.View style={[style, animated]}>{children}</Animated.View>
    </RNPressable>
  );
}
