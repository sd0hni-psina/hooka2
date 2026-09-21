import { StyleSheet, View } from 'react-native';

import { Icon, type IconName } from './Icon';
import { Text } from './Text';
import { alpha, color, radius, space, type ColorToken } from './tokens';

export type BadgeTone = 'neutral' | 'green' | 'yellow' | 'red' | 'accent';

type Props = { title: string; tone: BadgeTone; icon?: IconName; solid?: boolean };

const toneColor: Record<BadgeTone, ColorToken> = {
  neutral: 'textMuted', green: 'green', yellow: 'yellow', red: 'red', accent: 'accent',
};

export function Badge({ title, tone, icon, solid }: Props) {
  const c = toneColor[tone];
  return (
    <View style={[styles.badge, { borderColor: alpha(c, 0.4), backgroundColor: solid ? color[c] : alpha(c, 0.14) }]}>
      {icon ? <Icon name={icon} size={14} color={solid ? 'bg' : c} /> : null}
      <Text variant="label" color={solid ? 'bg' : c} numberOfLines={1}>
        {title}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  badge: {
    flexDirection: 'row',
    alignItems: 'center',
    alignSelf: 'flex-start',
    gap: space[1],
    paddingHorizontal: space[2],
    paddingVertical: space[1],
    borderRadius: radius.pill,
    borderWidth: 1,
  },
});
