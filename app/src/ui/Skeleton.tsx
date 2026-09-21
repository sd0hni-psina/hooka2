import { useEffect } from 'react';
import Animated, { useAnimatedStyle, useSharedValue, withRepeat, withTiming } from 'react-native-reanimated';

import { duration, timing } from './motion';
import { color, radius } from './tokens';

type Props = { width?: number | `${number}%`; height: number; radius?: keyof typeof radius };

// A skeleton is the honest shape of what is loading. It replaces the spinner
// that used to hold the hall screen on a cold start.
export function Skeleton({ width = '100%', height, radius: r = 'md' }: Props) {
  const fade = useSharedValue(0.5);
  useEffect(() => {
    fade.value = withRepeat(withTiming(1, { ...timing.slow, duration: duration.pulse }), -1, true);
  }, [fade]);
  const animated = useAnimatedStyle(() => ({ opacity: fade.value }));
  return (
    <Animated.View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      style={[{ width, height, borderRadius: radius[r], backgroundColor: color.surfaceAlt }, animated]}
    />
  );
}
