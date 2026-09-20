import type { ReactNode } from 'react';
import { Modal, Pressable as RNPressable, ScrollView, StyleSheet, View } from 'react-native';
import Animated, { SlideInDown, SlideOutDown } from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { IconButton } from './IconButton';
import { Text } from './Text';
import { color, elevation, overlay, radius, space } from './tokens';
import { t } from '../i18n/ru';

type Props = { visible: boolean; onClose: () => void; title: string; children: ReactNode; footer?: ReactNode };

export function Sheet({ visible, onClose, title, children, footer }: Props) {
  const insets = useSafeAreaInsets();
  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose} statusBarTranslucent>
      <View style={styles.root}>
        <RNPressable accessibilityLabel={t.ui.close} style={styles.scrim} onPress={onClose} />
        <Animated.View
          entering={SlideInDown.duration(240)}
          exiting={SlideOutDown.duration(180)}
          style={[styles.sheet, { paddingBottom: insets.bottom + space[4] }]}
        >
          <View style={styles.grabber} />
          <View style={styles.header}>
            <View style={styles.title}>
              <Text variant="h2">{title}</Text>
            </View>
            <IconButton icon="close" accessibilityLabel={t.ui.close} onPress={onClose} />
          </View>
          <ScrollView contentContainerStyle={styles.body} keyboardShouldPersistTaps="handled">
            {children}
          </ScrollView>
          {footer ? <View style={styles.footer}>{footer}</View> : null}
        </Animated.View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, justifyContent: 'flex-end' },
  scrim: { ...StyleSheet.absoluteFill, backgroundColor: overlay },
  sheet: {
    maxHeight: '88%',
    backgroundColor: color.surface,
    borderTopLeftRadius: radius.xl,
    borderTopRightRadius: radius.xl,
    borderTopWidth: 1,
    borderColor: color.border,
    paddingHorizontal: space[4],
    paddingTop: space[2],
    ...elevation.sheet,
  },
  grabber: {
    alignSelf: 'center', width: 40, height: 4, borderRadius: radius.pill,
    backgroundColor: color.border, marginBottom: space[2],
  },
  header: { flexDirection: 'row', alignItems: 'center' },
  title: { flex: 1 },
  body: { gap: space[3], paddingBottom: space[4] },
  footer: { gap: space[3] },
});
