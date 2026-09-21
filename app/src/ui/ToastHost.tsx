import { StyleSheet, View } from 'react-native';
import Animated, { FadeInUp, FadeOutUp, LinearTransition } from 'react-native-reanimated';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { Toast } from './Toast';
import { space } from './tokens';
import { useToastStore } from './toastStore';

// Mounted once in the root layout. Screens call toast() and forget.
export function ToastHost() {
  const items = useToastStore((s) => s.items);
  const dismiss = useToastStore((s) => s.dismiss);
  const insets = useSafeAreaInsets();
  if (items.length === 0) return null;
  return (
    <View pointerEvents="box-none" style={[styles.host, { paddingTop: insets.top + space[2] }]}>
      {items.map((item) => (
        <Animated.View
          key={item.id}
          entering={FadeInUp.duration(200)}
          exiting={FadeOutUp.duration(160)}
          layout={LinearTransition}
        >
          <Toast
            message={item.message}
            tone={item.tone}
            actionTitle={item.actionTitle}
            onAction={item.onAction ? () => {
              dismiss(item.id);
              item.onAction?.();
            } : undefined}
          />
        </Animated.View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  host: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    paddingHorizontal: space[4],
    gap: space[2],
    zIndex: 100,
  },
});
