import { useState } from 'react';
import { useRouter } from 'expo-router';

import { t } from '../../i18n/ru';
import { useAuthStore } from '../../store/auth';
import { useHallStore } from '../../store/hall';
import { Badge, Button, ConfirmDialog, Icon, ListRow, Sheet, Text } from '../../ui';

type Props = { visible: boolean; onClose: () => void };

// Who is signed in, who else is in this hall, how to switch halls and how to
// leave the shift, tech.md section 19.14.
export function ProfileSheet({ visible, onClose }: Props) {
  const router = useRouter();
  const me = useAuthStore((s) => s.me);
  const hallId = useHallStore((s) => s.hallId);
  const online = useHallStore((s) => s.state?.staff_online ?? []);
  const [confirmLogout, setConfirmLogout] = useState(false);
  const manager = me?.staff.role !== 'master';
  const others = online.filter((s) => s.id !== me?.staff.id);

  return (
    <>
      <Sheet
        visible={visible}
        onClose={onClose}
        title={t.profile.title}
        footer={
          <Button
            title={t.logout.action}
            variant="danger"
            size="lg"
            icon="close"
            onPress={() => {
              onClose();
              setConfirmLogout(true);
            }}
          />
        }
      >
        <ListRow
          title={me?.staff.display_name ?? ''}
          subtitle={me ? t.roles[me.staff.role] : undefined}
          left={<Icon name="user" color="accent" />}
        />

        <Text variant="label" color="textMuted">
          {t.profile.online}
        </Text>
        {others.length === 0 ? (
          <Text variant="label" color="textMuted">
            {t.profile.aloneOnShift}
          </Text>
        ) : (
          others.map((s) => <ListRow key={s.id} title={s.display_name} left={<Icon name="user" color="textMuted" />} />)
        )}

        <Text variant="label" color="textMuted">
          {t.profile.hall}
        </Text>
        {(me?.halls ?? []).map((h) => (
          <ListRow
            key={h.id}
            title={h.name}
            selected={h.id === hallId}
            right={h.id === hallId ? <Badge title={t.ui.chosen} tone="green" icon="check" /> : undefined}
            onPress={() => {
              onClose();
              void useAuthStore.getState().selectHall(h.id);
            }}
          />
        ))}

        {manager ? (
          <ListRow
            title={t.settings.title}
            left={<Icon name="settings" color="textMuted" />}
            right={<Icon name="chevron-right" color="textMuted" />}
            onPress={() => {
              onClose();
              router.push('/settings');
            }}
          />
        ) : (
          <Text variant="label" color="textMuted">
            {t.profile.onlyManagers}
          </Text>
        )}
      </Sheet>

      <ConfirmDialog
        visible={confirmLogout}
        title={t.logout.title}
        message={t.logout.message}
        confirmTitle={t.logout.action}
        destructive
        onCancel={() => setConfirmLogout(false)}
        onConfirm={() => {
          setConfirmLogout(false);
          void useAuthStore.getState().signOut();
        }}
      />
    </>
  );
}
