import { useRouter } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { StyleSheet, View } from 'react-native';

import { getServicePlan, setServicePlan } from '../../src/api/endpoints';
import type { Plan } from '../../src/api/types';
import { previewPlan, validatePlan } from '../../src/domain/plan';
import { describeError } from '../../src/features/join/form';
import { t } from '../../src/i18n/ru';
import { useHallStore } from '../../src/store/hall';
import {
  AppBar, Badge, Button, Card, IconButton, ListRow, NumberField, Screen, Spinner, Text, Toast,
} from '../../src/ui';
import { serverNow } from '../../src/ws/clock';
import { space } from '../../src/ui/tokens';

function minutes(sec: number): number {
  return Math.round(sec / 60);
}

// The editor works in minutes; the contract keeps seconds.
export default function PlanEditor() {
  const router = useRouter();
  const fromHall = useHallStore((s) => s.state?.plan ?? null);
  const [plan, setPlan] = useState<Plan | null>(fromHall);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (plan) return;
    getServicePlan()
      .then(setPlan)
      .catch((e: unknown) => setMessage(describeError(e).message ?? t.errors.unknown));
  }, [plan]);

  const problem = useMemo(() => (plan ? validatePlan(plan) : null), [plan]);
  const preview = useMemo(
    () => (plan && !problem ? previewPlan(plan, new Date(serverNow()).toISOString()) : []),
    [plan, problem],
  );

  if (!plan) {
    return (
      <Screen>
        <AppBar title={t.plan.title} left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />} />
        {message ? <Toast message={message} tone="danger" /> : <Spinner size="large" />}
      </Screen>
    );
  }

  const patchStep = (index: number, patch: Partial<Plan['steps'][number]>) =>
    setPlan({ ...plan, steps: plan.steps.map((s, i) => (i === index ? { ...s, ...patch } : s)) });

  const save = async () => {
    setBusy(true);
    setMessage(null);
    setSaved(false);
    try {
      const stored = await setServicePlan(plan);
      setPlan(stored);
      setSaved(true);
    } catch (e) {
      const d = describeError(e);
      setMessage(d.message ?? t.errors.unknown);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen
      scroll
      footer={
        <>
          {problem ? <Toast message={t.plan.invalid(problem.reason)} tone="danger" /> : null}
          {message ? <Toast message={message} tone="danger" /> : null}
          {saved ? <Toast message={t.plan.saved} tone="success" /> : null}
          <Button title={t.plan.save} variant="primary" size="lg" icon="check" loading={busy} disabled={problem !== null} onPress={() => void save()} />
        </>
      }
    >
      <AppBar
        title={t.plan.title}
        left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />}
      />
      <Text variant="label" color="textMuted">
        {t.plan.hint}
      </Text>

      {plan.steps.map((step, i) => (
        <Card key={`${step.kind}-${i}`}>
          <View style={styles.row}>
            <Text variant="h2">{t.plan.kinds[step.kind]}</Text>
            <Badge title={t.plan.step(i + 1)} tone="neutral" />
          </View>
          <Text variant="label" color="textMuted" numberOfLines={2}>
            {step.title}
          </Text>
          <Text variant="label" color="textMuted">
            {t.plan.offset}
          </Text>
          <NumberField
            value={minutes(step.offset_sec)}
            min={1}
            max={1440}
            step={1}
            suffix={t.plan.minutes}
            onChange={(v) => patchStep(i, { offset_sec: v * 60 })}
          />
          {step.kind === 'expire' ? null : (
            <>
              <Text variant="label" color="textMuted">
                {step.repeat_every_sec ? t.plan.repeat : t.plan.noRepeat}
              </Text>
              <NumberField
                value={step.repeat_every_sec ? minutes(step.repeat_every_sec) : 0}
                min={0}
                max={1440}
                step={1}
                suffix={t.plan.minutes}
                onChange={(v) => patchStep(i, { repeat_every_sec: v === 0 ? undefined : v * 60 })}
              />
            </>
          )}
        </Card>
      ))}

      <Card>
        <Text variant="label" color="textMuted">
          {t.plan.warn}
        </Text>
        <NumberField value={plan.warn_before_sec} min={0} max={300} step={10} suffix={t.plan.seconds} onChange={(v) => setPlan({ ...plan, warn_before_sec: v })} />
        <Text variant="label" color="textMuted">
          {t.plan.grace}
        </Text>
        <NumberField value={plan.grace_sec} min={30} max={900} step={10} suffix={t.plan.seconds} onChange={(v) => setPlan({ ...plan, grace_sec: v })} />
        <Text variant="label" color="textMuted">
          {t.plan.missed}
        </Text>
        <NumberField value={minutes(plan.missed_after_sec)} min={1} max={60} step={1} suffix={t.plan.minutes} onChange={(v) => setPlan({ ...plan, missed_after_sec: v * 60 })} />
        <Text variant="label" color="textMuted">
          {t.plan.pushRepeat}
        </Text>
        <NumberField value={minutes(plan.push_repeat_sec)} min={1} max={60} step={1} suffix={t.plan.minutes} onChange={(v) => setPlan({ ...plan, push_repeat_sec: v * 60 })} />
        <Text variant="label" color="textMuted">
          {t.plan.pushAttempts}
        </Text>
        <NumberField value={plan.push_max_attempts} min={1} max={10} step={1} suffix={t.plan.times} onChange={(v) => setPlan({ ...plan, push_max_attempts: v })} />
      </Card>

      <Text variant="label" color="textMuted">
        {t.plan.preview}
      </Text>
      <View>
        {preview.map((event) => (
          <ListRow
            key={event.seq}
            title={event.title}
            subtitle={t.plan.kinds[event.kind]}
            right={<Text variant="mono">{new Date(event.due_at).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}</Text>}
          />
        ))}
      </View>
    </Screen>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: space[2] },
});
