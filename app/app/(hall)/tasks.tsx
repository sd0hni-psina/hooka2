import { useRouter } from 'expo-router';
import { useMemo } from 'react';
import { StyleSheet, View } from 'react-native';

import { claimEvent, confirmEvent } from '../../src/api/endpoints';
import { formatDuration } from '../../src/domain/countdown';
import { t } from '../../src/i18n/ru';
import { useHallStore } from '../../src/store/hall';
import { selectTasks } from '../../src/store/selectors';
import { AppBar, Badge, EmptyState, IconButton, ListRow, Screen, useServerNow } from '../../src/ui';
import { space } from '../../src/ui/tokens';

// The tasks screen is for a master in motion: the most urgent task on top.
export default function TasksScreen() {
  const router = useRouter();
  const data = useHallStore((s) => s);
  const now = useServerNow();
  const tasks = useMemo(() => selectTasks(data, now), [data, now]);

  return (
    <Screen scroll>
      <AppBar title={t.tasks.title} subtitle={data.state?.hall.name} />
      {tasks.length === 0 ? (
        <EmptyState icon="list" title={t.tasks.empty} message={t.tasks.emptyHint} />
      ) : (
        <View>
          {tasks.map((task) => (
            <ListRow
              key={task.event.id}
              title={`${t.table.title(task.table.label)}: ${task.event.title}`}
              subtitle={task.event.claimed_by_name ? t.tasks.claimedBy(task.event.claimed_by_name) : undefined}
              right={
                <View style={styles.actions}>
                  <Badge
                    title={`${task.overdueMs >= 0 ? t.tasks.overdue : t.tasks.soon} ${formatDuration(Math.abs(task.overdueMs))}`}
                    tone={task.overdueMs >= 0 ? 'red' : 'yellow'}
                    icon="clock"
                  />
                  {task.event.state === 'due' && !task.event.claimed_by ? (
                    <IconButton
                      icon="hand"
                      variant="filled"
                      accessibilityLabel={t.tasks.claim}
                      onPress={() => void claimEvent(task.event.id).catch(() => undefined)}
                    />
                  ) : null}
                  <IconButton
                    icon="check"
                    variant="filled"
                    accessibilityLabel={t.tasks.confirm}
                    onPress={() => void confirmEvent(task.event.id).catch(() => undefined)}
                  />
                </View>
              }
              onPress={() => router.push({ pathname: '/table/[id]', params: { id: task.table.id } })}
            />
          ))}
        </View>
      )}
    </Screen>
  );
}

const styles = StyleSheet.create({
  actions: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
});
