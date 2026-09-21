import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import Svg, { Circle } from 'react-native-svg';

import type { TableStatus } from '../api/types';
import { color, tone } from './tokens';

type Props = {
  // progress is the share of the hookah resource already burned, 0..1.
  progress: number;
  status: TableStatus;
  size: number;
  thickness?: number;
  children?: ReactNode;
};

// ProgressRing answers the one question a master asks about a working table:
// how much of this hookah is left.
export function ProgressRing({ progress, status, size, thickness = 6, children }: Props) {
  const clamped = Math.min(1, Math.max(0, progress));
  const r = (size - thickness) / 2;
  const circumference = 2 * Math.PI * r;
  return (
    <View style={{ width: size, height: size }}>
      <Svg width={size} height={size}>
        <Circle cx={size / 2} cy={size / 2} r={r} stroke={color.border} strokeWidth={thickness} fill="none" />
        <Circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke={color[tone[status].fg]}
          strokeWidth={thickness}
          strokeLinecap="round"
          strokeDasharray={`${circumference * clamped} ${circumference}`}
          // Start at twelve o'clock instead of three.
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
          fill="none"
        />
      </Svg>
      {children ? <View style={[StyleSheet.absoluteFill, styles.center]}>{children}</View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  center: { alignItems: 'center', justifyContent: 'center' },
});
