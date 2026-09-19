import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMemo, useState } from 'react';
import { StyleSheet, View } from 'react-native';

import * as Crypto from 'expo-crypto';

import { finishHookah, startHookah, transferHookah } from '../../../src/api/endpoints';
import type { FinishReason, ServiceEvent } from '../../../src/api/types';
import { describeError } from '../../../src/features/join/form';
import { t } from '../../../src/i18n/ru';
import { useHallStore } from '../../../src/store/hall';
import { hookahByTable, statusesAt } from '../../../src/store/selectors';
import {
  AppBar, Badge, Button, Card, ConfirmDialog, Countdown, EmptyState, IconButton, ListRow, Screen,
  Sheet, Text, Toast, statusStyle, useServerNow,
} from '../../../src/ui';
import { space } from '../../../src/ui/tokens';

const reasons: FinishReason[] = ['guest_left', 'replaced', 'mistake', 'shift_end'];

function EventRow({ event }: { event: ServiceEvent }) {
  const tone = event.state === 'due' ? 'red' : event.state === 'pending' ? 'neutral' : 'green';
  return (
    <ListRow
      title={event.title}
      subtitle={t.table.eventState[event.state]}
      right={
        event.state === 'pending' || event.state === 'due' ? (
          <Countdown target={event.due_at} mode={event.state === 'due' ? 'up' : 'down'} />
        ) : (
          <Badge title={t.table.eventState[event.state]} tone={tone} />
        )
      }
    />
  );
}

export default function TableScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const data = useHallStore((s) => s);
  const now = useServerNow();

  const table = data.state?.tables.find((x) => x.id === id) ?? null;
  const hookah = useMemo(() => (id ? (hookahByTable(data).get(id) ?? null) : null), [data, id]);
  const status = statusesAt(data, now)[id ?? ''] ?? 'free';

  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmStart, setConfirmStart] = useState(false);
  const [showTransfer, setShowTransfer] = useState(false);
  const [showFinish, setShowFinish] = useState(false);
  const [reason, setReason] = useState<FinishReason>('guest_left');

  const freeTables = useMemo(() => {
    const busyTables = hookahByTable(data);
    return (data.state?.tables ?? []).filter((x) => x.id !== id && !busyTables.has(x.id));
  }, [data, id]);

  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    setMessage(null);
    try {
      await fn();
      after?.();
    } catch (e) {
      const d = describeError(e);
      setMessage(d.message ?? Object.values(d.fields)[0] ?? t.errors.unknown);
    } finally {
      setBusy(false);
    }
  };

  if (!table) {
    return (
      <Screen>
        <AppBar title={t.table.notFound} left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />} />
        <EmptyState icon="grid" title={t.table.notFound} message={t.hall.loading} />
      </Screen>
    );
  }

  return (
    <Screen
      scroll
      footer={
        hookah ? (
          <>
            {message ? <Toast message={message} tone="danger" /> : null}
            <Button title={t.table.transfer} variant="secondary" size="lg" icon="swap" loading={busy} onPress={() => setShowTransfer(true)} />
            <Button title={t.table.finish} variant="danger" size="md" icon="stop" loading={busy} onPress={() => setShowFinish(true)} />
          </>
        ) : (
          <>
            {message ? <Toast message={message} tone="danger" /> : null}
            <Button title={t.table.start} variant="primary" size="lg" icon="flame" loading={busy} onPress={() => setConfirmStart(true)} />
          </>
        )
      }
    >
      <AppBar
        title={t.table.title(table.label)}
        subtitle={statusStyle[status].label}
        left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />}
        right={<Badge title={statusStyle[status].label} tone={status === 'free' ? 'neutral' : status} icon={statusStyle[status].icon} />}
      />

      {hookah ? (
        <>
          <Card status={status}>
            <Text variant="label" color="textMuted">
              {t.table.elapsed}
            </Text>
            <Countdown target={hookah.started_at} mode="up" status={status} />
            <Text variant="label" color="textMuted">
              {t.table.leftToExpire}
            </Text>
            <Countdown target={hookah.expires_at} mode="down" />
            <Text variant="label" color="textMuted">
              {hookah.started_by_name}
            </Text>
          </Card>
          <Text variant="label" color="textMuted">
            {t.table.events}
          </Text>
          <View>
            {hookah.events
              .filter((e) => e.state !== 'cancelled')
              .map((e) => (
                <EventRow key={e.id} event={e} />
              ))}
          </View>
        </>
      ) : (
        <EmptyState icon="flame" title={t.table.free} message={t.table.startMessage(table.label)} />
      )}

      <ConfirmDialog
        visible={confirmStart}
        title={t.table.startConfirm}
        message={t.table.startMessage(table.label)}
        confirmTitle={t.table.start}
        onCancel={() => setConfirmStart(false)}
        onConfirm={() => {
          setConfirmStart(false);
          const key = Crypto.randomUUID();
          void run(() => startHookah(table.id, key));
        }}
      />

      <Sheet visible={showTransfer} onClose={() => setShowTransfer(false)} title={t.table.transferTitle}>
        {freeTables.length === 0 ? (
          <Text variant="body" color="textMuted">
            {t.table.transferEmpty}
          </Text>
        ) : (
          freeTables.map((x) => (
            <ListRow
              key={x.id}
              title={t.table.title(x.label)}
              onPress={() => {
                setShowTransfer(false);
                if (!hookah) return;
                void run(
                  () => transferHookah(hookah.id, x.id),
                  () => router.replace({ pathname: '/table/[id]', params: { id: x.id } }),
                );
              }}
            />
          ))
        )}
      </Sheet>

      <Sheet
        visible={showFinish}
        onClose={() => setShowFinish(false)}
        title={t.table.finishTitle}
        footer={
          <Button
            title={t.table.finish}
            variant="danger"
            size="lg"
            loading={busy}
            onPress={() => {
              setShowFinish(false);
              if (!hookah) return;
              void run(() => finishHookah(hookah.id, reason), () => router.back());
            }}
          />
        }
      >
        <Text variant="body" color="textMuted">
          {t.table.finishMessage}
        </Text>
        <View style={styles.reasons}>
          {reasons.map((r) => (
            <ListRow
              key={r}
              title={t.table.reason[r]}
              right={r === reason ? <Badge title={t.table.eventState.confirmed} tone="green" icon="check" /> : undefined}
              onPress={() => setReason(r)}
            />
          ))}
        </View>
      </Sheet>
    </Screen>
  );
}

const styles = StyleSheet.create({
  reasons: { gap: space[1] },
});
