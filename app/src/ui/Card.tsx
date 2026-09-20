import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';

import type { TableStatus } from '../api/types';
import { Pressable } from './Pressable';
import { color, radius, space, tone } from './tokens';

type Props = { children: ReactNode; onPress?: () => void; onLongPress?: () => void; status?: TableStatus; compact?: boolean };

export function Card({ children, onPress, onLongPress, status, compact }: Props) {
  const toned = status && status !== 'free' ? { borderColor: tone[status].ring, backgroundColor: tone[status].fill } : null;
  const inner = <View style={compact ? styles.innerCompact : styles.inner}>{children}</View>;
  if (!onPress && !onLongPress) return <View style={[styles.card, toned]}>{inner}</View>;
  return (
    <Pressable onPress={onPress} onLongPress={onLongPress} style={[styles.card, toned]}>
      {inner}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { backgroundColor: color.surface, borderRadius: radius.lg, borderWidth: 1, borderColor: color.border, overflow: 'hidden' },
  inner: { padding: space[4], gap: space[2] },
  innerCompact: { padding: space[3], gap: space[1] },
});
