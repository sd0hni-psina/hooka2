import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';

import { Text } from './Text';
import { hit, space } from './tokens';

type Props = { title: string; subtitle?: string; left?: ReactNode; right?: ReactNode };

export function AppBar({ title, subtitle, left, right }: Props) {
  return (
    <View style={styles.bar}>
      {left ? <View style={styles.side}>{left}</View> : null}
      <View style={styles.titles}>
        <Text variant="h2" numberOfLines={1}>
          {title}
        </Text>
        {subtitle ? (
          <Text variant="label" color="textMuted" numberOfLines={1}>
            {subtitle}
          </Text>
        ) : null}
      </View>
      {right ? <View style={styles.side}>{right}</View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  bar: { flexDirection: 'row', alignItems: 'center', minHeight: hit.min, gap: space[2] },
  titles: { flex: 1 },
  side: { flexDirection: 'row', alignItems: 'center' },
});
