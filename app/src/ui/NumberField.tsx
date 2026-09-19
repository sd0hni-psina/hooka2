import { StyleSheet, View } from 'react-native';

import { IconButton } from './IconButton';
import { Text } from './Text';
import { color, radius, space } from './tokens';
import { t } from '../i18n/ru';

type Props = {
  value: number;
  onChange: (v: number) => void;
  min: number;
  max: number;
  step: number;
  suffix?: string;
};

export function NumberField({ value, onChange, min, max, step, suffix }: Props) {
  const clamp = (v: number) => Math.min(max, Math.max(min, v));
  return (
    <View style={styles.row}>
      <IconButton icon="minus" variant="filled" accessibilityLabel={t.ui.decrease} onPress={() => onChange(clamp(value - step))} />
      <View style={styles.value}>
        <Text variant="mono">
          {value}
          {suffix ? ` ${suffix}` : ''}
        </Text>
      </View>
      <IconButton icon="plus" variant="filled" accessibilityLabel={t.ui.increase} onPress={() => onChange(clamp(value + step))} />
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
  value: {
    flex: 1,
    alignItems: 'center',
    paddingVertical: space[2],
    borderRadius: radius.md,
    backgroundColor: color.surface,
    borderWidth: 1,
    borderColor: color.border,
  },
});
