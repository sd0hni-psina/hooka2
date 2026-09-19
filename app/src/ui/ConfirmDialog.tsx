import { Modal, StyleSheet, View } from 'react-native';

import { Button } from './Button';
import { Text } from './Text';
import { color, overlay, radius, space } from './tokens';
import { t } from '../i18n/ru';

type Props = {
  visible: boolean;
  title: string;
  message: string;
  confirmTitle: string;
  onConfirm: () => void;
  onCancel: () => void;
  destructive?: boolean;
};

export function ConfirmDialog({ visible, title, message, confirmTitle, onConfirm, onCancel, destructive }: Props) {
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onCancel}>
      <View style={styles.root}>
        <View style={styles.dialog} accessibilityRole="alert">
          <Text variant="h2">{title}</Text>
          <Text variant="body" color="textMuted">
            {message}
          </Text>
          <View style={styles.actions}>
            <Button title={confirmTitle} variant={destructive ? 'danger' : 'primary'} size="lg" onPress={onConfirm} />
            <Button title={t.ui.cancel} variant="ghost" size="md" onPress={onCancel} />
          </View>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: overlay, justifyContent: 'center', padding: space[5] },
  dialog: { backgroundColor: color.surface, borderRadius: radius.lg, padding: space[5], gap: space[3] },
  actions: { gap: space[2], marginTop: space[2] },
});
