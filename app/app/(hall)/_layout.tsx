import { Tabs } from 'expo-router';
import { useMemo } from 'react';

import { useHallConnection } from '../../src/features/hookah/useHallConnection';
import { t } from '../../src/i18n/ru';
import { useAuthStore } from '../../src/store/auth';
import { useHallStore } from '../../src/store/hall';
import { countTasks } from '../../src/store/selectors';
import { Icon, useServerNow } from '../../src/ui';
import { color } from '../../src/ui/tokens';

export default function HallLayout() {
  const hallId = useAuthStore((s) => s.hallId ?? s.me?.halls[0]?.id ?? null);
  useHallConnection(hallId);

  // The badge is the only thing a master needs from the tab bar: how many
  // tables are waiting while he looks at the map.
  const state = useHallStore((s) => s.state);
  const pending = useHallStore((s) => s.pending);
  const now = useServerNow();
  const counts = useMemo(() => countTasks({ state, pending }, now), [state, pending, now]);
  const badge = counts.due + counts.soon;

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
        options={{
          title: t.tabs.tasks,
          tabBarBadge: badge > 0 ? badge : undefined,
          tabBarBadgeStyle: { backgroundColor: counts.due > 0 ? color.red : color.yellow, color: color.bg },
          tabBarIcon: ({ focused }) => <Icon name="list" color={focused ? 'text' : 'textMuted'} />,
        }}
      />
      <Tabs.Screen name="table/[id]" options={{ href: null }} />
    </Tabs>
  );
}
