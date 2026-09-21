import { useState } from 'react';
import { StyleSheet, View } from 'react-native';

import { deviceInfo } from '../src/api/device';
import { join, login, register } from '../src/api/endpoints';
import type { Hall } from '../src/api/types';
import { describeError, type FieldErrors } from '../src/api/errors';
import { validateJoin, validateLogin, validateRegister } from '../src/features/join/form';
import { t } from '../src/i18n/ru';
import { useAuthStore } from '../src/store/auth';
import { AppBar, Button, CodeInput, EmptyState, Icon, Input, ListRow, Screen, SegmentedControl, Text, Toast } from '../src/ui';
import { space } from '../src/ui/tokens';

type Mode = 'code' | 'login' | 'register';

function HallPicker({ halls }: { halls: Hall[] }) {
  if (halls.length === 0) {
    return <EmptyState icon="grid" title={t.join.noHalls} message={t.hall.noHallsHint} />;
  }
  return (
    <View>
      {halls.map((h) => (
        <ListRow key={h.id} title={h.name} right={<Icon name="chevron-right" color="textMuted" />} onPress={() => void useAuthStore.getState().selectHall(h.id)} />
      ))}
    </View>
  );
}

export default function JoinScreen() {
  const me = useAuthStore((s) => s.me);
  const [mode, setMode] = useState<Mode>('code');
  const [code, setCode] = useState('');
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [venueName, setVenueName] = useState('');
  const [fields, setFields] = useState<FieldErrors>({});
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // Signed in without a chosen hall: the guard keeps us here for the hall choice.
  if (me) {
    return (
      <Screen scroll>
        <AppBar title={t.join.chooseHall} subtitle={me.venue.name} />
        <HallPicker halls={me.halls} />
      </Screen>
    );
  }

  const submit = async () => {
    const local =
      mode === 'code'
        ? validateJoin(code, name)
        : mode === 'login'
          ? validateLogin(email, password)
          : validateRegister({ email, password, display_name: name, venue_name: venueName });
    setFields(local);
    setMessage(null);
    if (Object.keys(local).length > 0) return;

    setBusy(true);
    try {
      const device = await deviceInfo();
      const res =
        mode === 'code'
          ? await join(code, name.trim(), device)
          : mode === 'login'
            ? await login(email.trim(), password, device)
            : await register({ email: email.trim(), password, display_name: name.trim(), venue_name: venueName.trim() }, device);
      await useAuthStore.getState().signIn(res.tokens);
    } catch (e) {
      const d = describeError(e);
      setFields(d.fields);
      setMessage(d.message);
    } finally {
      setBusy(false);
    }
  };

  const title = mode === 'register' ? t.join.registerSubmit : t.join.submit;

  return (
    <Screen scroll footer={<Button title={title} variant="primary" size="lg" loading={busy} onPress={() => void submit()} />}>
      <AppBar title={t.join.title} />
      <SegmentedControl
        options={[
          { value: 'code', label: t.join.modeCode },
          { value: 'login', label: t.join.modeLogin },
          { value: 'register', label: t.join.modeRegister },
        ]}
        value={mode}
        onChange={(m) => {
          setMode(m);
          setFields({});
          setMessage(null);
        }}
      />
      {message ? <Toast message={message} tone="danger" /> : null}
      {mode === 'code' ? (
        <View style={styles.form}>
          <CodeInput length={6} value={code} onChange={setCode} autoFocus />
          {fields.code ? (
            <Text variant="label" color="danger">
              {fields.code}
            </Text>
          ) : (
            <Text variant="label" color="textMuted">
              {t.join.codeHint}
            </Text>
          )}
          <Input label={t.join.name} placeholder={t.join.namePlaceholder} value={name} onChangeText={setName} error={fields.display_name} />
        </View>
      ) : (
        <View style={styles.form}>
          <Input label={t.join.email} value={email} onChangeText={setEmail} keyboardType="email-address" error={fields.email} />
          <Input label={t.join.password} value={password} onChangeText={setPassword} error={fields.password} secure />
          {mode === 'register' ? (
            <>
              <Input label={t.join.ownerName} value={name} onChangeText={setName} error={fields.display_name} />
              <Input label={t.join.venueName} value={venueName} onChangeText={setVenueName} error={fields.venue_name} />
            </>
          ) : null}
        </View>
      )}
    </Screen>
  );
}

const styles = StyleSheet.create({
  form: { gap: space[4] },
});
