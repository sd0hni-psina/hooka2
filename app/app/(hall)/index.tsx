import { useRouter } from 'expo-router';
import { useMemo, useState } from 'react';
import { ScrollView, StyleSheet, View } from 'react-native';

import * as Crypto from 'expo-crypto';

import { startHookah } from '../../src/api/endpoints';
import type { Table } from '../../src/api/types';
import { formatDuration } from '../../src/domain/countdown';
import { describeError } from '../../src/features/join/form';
import { t } from '../../src/i18n/ru';
import { useAuthStore } from '../../src/store/auth';
import { useHallStore } from '../../src/store/hall';
import { hookahByTable, selectTasks, statusesAt } from '../../src/store/selectors';
import {
  AppBar, Badge, Card, ConfirmDialog, EmptyState, HallCanvas, IconButton, Screen, Spinner, Text, Toast, useServerNow,
} from '../../src/ui';
import { space } from '../../src/ui/tokens';

function ConnectionBadge() {
  const connection = useHallStore((s) => s.connection);
  if (connection === 'online') return <Badge title={t.hall.online} tone="green" icon="check" />;
  if (connection === 'offline') return <Badge title={t.hall.offline} tone="red" icon="wifi-off" />;
  return <Badge title={t.hall.connecting} tone="neutral" icon="refresh" />;
}

export default function HallScreen() {
  const router = useRouter();
  const me = useAuthStore((s) => s.me);
  const data = useHallStore((s) => s);
  const now = useServerNow();
  const [pending, setPending] = useState<{ table: Table; key: string } | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const statuses = useMemo(() => statusesAt(data, now), [data, now]);
  const byTable = useMemo(() => hookahByTable(data), [data]);
  const tasks = useMemo(() => selectTasks(data, now), [data, now]);
  // Every live table shows how long its hookah has been running.
  const badges = useMemo(() => {
    const out: Record<string, string> = {};
    for (const [tableId, h] of byTable) out[tableId] = formatDuration(now - Date.parse(h.started_at));
    return out;
  }, [byTable, now]);

  const openTable = (table: Table) => {
    const hookah = byTable.get(table.id);
    if (hookah) {
      router.push({ pathname: '/table/[id]', params: { id: table.id } });
      return;
    }
    setMessage(null);
    setPending({ table, key: Crypto.randomUUID() });
  };

  const confirmStart = async () => {
    if (!pending) return;
    const { table, key } = pending;
    setPending(null);
    try {
      await startHookah(table.id, key);
    } catch (e) {
      const d = describeError(e);
      setMessage(d.message ?? Object.values(d.fields)[0] ?? t.errors.unknown);
    }
  };

  const hall = me?.halls.find((h) => h.id === data.hallId);
  const manager = me?.staff.role !== 'master';

  return (
    <Screen scroll>
      <AppBar
        title={hall?.name ?? t.appName}
        subtitle={me?.venue.name}
        left={<ConnectionBadge />}
        right={manager ? <IconButton icon="settings" accessibilityLabel={t.settings.title} onPress={() => router.push('/settings')} /> : undefined}
      />
      {message ? <Toast message={message} tone="danger" /> : null}

      {tasks.length > 0 ? (
        <View style={styles.strip}>
          <Text variant="label" color="textMuted">
            {t.tasks.strip}
          </Text>
          <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.stripRow}>
            {tasks.slice(0, 6).map((task) => (
              <View key={task.event.id} style={styles.taskCard}>
                <Card status={statuses[task.table.id]} onPress={() => router.push({ pathname: '/table/[id]', params: { id: task.table.id } })}>
                  <Text variant="h2">{t.table.title(task.table.label)}</Text>
                  <Text variant="label" color="textMuted" numberOfLines={2}>
                    {task.event.title}
                  </Text>
                  <Badge
                    title={`${task.overdueMs >= 0 ? t.tasks.overdue : t.tasks.soon} ${formatDuration(Math.abs(task.overdueMs))}`}
                    tone={task.overdueMs >= 0 ? 'red' : 'yellow'}
                    icon="clock"
                  />
                </Card>
              </View>
            ))}
          </ScrollView>
        </View>
      ) : null}

      {data.state ? (
        <HallCanvas tables={data.state.tables} statuses={statuses} badges={badges} onTablePress={openTable} />
      ) : me && me.halls.length === 0 ? (
        <EmptyState icon="grid" title={t.hall.noHalls} message={t.hall.noHallsHint} />
      ) : (
        <View style={styles.center}>
          <Spinner size="large" />
          <Text variant="label" color="textMuted">
            {t.hall.loading}
          </Text>
        </View>
      )}

      <ConfirmDialog
        visible={pending !== null}
        title={t.table.startConfirm}
        message={pending ? t.table.startMessage(pending.table.label) : ''}
        confirmTitle={t.table.start}
        onCancel={() => setPending(null)}
        onConfirm={() => void confirmStart()}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  strip: { gap: space[2] },
  stripRow: { gap: space[3], paddingRight: space[4] },
  taskCard: { width: 220 },
  center: { alignItems: 'center', gap: space[3], padding: space[6] },
});
