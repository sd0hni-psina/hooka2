import { useLocalSearchParams, useRouter } from 'expo-router';
import { useMemo, useState } from 'react';
import { StyleSheet, View } from 'react-native';

import { finishHookah, transferHookah } from '../../../src/api/endpoints';
import { explain } from '../../../src/api/errors';
import type { FinishReason, ServiceEvent } from '../../../src/api/types';
import { useStartHookah } from '../../../src/features/hookah/useStartHookah';
import { useTaskActions } from '../../../src/features/tasks/useTaskActions';
import { t } from '../../../src/i18n/ru';
import { useHallStore } from '../../../src/store/hall';
import { hookahByTable, progressAt, statusesAt } from '../../../src/store/selectors';
import {
  AppBar, Badge, Button, Card, ConfirmDialog, Countdown, EmptyState, Icon, IconButton, ListRow, ProgressRing,
  Screen, Sheet, Spinner, Stat, Text, statusStyle, toast, useServerNow,
} from '../../../src/ui';
import { space } from '../../../src/ui/tokens';

const reasons: FinishReason[] = ['guest_left', 'replaced', 'mistake', 'shift_end'];

type EventRowProps = {
  event: ServiceEvent;
  pending: boolean;
  disabled: boolean;
  onClaim: () => void;
  onConfirm: () => void;
};

function EventRow({ event, pending, disabled, onClaim, onConfirm }: EventRowProps) {
  const open = event.state === 'due' || event.state === 'pending';
  const who = event.claimed_by_name
    ? t.tasks.claimedBy(event.claimed_by_name)
    : event.confirmed_by_name
      ? t.tasks.confirmedBy(event.confirmed_by_name)
      : t.table.eventState[event.state];
  return (
    <ListRow
      title={event.title}
      subtitle={who}
      left={
        <Icon
          name={event.state === 'confirmed' ? 'check' : event.state === 'due' ? 'alert' : 'clock'}
          color={event.state === 'confirmed' ? 'green' : event.state === 'due' ? 'yellow' : 'textMuted'}
        />
      }
      right={
        <View style={styles.actions}>
          {open ? <Countdown target={event.due_at} mode={event.state === 'due' ? 'up' : 'down'} variant="label" /> : null}
          {pending ? <Spinner /> : null}
          {!pending && open && event.state === 'due' && !event.claimed_by ? (
            <IconButton icon="hand" variant="filled" accessibilityLabel={t.tasks.claim} disabled={disabled} onPress={onClaim} haptic="select" />
          ) : null}
          {!pending && open ? (
            <IconButton icon="check" variant="accent" accessibilityLabel={t.tasks.confirm} disabled={disabled} onPress={onConfirm} haptic="success" />
          ) : null}
          {!open ? (
            <Badge
              title={event.confirmed_early ? t.table.early : t.table.eventState[event.state]}
              tone={event.state === 'confirmed' ? 'green' : 'neutral'}
            />
          ) : null}
        </View>
      }
    />
  );
}

export default function TableScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const state = useHallStore((s) => s.state);
  const pending = useHallStore((s) => s.pending);
  const connection = useHallStore((s) => s.connection);
  const stale = useHallStore((s) => s.stale);
  const now = useServerNow();
  const actions = useTaskActions();
  const { start, starting } = useStartHookah();
  const frozen = connection !== 'online' || stale;

  const data = useMemo(() => ({ state, pending }), [state, pending]);
  const table = state?.tables.find((x) => x.id === id) ?? null;
  const hookah = useMemo(() => (id ? (hookahByTable(data).get(id) ?? null) : null), [data, id]);
  const status = statusesAt(data, now)[id ?? ''] ?? 'free';
  const progress = progressAt(data, now)[id ?? ''] ?? 0;

  const [busy, setBusy] = useState(false);
  const [confirmStart, setConfirmStart] = useState(false);
  const [showTransfer, setShowTransfer] = useState(false);
  const [showFinish, setShowFinish] = useState(false);
  const [reason, setReason] = useState<FinishReason>('guest_left');

  const freeTables = useMemo(() => {
    const busyTables = hookahByTable(data);
    return (state?.tables ?? []).filter((x) => x.id !== id && !busyTables.has(x.id));
  }, [data, state, id]);

  const run = async (fn: () => Promise<unknown>, after?: () => void) => {
    setBusy(true);
    try {
      await fn();
      after?.();
    } catch (e) {
      toast({ key: 'table', message: explain(e), tone: 'danger' });
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
            <Button title={t.table.transfer} variant="secondary" size="lg" icon="swap" loading={busy} disabled={frozen} onPress={() => setShowTransfer(true)} />
            <Button title={t.table.finish} variant="danger" size="md" icon="stop" loading={busy} disabled={frozen} onPress={() => setShowFinish(true)} />
          </>
        ) : (
          <Button title={t.table.start} variant="primary" size="lg" icon="flame" loading={busy || starting} disabled={frozen} onPress={() => setConfirmStart(true)} />
        )
      }
    >
      <AppBar
        title={t.table.title(table.label)}
        subtitle={t.table.seats(table.seats)}
        left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />}
        right={<Badge title={statusStyle[status].label} tone={status === 'free' ? 'neutral' : status} icon={statusStyle[status].icon} />}
      />

      {hookah ? (
        <>
          <Card status={status}>
            <View style={styles.hero}>
              <ProgressRing progress={progress} status={status} size={132} thickness={8}>
                <Countdown target={hookah.started_at} mode="up" status={status} variant="mono" />
                <Text variant="label" color="textMuted">
                  {t.table.elapsed}
                </Text>
              </ProgressRing>
              <View style={styles.stats}>
                <Stat label={t.table.leftToExpire} hint={t.table.startedBy(hookah.started_by_name)}>
                  <Countdown target={hookah.expires_at} mode="down" variant="mono" />
                </Stat>
              </View>
            </View>
          </Card>

          <Text variant="label" color="textMuted">
            {t.table.events}
          </Text>
          <View>
            {hookah.events
              .filter((e) => e.state !== 'cancelled')
              .map((e) => (
                <EventRow
                  key={e.id}
                  event={e}
                  pending={pending[e.id] !== undefined}
                  disabled={frozen}
                  onClaim={() => actions.claim(e.id)}
                  onConfirm={() => actions.confirm(e.id)}
                />
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
          void start(table);
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
              subtitle={t.table.seats(x.seats)}
              right={<Icon name="chevron-right" color="textMuted" />}
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
              selected={r === reason}
              right={r === reason ? <Icon name="check" color="green" /> : undefined}
              onPress={() => setReason(r)}
            />
          ))}
        </View>
      </Sheet>
    </Screen>
  );
}

const styles = StyleSheet.create({
  hero: { flexDirection: 'row', alignItems: 'center', gap: space[4] },
  stats: { flex: 1, gap: space[1] },
  reasons: { gap: space[1] },
  actions: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
});
