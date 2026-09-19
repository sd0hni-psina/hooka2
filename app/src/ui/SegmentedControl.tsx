import { Pressable, StyleSheet, View } from 'react-native';

import { Text } from './Text';
import { color, hit, radius, space } from './tokens';

export type SegmentOption<T extends string> = { value: T; label: string };

type Props<T extends string> = { options: SegmentOption<T>[]; value: T; onChange: (v: T) => void };

export function SegmentedControl<T extends string>({ options, value, onChange }: Props<T>) {
  return (
    <View style={styles.row} accessibilityRole="tablist">
      {options.map((o) => {
        const active = o.value === value;
        return (
          <Pressable
            key={o.value}
            accessibilityRole="tab"
            accessibilityState={{ selected: active }}
            onPress={() => onChange(o.value)}
            style={[styles.segment, active && styles.active]}
          >
            <Text variant="label" color={active ? 'text' : 'textMuted'}>
              {o.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', backgroundColor: color.surface, borderRadius: radius.md, padding: space[1], gap: space[1] },
  segment: { flex: 1, minHeight: hit.min - space[2], alignItems: 'center', justifyContent: 'center', borderRadius: radius.sm },
  active: { backgroundColor: color.surfaceAlt },
});
