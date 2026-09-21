import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';

import { Pressable } from './Pressable';
import { Text } from './Text';
import { color, hit, radius, space } from './tokens';

type Props = {
  title: string;
  subtitle?: string;
  right?: ReactNode;
  left?: ReactNode;
  onPress?: () => void;
  onLongPress?: () => void;
  selected?: boolean;
};

export function ListRow({ title, subtitle, right, left, onPress, onLongPress, selected }: Props) {
  const body = (
    <View style={[styles.row, selected && styles.selected]}>
      {left}
      <View style={styles.text}>
        <Text variant="body" numberOfLines={1}>
          {title}
        </Text>
        {subtitle ? (
          <Text variant="label" color="textMuted" numberOfLines={2}>
            {subtitle}
          </Text>
        ) : null}
      </View>
      {right}
    </View>
  );
  if (!onPress && !onLongPress) return body;
  return (
    <Pressable onPress={onPress} onLongPress={onLongPress} accessibilityState={{ selected }}>
      {body}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: space[3],
    minHeight: hit.min + space[2],
    paddingVertical: space[2],
    paddingHorizontal: space[2],
    borderRadius: radius.md,
    borderBottomWidth: 1,
    borderBottomColor: color.border,
  },
  selected: { backgroundColor: color.surfaceAlt, borderBottomColor: 'transparent' },
  text: { flex: 1, gap: space[1] },
});
