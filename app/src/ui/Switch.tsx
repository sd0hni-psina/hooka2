import { StyleSheet } from 'react-native';
import Animated, { useAnimatedStyle, withSpring } from 'react-native-reanimated';

import { Pressable } from './Pressable';
import { spring } from './motion';
import { color, radius } from './tokens';

type Props = { value: boolean; onChange: (v: boolean) => void; accessibilityLabel: string };

const TRACK_W = 52;
const TRACK_H = 32;
const KNOB = 26;

export function Switch({ value, onChange, accessibilityLabel }: Props) {
  const knob = useAnimatedStyle(() => ({
    transform: [{ translateX: withSpring(value ? TRACK_W - KNOB - 3 : 3, spring) }],
  }));
  return (
    <Pressable
      accessibilityRole="switch"
      accessibilityLabel={accessibilityLabel}
      accessibilityState={{ selected: value }}
      haptic="select"
      onPress={() => onChange(!value)}
      style={[styles.track, value && styles.on]}
    >
      <Animated.View style={[styles.knob, knob]} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  track: {
    width: TRACK_W, height: TRACK_H, borderRadius: radius.pill,
    backgroundColor: color.surfaceHigh, borderWidth: 1, borderColor: color.border, justifyContent: 'center',
  },
  on: { backgroundColor: color.accent, borderColor: color.accent },
  knob: { width: KNOB, height: KNOB, borderRadius: radius.pill, backgroundColor: color.text },
});
