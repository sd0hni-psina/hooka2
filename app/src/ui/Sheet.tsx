import type { ReactNode } from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { IconButton } from './IconButton';
import { Text } from './Text';
import { color, overlay, radius, space } from './tokens';
import { t } from '../i18n/ru';

type Props = { visible: boolean; onClose: () => void; title: string; children: ReactNode; footer?: ReactNode };

export function Sheet({ visible, onClose, title, children, footer }: Props) {
  const insets = useSafeAreaInsets();
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <View style={styles.root}>
        <Pressable accessibilityLabel={t.ui.close} style={styles.scrim} onPress={onClose} />
        <View style={[styles.sheet, { paddingBottom: insets.bottom + space[4] }]}>
          <View style={styles.header}>
            <View style={styles.title}>
              <Text variant="h2">{title}</Text>
            </View>
            <IconButton icon="close" accessibilityLabel={t.ui.close} onPress={onClose} />
          </View>
          <ScrollView contentContainerStyle={styles.body}>{children}</ScrollView>
          {footer ? <View style={styles.footer}>{footer}</View> : null}
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, justifyContent: 'flex-end' },
  scrim: { ...StyleSheet.absoluteFill, backgroundColor: overlay },
  sheet: {
    maxHeight: '85%',
    backgroundColor: color.surface,
    borderTopLeftRadius: radius.lg,
    borderTopRightRadius: radius.lg,
    paddingHorizontal: space[4],
    paddingTop: space[2],
  },
  header: { flexDirection: 'row', alignItems: 'center' },
  title: { flex: 1 },
  body: { gap: space[3], paddingBottom: space[4] },
  footer: { gap: space[3] },
});
