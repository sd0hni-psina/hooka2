import { useRouter } from 'expo-router';
import { useMemo, useState } from 'react';
import { StyleSheet, View } from 'react-native';

import { ApiError, NetworkError } from '../../src/api/errors';
import { t } from '../../src/i18n/ru';
import { useAuthStore } from '../../src/store/auth';
import { useHallStore } from '../../src/store/hall';
import { statusesAt } from '../../src/store/selectors';
import { AppBar, Badge, Button, Card, ConfirmDialog, EmptyState, HallCanvas, IconButton, Screen, Spinner, Text, useServerNow } from '../../src/ui';
import { space } from '../../src/ui/tokens';
import { clockOffset, isClockSynced } from '../../src/ws/clock';

function ConnectionBadge() {
  const connection = useHallStore((s) => s.connection);
  if (connection === 'online') return <Badge title={t.hall.online} tone="green" icon="check" />;
  if (connection === 'connecting' || connection === 'idle') return <Badge title={t.hall.connecting} tone="neutral" icon="refresh" />;
  return <Badge title={t.hall.offline} tone="red" icon="wifi-off" />;
}

export default function HallScreen() {
  const router = useRouter();
  const me = useAuthStore((s) => s.me);
  const error = useAuthStore((s) => s.error);
  const data = useHallStore((s) => s);
  const now = useServerNow();
  const statuses = useMemo(() => statusesAt(data, now), [data, now]);
  const [confirmLogout, setConfirmLogout] = useState(false);

  if (!me) {
    const message = error instanceof NetworkError ? t.errors.network : error instanceof ApiError ? error.message : t.me.loadFailed;
    return (
      <Screen>
        <EmptyState icon="wifi-off" title={t.me.loadFailed} message={message} actionTitle={t.ui.retry} onAction={() => void useAuthStore.getState().refreshMe()} />
      </Screen>
    );
  }

  const hall = me.halls.find((h) => h.id === data.hallId);
  const manager = me.staff.role !== 'master';

  return (
    <Screen scroll>
      <AppBar
        title={hall?.name ?? t.appName}
        subtitle={me.venue.name}
        right={manager ? <IconButton icon="settings" accessibilityLabel={t.settings.title} onPress={() => router.push('/settings')} /> : undefined}
      />
      <Card>
        <Text variant="h2">{me.staff.display_name}</Text>
        <Text variant="label" color="textMuted">
          {t.me.role[me.staff.role]}
        </Text>
        {me.venue.join_code ? <Text variant="label">{t.me.joinCode(me.venue.join_code)}</Text> : null}
        <View style={styles.row}>
          <ConnectionBadge />
          {isClockSynced() ? <Badge title={t.hall.clockOffset(Math.round(clockOffset() / 1000))} tone="neutral" icon="clock" /> : null}
          {data.seq !== null ? <Badge title={t.hall.seq(data.seq)} tone="neutral" /> : null}
        </View>
        <Button title={t.logout.action} variant="ghost" size="md" icon="close" onPress={() => setConfirmLogout(true)} />
      </Card>
      <ConfirmDialog
        visible={confirmLogout}
        title={t.logout.title}
        message={t.logout.message}
        confirmTitle={t.logout.action}
        destructive
        onConfirm={() => {
          setConfirmLogout(false);
          void useAuthStore.getState().signOut();
        }}
        onCancel={() => setConfirmLogout(false)}
      />
      {me.halls.length === 0 ? (
        <EmptyState icon="grid" title={t.hall.noHalls} message={t.hall.noHallsHint} />
      ) : data.state ? (
        <HallCanvas
          tables={data.state.tables}
          statuses={statuses}
          onTablePress={(table) => router.push({ pathname: '/table/[id]', params: { id: table.id } })}
        />
      ) : (
        <View style={styles.center}>
          <Spinner size="large" />
          <Text variant="label" color="textMuted">
            {t.hall.loading}
          </Text>
        </View>
      )}
    </Screen>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: space[2] },
  center: { alignItems: 'center', gap: space[3], padding: space[6] },
});
