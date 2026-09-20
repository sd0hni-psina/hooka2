import { useRouter } from 'expo-router';
import { useMemo } from 'react';
import { SectionList, StyleSheet, View } from 'react-native';

import { TaskRow } from '../../src/features/tasks/TaskRow';
import { useTaskActions } from '../../src/features/tasks/useTaskActions';
import { t } from '../../src/i18n/ru';
import { useHallStore } from '../../src/store/hall';
import { selectTasks, type Task } from '../../src/store/selectors';
import { AppBar, Badge, EmptyState, Screen, Text, useServerNow } from '../../src/ui';
import { space } from '../../src/ui/tokens';

// The tasks screen is for a master in motion: the most urgent task on top.
export default function TasksScreen() {
  const router = useRouter();
  const state = useHallStore((s) => s.state);
  const pending = useHallStore((s) => s.pending);
  const now = useServerNow();
  const actions = useTaskActions();

  const data = useMemo(() => ({ state, pending }), [state, pending]);
  const tasks = useMemo(() => selectTasks(data, now), [data, now]);

  const sections = useMemo(() => {
    const overdue = tasks.filter((task) => task.overdueMs >= 0);
    const soon = tasks.filter((task) => task.overdueMs < 0);
    return [
      { title: t.tasks.groupOverdue, tone: 'red' as const, data: overdue },
      { title: t.tasks.groupSoon, tone: 'yellow' as const, data: soon },
    ].filter((s) => s.data.length > 0);
  }, [tasks]);

  return (
    <Screen>
      <AppBar title={t.tasks.title} subtitle={state?.hall.name} />
      {tasks.length === 0 ? (
        <EmptyState icon="list" title={t.tasks.empty} message={t.tasks.emptyHint} />
      ) : (
        <SectionList<Task, { title: string; tone: 'red' | 'yellow' }>
          sections={sections}
          keyExtractor={(task) => task.event.id}
          stickySectionHeadersEnabled={false}
          initialNumToRender={10}
          windowSize={7}
          contentContainerStyle={styles.list}
          renderSectionHeader={({ section }) => (
            <View style={styles.header}>
              <Text variant="label" color="textMuted">
                {section.title}
              </Text>
              <Badge title={String(section.data.length)} tone={section.tone} />
            </View>
          )}
          renderItem={({ item }) => (
            <TaskRow
              task={item}
              variant="row"
              onOpen={() => router.push({ pathname: '/table/[id]', params: { id: item.table.id } })}
              onClaim={() => actions.claim(item.event.id)}
              onConfirm={() => actions.confirm(item.event.id)}
            />
          )}
        />
      )}
    </Screen>
  );
}

const styles = StyleSheet.create({
  list: { paddingBottom: space[5] },
  header: { flexDirection: 'row', alignItems: 'center', gap: space[2], paddingTop: space[4], paddingBottom: space[2] },
});
