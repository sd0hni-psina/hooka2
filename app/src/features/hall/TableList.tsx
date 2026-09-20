import { FlatList, StyleSheet, View } from 'react-native';

import type { Hookah, Table, TableStatus } from '../../api/types';
import { t } from '../../i18n/ru';
import { Badge, Countdown, Icon, ListRow, Text, statusStyle } from '../../ui';
import { space } from '../../ui/tokens';

type Row = { table: Table; hookah: Hookah | null; status: TableStatus };

type Props = {
  tables: Table[];
  statuses: Record<string, TableStatus>;
  hookahs: Map<string, Hookah>;
  onPress: (table: Table) => void;
  onLongPress: (table: Table) => void;
};

const order: Record<TableStatus, number> = { red: 0, yellow: 1, green: 2, free: 3 };

// The map answers where, the list answers what. A master who knows the hall
// by heart reads the list faster, tech.md section 19.15.
export function TableList({ tables, statuses, hookahs, onPress, onLongPress }: Props) {
  const rows: Row[] = tables
    .map((table) => ({ table, hookah: hookahs.get(table.id) ?? null, status: statuses[table.id] ?? 'free' }))
    .sort((a, b) => order[a.status] - order[b.status] || a.table.label.localeCompare(b.table.label, 'ru', { numeric: true }));

  return (
    <FlatList
      data={rows}
      keyExtractor={(r) => r.table.id}
      contentContainerStyle={styles.list}
      initialNumToRender={12}
      windowSize={7}
      renderItem={({ item }) => (
        <ListRow
          title={t.table.title(item.table.label)}
          subtitle={item.hookah ? t.table.startedBy(item.hookah.started_by_name) : t.table.seats(item.table.seats)}
          left={<Icon name={statusStyle[item.status].icon} color={statusStyle[item.status].color} />}
          onPress={() => onPress(item.table)}
          onLongPress={() => onLongPress(item.table)}
          right={
            <View style={styles.right}>
              {item.hookah ? (
                <Countdown target={item.hookah.started_at} mode="up" status={item.status} granularity="minute" variant="label" />
              ) : null}
              <Badge title={statusStyle[item.status].label} tone={item.status === 'free' ? 'neutral' : item.status} />
            </View>
          }
        />
      )}
      ListEmptyComponent={
        <Text variant="body" color="textMuted" align="center">
          {t.hall.notFound}
        </Text>
      }
    />
  );
}

const styles = StyleSheet.create({
  list: { paddingBottom: space[5] },
  right: { flexDirection: 'row', alignItems: 'center', gap: space[2] },
});
