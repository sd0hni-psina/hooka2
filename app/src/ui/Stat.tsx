import { StyleSheet, View } from 'react-native';

import type { TableStatus } from '../api/types';
import { Text, type TextVariant } from './Text';
import { space, tone } from './tokens';

type Props = {
  label: string;
  value: string;
  status?: TableStatus;
  hint?: string;
  variant?: Extract<TextVariant, 'display' | 'mono'>;
};

// Stat is a label over a number. The table screen is mostly made of these.
export function Stat({ label, value, status, hint, variant = 'mono' }: Props) {
  return (
    <View style={styles.root}>
      <Text variant="label" color="textMuted">
        {label}
      </Text>
      <Text variant={variant} color={status ? tone[status].fg : 'text'}>
        {value}
      </Text>
      {hint ? (
        <Text variant="label" color="textMuted">
          {hint}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  root: { gap: space[1] },
});
