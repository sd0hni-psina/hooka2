import { StyleSheet, View } from 'react-native';

import { Button } from './Button';
import { Icon, type IconName } from './Icon';
import { Text } from './Text';
import { space } from './tokens';

type Props = { icon: IconName; title: string; message: string; actionTitle?: string; onAction?: () => void };

export function EmptyState({ icon, title, message, actionTitle, onAction }: Props) {
  return (
    <View style={styles.root}>
      <Icon name={icon} size={48} color="textMuted" />
      <Text variant="h2">{title}</Text>
      <Text variant="body" color="textMuted">
        {message}
      </Text>
      {actionTitle && onAction ? <Button title={actionTitle} variant="secondary" size="md" onPress={onAction} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  root: { alignItems: 'center', justifyContent: 'center', gap: space[3], padding: space[6] },
});
