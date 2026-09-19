import type { ReactNode } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';

import type { TableStatus } from '../api/types';
import { statusStyle } from './status';
import { color, radius, space } from './tokens';

type Props = { children: ReactNode; onPress?: () => void; status?: TableStatus };

export function Card({ children, onPress, status }: Props) {
  const stripe = status ? { borderLeftWidth: space[1], borderLeftColor: color[statusStyle[status].color] } : null;
  const content = <View style={styles.inner}>{children}</View>;
  if (!onPress) return <View style={[styles.card, stripe]}>{content}</View>;
  return (
    <Pressable accessibilityRole="button" onPress={onPress} style={({ pressed }) => [styles.card, stripe, pressed && styles.pressed]}>
      {content}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { backgroundColor: color.surface, borderRadius: radius.md, borderWidth: 1, borderColor: color.border, overflow: 'hidden' },
  inner: { padding: space[4], gap: space[2] },
  pressed: { backgroundColor: color.surfaceAlt },
});
