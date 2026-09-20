import { memo } from 'react';
import { StyleSheet, View } from 'react-native';

import { formatDuration } from '../../domain/countdown';
import { t } from '../../i18n/ru';
import type { Task } from '../../store/selectors';
import { Badge, Card, IconButton, ListRow, Spinner, Text } from '../../ui';
import { space } from '../../ui/tokens';

type Props = {
  task: Task;
  onOpen: () => void;
  onClaim: () => void;
  onConfirm: () => void;
  variant: 'row' | 'card';
};

function urgency(task: Task) {
  return {
    title: `${task.overdueMs >= 0 ? t.tasks.overdue : t.tasks.soon} ${formatDuration(Math.abs(task.overdueMs))}`,
    tone: task.overdueMs >= 0 ? ('red' as const) : ('yellow' as const),
  };
}

// Actions sit in the row itself: a master in motion must not open a screen to
// say Done, tech.md section 19.10.
function Actions({ task, onClaim, onConfirm }: Pick<Props, 'task' | 'onClaim' | 'onConfirm'>) {
  if (task.pending) {
    return (
      <View style={styles.actions}>
        <Spinner />
      </View>
    );
  }
  return (
    <View style={styles.actions}>
      {task.event.state === 'due' && !task.event.claimed_by ? (
        <IconButton icon="hand" variant="filled" accessibilityLabel={t.tasks.claim} onPress={onClaim} haptic="select" />
      ) : null}
      <IconButton icon="check" variant="accent" accessibilityLabel={t.tasks.confirm} onPress={onConfirm} haptic="success" />
    </View>
  );
}

function TaskRowView({ task, onOpen, onClaim, onConfirm, variant }: Props) {
  const mark = urgency(task);
  const who = task.event.claimed_by_name ? t.tasks.claimedBy(task.event.claimed_by_name) : undefined;

  if (variant === 'card') {
    return (
      <View style={styles.card}>
        <Card status={task.overdueMs >= 0 ? 'red' : 'yellow'} onPress={onOpen} compact>
          <View style={styles.cardHead}>
            <Text variant="h2">{t.table.title(task.table.label)}</Text>
            <Badge title={mark.title} tone={mark.tone} icon="clock" />
          </View>
          <Text variant="label" color="textMuted" numberOfLines={2}>
            {task.event.title}
          </Text>
          <View style={styles.cardFoot}>
            <View style={styles.who}>
              {who ? (
                <Text variant="label" color="textMuted" numberOfLines={1}>
                  {who}
                </Text>
              ) : null}
            </View>
            <Actions task={task} onClaim={onClaim} onConfirm={onConfirm} />
          </View>
        </Card>
      </View>
    );
  }

  return (
    <ListRow
      title={`${t.table.title(task.table.label)}: ${task.event.title}`}
      subtitle={who}
      onPress={onOpen}
      right={
        <View style={styles.rowRight}>
          <Badge title={mark.title} tone={mark.tone} icon="clock" />
          <Actions task={task} onClaim={onClaim} onConfirm={onConfirm} />
        </View>
      }
    />
  );
}

export const TaskRow = memo(TaskRowView);

const styles = StyleSheet.create({
  actions: { flexDirection: 'row', alignItems: 'center', gap: space[2], minWidth: 48, justifyContent: 'flex-end' },
  card: { width: 250 },
  cardHead: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: space[2] },
  cardFoot: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: space[2] },
  who: { flex: 1 },
  rowRight: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
});
