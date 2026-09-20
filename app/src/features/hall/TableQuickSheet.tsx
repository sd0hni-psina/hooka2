import { useRouter } from 'expo-router';

import type { Hookah, Table, TableStatus } from '../../api/types';
import { formatDuration } from '../../domain/countdown';
import { t } from '../../i18n/ru';
import { Badge, Icon, ListRow, Sheet, statusStyle, Text, useServerNow } from '../../ui';
import { useTaskActions } from '../tasks/useTaskActions';

type Props = {
  table: Table | null;
  hookah: Hookah | null;
  status: TableStatus;
  disabled: boolean;
  onClose: () => void;
  onStart: (table: Table) => void;
};

// A long press answers the common case without a screen change: confirm the
// task that fired, or put a hookah on a free table.
export function TableQuickSheet({ table, hookah, status, disabled, onClose, onStart }: Props) {
  const router = useRouter();
  const now = useServerNow();
  const actions = useTaskActions();
  if (!table) return null;

  const open = hookah?.events.find((e) => e.state === 'due') ?? hookah?.events.find((e) => e.state === 'pending');

  return (
    <Sheet visible onClose={onClose} title={t.table.title(table.label)}>
      <ListRow
        title={statusStyle[status].label}
        subtitle={hookah ? t.table.startedBy(hookah.started_by_name) : t.table.seats(table.seats)}
        left={<Icon name={statusStyle[status].icon} color={statusStyle[status].color} />}
        right={
          hookah ? (
            <Badge title={formatDuration(now - Date.parse(hookah.started_at))} tone="neutral" icon="clock" />
          ) : undefined
        }
      />

      {hookah ? (
        <>
          {open ? (
            <>
              <Text variant="label" color="textMuted">
                {t.table.nextTask}
              </Text>
              <ListRow
                title={open.title}
                subtitle={open.claimed_by_name ? t.tasks.claimedBy(open.claimed_by_name) : undefined}
                left={<Icon name="check" color="green" />}
                onPress={disabled ? undefined : () => {
                  onClose();
                  actions.confirm(open.id);
                }}
              />
              {open.state === 'due' && !open.claimed_by ? (
                <ListRow
                  title={t.tasks.claim}
                  left={<Icon name="hand" color="accent" />}
                  onPress={disabled ? undefined : () => {
                    onClose();
                    actions.claim(open.id);
                  }}
                />
              ) : null}
            </>
          ) : (
            <Text variant="label" color="textMuted">
              {t.table.noTasks}
            </Text>
          )}
          <ListRow
            title={t.table.open}
            left={<Icon name="grid" color="textMuted" />}
            right={<Icon name="chevron-right" color="textMuted" />}
            onPress={() => {
              onClose();
              router.push({ pathname: '/table/[id]', params: { id: table.id } });
            }}
          />
        </>
      ) : (
        <ListRow
          title={t.table.start}
          left={<Icon name="flame" color="accent" />}
          onPress={disabled ? undefined : () => {
            onClose();
            onStart(table);
          }}
        />
      )}
    </Sheet>
  );
}
