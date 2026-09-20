import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';

import type { TableStatus } from '../api/types';
import { Text } from './Text';
import { space, tone } from './tokens';

type Props = {
  label: string;
  // value renders as a number; children take over when the value is live,
  // like a Countdown that owns its own ticker.
  value?: string;
  children?: ReactNode;
  status?: TableStatus;
  hint?: string;
};

// Stat is a label over a number. The table screen is mostly made of these.
export function Stat({ label, value, children, status, hint }: Props) {
  return (
    <View style={styles.root}>
      <Text variant="label" color="textMuted">
        {label}
      </Text>
      {children ?? (
        <Text variant="mono" color={status ? tone[status].fg : 'text'}>
          {value ?? ''}
        </Text>
      )}
      {hint ? (
        <Text variant="label" color="textMuted" numberOfLines={1}>
          {hint}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  root: { gap: space[1] },
});
