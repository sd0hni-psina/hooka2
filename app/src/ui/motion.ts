import { Easing, ReduceMotion, type WithSpringConfig, type WithTimingConfig } from 'react-native-reanimated';

// Durations and curves of tech.md section 19.15. Every animated primitive
// reads them, so the app moves at one speed.
export const duration = { fast: 120, base: 200, slow: 320, pulse: 1200 } as const;

export const timing: Record<'fast' | 'base' | 'slow', WithTimingConfig> = {
  fast: { duration: duration.fast, easing: Easing.out(Easing.cubic), reduceMotion: ReduceMotion.System },
  base: { duration: duration.base, easing: Easing.out(Easing.cubic), reduceMotion: ReduceMotion.System },
  slow: { duration: duration.slow, easing: Easing.out(Easing.cubic), reduceMotion: ReduceMotion.System },
};

export const spring: WithSpringConfig = {
  damping: 18,
  stiffness: 220,
  mass: 0.6,
  reduceMotion: ReduceMotion.System,
};

// Scale a pressable settles on while a finger holds it.
export const pressScale = 0.97;
