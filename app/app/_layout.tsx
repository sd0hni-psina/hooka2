import { Stack, useRouter, useSegments } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useEffect } from 'react';
import { StyleSheet, View } from 'react-native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { registerPush } from '../src/features/notifications/registerPush';
import { useAuthStore } from '../src/store/auth';
import { Spinner } from '../src/ui';
import { color } from '../src/ui/tokens';

// Auth guard, tech.md section 19.2: no session goes to join, a session goes to the hall.
function useAuthGuard() {
  const status = useAuthStore((s) => s.status);
  // Signed in but no hall picked yet: stay on join for the hall choice.
  const needsHall = useAuthStore((s) => s.hallId === null && (s.me?.halls.length ?? 0) > 0);
  const segments = useSegments();
  const router = useRouter();

  useEffect(() => {
    void useAuthStore.getState().bootstrap();
  }, []);

  useEffect(() => {
    if (status === 'loading') return;
    const first = segments[0];
    const inJoin = first === 'join';
    const inDevTools = first === 'kitchen-sink';
    if (status === 'signedOut' && !inJoin && !inDevTools) router.replace('/join');
    if (status === 'signedIn' && inJoin && !needsHall) router.replace('/');
    if (status === 'signedIn' && !inJoin && !inDevTools && needsHall) router.replace('/join');
  }, [status, needsHall, segments, router]);

  useEffect(() => {
    if (status !== 'signedIn') return;
    registerPush().catch(() => undefined);
  }, [status]);

  return status;
}

export default function RootLayout() {
  const status = useAuthGuard();
  return (
    <GestureHandlerRootView style={styles.root}>
      <SafeAreaProvider>
        <StatusBar style="light" />
        {status === 'loading' ? (
          <View style={styles.center}>
            <Spinner size="large" />
          </View>
        ) : (
          <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: color.bg } }}>
            <Stack.Screen name="(hall)" />
            <Stack.Screen name="join" />
            <Stack.Screen name="settings" />
            <Stack.Screen name="kitchen-sink" />
          </Stack>
        )}
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: color.bg },
  center: { flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: color.bg },
});
