import { useRouter } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { FlatList, StyleSheet, View } from 'react-native';

import { claimEvent } from '../../src/api/endpoints';
import type { Table } from '../../src/api/types';
import { ConnectionBar } from '../../src/features/hall/ConnectionBar';
import { ProfileSheet } from '../../src/features/hall/ProfileSheet';
import { TableList } from '../../src/features/hall/TableList';
import { TableQuickSheet } from '../../src/features/hall/TableQuickSheet';
import { useStartHookah } from '../../src/features/hookah/useStartHookah';
import { TaskRow } from '../../src/features/tasks/TaskRow';
import { useTaskActions } from '../../src/features/tasks/useTaskActions';
import { t } from '../../src/i18n/ru';
import { useAuthStore } from '../../src/store/auth';
import { useHallStore } from '../../src/store/hall';
import { hookahByTable, progressAt, selectTasks, statusesAt, tableLabels } from '../../src/store/selectors';
import {
  AppBar, Badge, ConfirmDialog, EmptyState, HallCanvas, IconButton, Screen, SearchField, Skeleton, Text, toast,
  useServerNow,
} from '../../src/ui';
import { space } from '../../src/ui/tokens';

type Mode = 'map' | 'list';

export default function HallScreen() {
  const router = useRouter();
  const me = useAuthStore((s) => s.me);
  const state = useHallStore((s) => s.state);
  const pending = useHallStore((s) => s.pending);
  const banner = useHallStore((s) => s.banner);
  const connection = useHallStore((s) => s.connection);
  const stale = useHallStore((s) => s.stale);
  const actions = useTaskActions();
  const { start } = useStartHookah();

  const second = useServerNow();
  const minute = useServerNow('minute');

  const [mode, setMode] = useState<Mode>('map');
  const [query, setQuery] = useState('');
  const [searching, setSearching] = useState(false);
  const [starting, setStarting] = useState<Table | null>(null);
  const [quick, setQuick] = useState<Table | null>(null);
  const [showProfile, setShowProfile] = useState(false);

  // Cached data may lie, so it blocks mutations exactly like a dead socket,
  // tech.md section 19.15.
  const frozen = connection !== 'online' || stale;

  const data = useMemo(() => ({ state, pending }), [state, pending]);
  const statuses = useMemo(() => statusesAt(data, second), [data, second]);
  const byTable = useMemo(() => hookahByTable(data), [data]);
  const labels = useMemo(() => tableLabels(data, minute), [data, minute]);
  const progress = useMemo(() => progressAt(data, minute), [data, minute]);
  const tasks = useMemo(() => selectTasks(data, second), [data, second]);
  const tables = useMemo(() => state?.tables ?? [], [state]);

  const urgentIds = useMemo(
    () => new Set(Object.entries(statuses).filter(([, s]) => s === 'red').map(([id]) => id)),
    [statuses],
  );

  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return null;
    return tables.filter((table) => table.label.toLowerCase().includes(q));
  }, [query, tables]);

  const dimIds = useMemo(() => {
    if (!matches) return undefined;
    const keep = new Set(matches.map((table) => table.id));
    return new Set(tables.filter((table) => !keep.has(table.id)).map((table) => table.id));
  }, [matches, tables]);

  // A single match is what the master meant: the canvas flies to it.
  const focusId = matches?.length === 1 ? (matches[0]?.id ?? null) : null;

  // The fired task speaks once, over the screen, and offers to take itself.
  useEffect(() => {
    if (!banner) return;
    toast({
      key: banner.eventId,
      message: t.tasks.banner(banner.tableLabel, banner.title),
      tone: 'warning',
      ttlMs: 8000,
      actionTitle: t.tasks.claim,
      onAction: () => void claimEvent(banner.eventId).catch(() => undefined),
    });
    useHallStore.getState().dismissBanner();
  }, [banner]);

  const openTable = (table: Table) => {
    const hookah = byTable.get(table.id);
    if (hookah) {
      router.push({ pathname: '/table/[id]', params: { id: table.id } });
      return;
    }
    if (frozen) return;
    setStarting(table);
  };

  const confirmStart = () => {
    if (!starting) return;
    const table = starting;
    setStarting(null);
    void start(table);
  };

  const hall = me?.halls.find((h) => h.id === state?.hall.id) ?? null;
  const busy = byTable.size;

  return (
    <Screen>
      <AppBar
        title={hall?.name ?? state?.hall.name ?? t.appName}
        subtitle={tables.length > 0 ? t.hall.counters(busy, tables.length) : me?.venue.name}
        left={<IconButton icon="user" accessibilityLabel={t.profile.title} onPress={() => setShowProfile(true)} />}
        right={
          <View style={styles.bar}>
            <IconButton
              icon="search"
              variant={searching ? 'filled' : 'plain'}
              accessibilityLabel={t.hall.search}
              onPress={() => {
                setSearching((v) => !v);
                setQuery('');
              }}
            />
            <IconButton
              icon={mode === 'map' ? 'list' : 'map'}
              accessibilityLabel={mode === 'map' ? t.hall.modeList : t.hall.modeMap}
              onPress={() => setMode((m) => (m === 'map' ? 'list' : 'map'))}
            />
          </View>
        }
      />

      <ConnectionBar />

      {searching ? (
        <SearchField value={query} onChange={setQuery} placeholder={t.hall.search} keyboardType="number-pad" autoFocus />
      ) : null}

      {tasks.length > 0 ? (
        <View style={styles.strip}>
          <View style={styles.stripHead}>
            <Text variant="label" color="textMuted">
              {t.tasks.strip}
            </Text>
            <Badge title={String(tasks.length)} tone={tasks[0] && tasks[0].overdueMs >= 0 ? 'red' : 'yellow'} />
          </View>
          <FlatList
            horizontal
            data={tasks.slice(0, 8)}
            keyExtractor={(task) => task.event.id}
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.stripRow}
            renderItem={({ item }) => (
              <TaskRow
                task={item}
                variant="card"
                onOpen={() => router.push({ pathname: '/table/[id]', params: { id: item.table.id } })}
                onClaim={() => actions.claim(item.event.id)}
                onConfirm={() => actions.confirm(item.event.id)}
              />
            )}
          />
        </View>
      ) : null}

      {state ? (
        mode === 'map' ? (
          <HallCanvas
            fill
            tables={tables}
            statuses={statuses}
            labels={labels}
            progress={progress}
            urgentIds={urgentIds}
            dimIds={dimIds}
            focusId={focusId}
            onTablePress={openTable}
            onTableLongPress={setQuick}
          />
        ) : (
          <View style={styles.fill}>
            <TableList
              tables={matches ?? tables}
              statuses={statuses}
              hookahs={byTable}
              onPress={openTable}
              onLongPress={setQuick}
            />
          </View>
        )
      ) : me && me.halls.length === 0 ? (
        <View style={styles.fill}>
          <EmptyState icon="grid" title={t.hall.noHalls} message={t.hall.noHallsHint} />
        </View>
      ) : (
        <View style={styles.loading}>
          <Skeleton height={64} />
          <Skeleton height={64} />
          <Skeleton height={220} radius="lg" />
          <Text variant="label" color="textMuted" align="center">
            {t.hall.loading}
          </Text>
        </View>
      )}

      <ProfileSheet visible={showProfile} onClose={() => setShowProfile(false)} />

      <TableQuickSheet
        table={quick}
        hookah={quick ? (byTable.get(quick.id) ?? null) : null}
        status={quick ? (statuses[quick.id] ?? 'free') : 'free'}
        disabled={frozen}
        onClose={() => setQuick(null)}
        onStart={setStarting}
      />

      <ConfirmDialog
        visible={starting !== null}
        title={t.table.startConfirm}
        message={starting ? t.table.startMessage(starting.label) : ''}
        confirmTitle={t.table.start}
        onCancel={() => setStarting(null)}
        onConfirm={confirmStart}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  bar: { flexDirection: 'row', alignItems: 'center' },
  strip: { gap: space[2] },
  stripHead: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
  stripRow: { gap: space[3], paddingRight: space[4] },
  loading: { flex: 1, gap: space[3] },
  fill: { flex: 1 },
});
