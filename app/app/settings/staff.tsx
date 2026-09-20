import { useRouter } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { View } from 'react-native';

import { getStaff, patchStaff, rotateCode } from '../../src/api/endpoints';
import type { Staff } from '../../src/api/types';
import { describeError } from '../../src/features/join/form';
import { t } from '../../src/i18n/ru';
import { useAuthStore } from '../../src/store/auth';
import {
  AppBar, Badge, Button, Card, ConfirmDialog, EmptyState, IconButton, ListRow, Screen, Spinner, Text, Toast,
} from '../../src/ui';

export default function StaffScreen() {
  const router = useRouter();
  const me = useAuthStore((s) => s.me);
  const [list, setList] = useState<Staff[] | null>(null);
  const [code, setCode] = useState<string | null>(me?.venue.join_code ?? null);
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmBlock, setConfirmBlock] = useState<Staff | null>(null);
  const [confirmRotate, setConfirmRotate] = useState(false);

  const run = useCallback(async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setMessage(null);
    try {
      await fn();
    } catch (e) {
      setMessage(describeError(e).message ?? t.errors.unknown);
    } finally {
      setBusy(false);
    }
  }, []);

  // The first load sets state only after the await, so the effect itself
  // does not trigger a render cascade.
  const reload = useCallback(async () => {
    try {
      setList(await getStaff());
    } catch (e) {
      setMessage(describeError(e).message ?? t.errors.unknown);
    }
  }, []);

  useEffect(() => {
    // State is set from the promise callback, never in the effect body.
    getStaff()
      .then(setList)
      .catch((e: unknown) => setMessage(describeError(e).message ?? t.errors.unknown));
  }, []);

  const isOwner = me?.staff.role === 'owner';

  return (
    <Screen scroll>
      <AppBar
        title={t.staff.title}
        subtitle={me?.venue.name}
        left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />}
        right={<IconButton icon="refresh" accessibilityLabel={t.ui.retry} onPress={() => void reload()} />}
      />
      {message ? <Toast message={message} tone="danger" /> : null}

      <Card>
        <Text variant="h2">{code ? t.staff.code(code) : t.staff.title}</Text>
        <Button title={t.staff.rotate} variant="secondary" size="md" icon="refresh" loading={busy} onPress={() => setConfirmRotate(true)} />
      </Card>

      {list === null ? (
        <Spinner size="large" />
      ) : list.length === 0 ? (
        <EmptyState icon="users" title={t.staff.empty} message={t.staff.emptyHint} />
      ) : (
        <View>
          {list.map((person) => (
            <ListRow
              key={person.id}
              title={person.display_name}
              subtitle={t.staff.role[person.role]}
              right={
                <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
                  {person.blocked ? <Badge title={t.staff.blocked} tone="red" icon="alert" /> : null}
                  {isOwner && person.role !== 'owner' ? (
                    <IconButton
                      icon="user"
                      variant="filled"
                      accessibilityLabel={person.role === 'admin' ? t.staff.makeMaster : t.staff.makeAdmin}
                      onPress={() =>
                        void run(async () => {
                          await patchStaff(person.id, { role: person.role === 'admin' ? 'master' : 'admin' });
                          await reload();
                        })
                      }
                    />
                  ) : null}
                  {person.role !== 'owner' && person.id !== me?.staff.id ? (
                    <IconButton
                      icon={person.blocked ? 'check' : 'close'}
                      variant="filled"
                      accessibilityLabel={person.blocked ? t.staff.unblock : t.staff.block}
                      onPress={() => {
                        if (person.blocked) {
                          void run(async () => {
                            await patchStaff(person.id, { blocked: false });
                            await reload();
                          });
                          return;
                        }
                        setConfirmBlock(person);
                      }}
                    />
                  ) : null}
                </View>
              }
            />
          ))}
        </View>
      )}

      <ConfirmDialog
        visible={confirmBlock !== null}
        title={t.staff.blockTitle}
        message={t.staff.blockMessage}
        confirmTitle={t.staff.block}
        destructive
        onCancel={() => setConfirmBlock(null)}
        onConfirm={() => {
          const person = confirmBlock;
          setConfirmBlock(null);
          if (!person) return;
          void run(async () => {
            await patchStaff(person.id, { blocked: true });
            await reload();
          });
        }}
      />

      <ConfirmDialog
        visible={confirmRotate}
        title={t.staff.rotateTitle}
        message={t.staff.rotateMessage}
        confirmTitle={t.staff.rotate}
        onCancel={() => setConfirmRotate(false)}
        onConfirm={() => {
          setConfirmRotate(false);
          void run(async () => {
            setCode(await rotateCode());
            await useAuthStore.getState().refreshMe();
          });
        }}
      />
    </Screen>
  );
}
