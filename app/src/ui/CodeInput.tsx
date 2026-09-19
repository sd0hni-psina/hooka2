import { useRef } from 'react';
import { Pressable, StyleSheet, TextInput, View } from 'react-native';

import { Text } from './Text';
import { color, hit, radius, space } from './tokens';
import { t } from '../i18n/ru';

type Props = { length: 4; value: string; onChange: (v: string) => void; autoFocus?: boolean };

// One hidden input drives the cells, so paste and backspace behave natively.
export function CodeInput({ length, value, onChange, autoFocus }: Props) {
  const ref = useRef<TextInput>(null);
  const cells = Array.from({ length }, (_, i) => value[i] ?? '');
  return (
    <Pressable accessibilityLabel={t.ui.code} onPress={() => ref.current?.focus()} style={styles.row}>
      {cells.map((c, i) => (
        <View key={i} style={[styles.cell, i === value.length && styles.active]}>
          <Text variant="h1">{c}</Text>
        </View>
      ))}
      <TextInput
        ref={ref}
        value={value}
        onChangeText={(v) => onChange(v.replace(/\D/g, '').slice(0, length))}
        keyboardType="number-pad"
        maxLength={length}
        autoFocus={autoFocus}
        caretHidden
        style={styles.hidden}
      />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', gap: space[3], justifyContent: 'center' },
  cell: {
    width: hit.min + space[3],
    height: hit.min + space[5],
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: color.border,
    backgroundColor: color.surface,
    alignItems: 'center',
    justifyContent: 'center',
  },
  active: { borderColor: color.accent },
  hidden: { position: 'absolute', opacity: 0, width: 1, height: 1 },
});
