import { Tabs } from 'expo-router';

import { useHallConnection } from '../../src/features/hookah/useHallConnection';
import { t } from '../../src/i18n/ru';
import { useAuthStore } from '../../src/store/auth';
import { Icon } from '../../src/ui';
import { color } from '../../src/ui/tokens';

export default function HallLayout() {
  // Slice 0 opens the first hall; the hall picker arrives with join.
  const hallId = useAuthStore((s) => s.me?.halls[0]?.id ?? null);
  useHallConnection(hallId);

  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarStyle: { backgroundColor: color.surface, borderTopColor: color.border },
        tabBarActiveTintColor: color.text,
        tabBarInactiveTintColor: color.textMuted,
        sceneStyle: { backgroundColor: color.bg },
      }}
    >
      <Tabs.Screen
        name="index"
        options={{ title: t.tabs.hall, tabBarIcon: ({ focused }) => <Icon name="grid" color={focused ? 'text' : 'textMuted'} /> }}
      />
      <Tabs.Screen
        name="tasks"
        options={{ title: t.tabs.tasks, tabBarIcon: ({ focused }) => <Icon name="list" color={focused ? 'text' : 'textMuted'} /> }}
      />
      <Tabs.Screen name="table/[id]" options={{ href: null }} />
    </Tabs>
  );
}
