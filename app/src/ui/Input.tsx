import { StyleSheet, TextInput, View, type KeyboardTypeOptions } from 'react-native';

import { Text } from './Text';
import { color, font, hit, radius, space } from './tokens';

type Props = {
  value: string;
  onChangeText: (v: string) => void;
  label?: string;
  placeholder?: string;
  error?: string;
  keyboardType?: KeyboardTypeOptions;
};

export function Input({ value, onChangeText, label, placeholder, error, keyboardType }: Props) {
  return (
    <View style={styles.wrap}>
      {label ? (
        <Text variant="label" color="textMuted">
          {label}
        </Text>
      ) : null}
      <TextInput
        accessibilityLabel={label}
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
        placeholderTextColor={color.textMuted}
        keyboardType={keyboardType}
        style={[styles.input, !!error && styles.invalid]}
      />
      {error ? (
        <Text variant="label" color="danger">
          {error}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: space[1] },
  input: {
    minHeight: hit.min,
    paddingHorizontal: space[3],
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: color.border,
    backgroundColor: color.surface,
    color: color.text,
    fontSize: font.body.size,
  },
  invalid: { borderColor: color.danger },
});
