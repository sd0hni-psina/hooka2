import { Redirect, useRouter } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, View } from 'react-native';
import Svg from 'react-native-svg';

import type { Table, TableStatus } from '../src/api/types';
import {
  AppBar, Badge, Button, Card, CodeInput, ConfirmDialog, Countdown, EmptyState, HallCanvas, IconButton, Input,
  ListRow, NumberField, Screen, SegmentedControl, Sheet, Spinner, TableShape, Text, Toast, statusStyle,
} from '../src/ui';
import { space } from '../src/ui/tokens';
import { serverNow } from '../src/ws/clock';

// Dev-only screen that renders every primitive of tech.md section 12.
const sampleTables: Table[] = [
  { id: 't1', hall_id: 'h', label: '1', shape: 'rect', x: 6, y: 8, w: 18, h: 12, rotation: 0, seats: 4 },
  { id: 't2', hall_id: 'h', label: '2', shape: 'circle', x: 36, y: 8, w: 14, h: 14, rotation: 0, seats: 4 },
  { id: 't3', hall_id: 'h', label: '3', shape: 'rect', x: 62, y: 8, w: 18, h: 12, rotation: 15, seats: 6 },
  { id: 't4', hall_id: 'h', label: '15', shape: 'rect', x: 20, y: 50, w: 24, h: 14, rotation: 0, seats: 8 },
];
const sampleStatuses: Record<string, TableStatus> = { t1: 'free', t2: 'green', t3: 'yellow', t4: 'red' };
const statusList: TableStatus[] = ['free', 'green', 'yellow', 'red'];

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <View style={styles.section}>
      <Text variant="label" color="textMuted">
        {title}
      </Text>
      {children}
    </View>
  );
}

export default function KitchenSink() {
  const router = useRouter();
  const [text, setText] = useState('');
  const [code, setCode] = useState('12');
  const [num, setNum] = useState(15);
  const [seg, setSeg] = useState<'hall' | 'tasks'>('hall');
  const [sheet, setSheet] = useState(false);
  const [dialog, setDialog] = useState(false);
  const [started] = useState(() => new Date(serverNow() - 17 * 60_000).toISOString());
  const [deadline] = useState(() => new Date(serverNow() + 4 * 60_000).toISOString());

  if (!__DEV__) return <Redirect href="/" />;

  return (
    <Screen scroll footer={<Button title="Primary lg" variant="primary" size="lg" icon="flame" onPress={() => setDialog(true)} />}>
      <AppBar
        title="Kitchen sink"
        subtitle="src/ui"
        left={<IconButton icon="chevron-left" accessibilityLabel="Назад" onPress={() => router.back()} />}
        right={<IconButton icon="settings" variant="filled" accessibilityLabel="Настройки" onPress={() => setSheet(true)} />}
      />
      <Section title="Text">
        <Text variant="h1">Заголовок h1</Text>
        <Text variant="h2">Заголовок h2</Text>
        <Text variant="body">Основной текст body</Text>
        <Text variant="label" color="textMuted">
          Подпись label
        </Text>
        <Text variant="mono">1:20:00</Text>
      </Section>
      <Section title="Button">
        <Button title="Primary" variant="primary" size="md" onPress={() => undefined} />
        <Button title="Secondary" variant="secondary" size="md" icon="swap" onPress={() => undefined} />
        <Button title="Danger" variant="danger" size="md" icon="stop" onPress={() => undefined} />
        <Button title="Ghost" variant="ghost" size="md" onPress={() => undefined} />
        <Button title="Loading" variant="primary" size="md" loading onPress={() => undefined} />
        <Button title="Disabled" variant="primary" size="md" disabled onPress={() => undefined} />
      </Section>
      <Section title="IconButton">
        <View style={styles.row}>
          <IconButton icon="check" accessibilityLabel="check" onPress={() => undefined} />
          <IconButton icon="hand" variant="filled" accessibilityLabel="hand" onPress={() => undefined} />
          <IconButton icon="trash" variant="filled" accessibilityLabel="trash" onPress={() => undefined} />
        </View>
      </Section>
      <Section title="Input">
        <Input label="Имя" value={text} onChangeText={setText} placeholder="Ерлан" />
        <Input label="С ошибкой" value="x" onChangeText={() => undefined} error="Слишком коротко" />
      </Section>
      <Section title="NumberField">
        <NumberField value={num} onChange={setNum} min={2} max={60} step={1} suffix="мин" />
      </Section>
      <Section title="CodeInput">
        <CodeInput length={4} value={code} onChange={setCode} />
      </Section>
      <Section title="Card + Countdown">
        {statusList.map((s) => (
          <Card key={s} status={s} onPress={() => undefined}>
            <Text variant="body">{statusStyle[s].label}</Text>
            <Countdown target={started} mode="up" status={s} />
          </Card>
        ))}
        <Card>
          <Text variant="label" color="textMuted">
            До дедлайна
          </Text>
          <Countdown target={deadline} mode="down" />
          <Text variant="label" color="textMuted">
            Остановлен
          </Text>
          <Countdown target={started} from={deadline} mode="up" />
        </Card>
      </Section>
      <Section title="Badge">
        <View style={styles.row}>
          <Badge title="neutral" tone="neutral" />
          <Badge title="Работает" tone="green" icon="flame" />
          <Badge title="Взял Ерлан" tone="yellow" icon="hand" />
          <Badge title="Срочно" tone="red" icon="alert" />
          <Badge title="accent" tone="accent" />
        </View>
      </Section>
      <Section title="ListRow">
        <ListRow title="Стол 15" subtitle="Стряхните угли" right={<Badge title="2:10" tone="yellow" />} onPress={() => undefined} />
        <ListRow title="Без действия" />
      </Section>
      <Section title="SegmentedControl">
        <SegmentedControl
          options={[
            { value: 'hall', label: 'Зал' },
            { value: 'tasks', label: 'Задачи' },
          ]}
          value={seg}
          onChange={setSeg}
        />
      </Section>
      <Section title="TableShape">
        <Svg width="100%" height={120} viewBox="0 0 100 24">
          {statusList.map((s, i) => (
            <TableShape
              key={s}
              table={{ id: s, hall_id: 'h', label: String(i + 1), shape: i % 2 ? 'circle' : 'rect', x: 4 + i * 24, y: 2, w: 18, h: 18, rotation: 0, seats: 4 }}
              status={s}
              label={String(i + 1)}
              badge={s === 'yellow' ? 'Ерлан' : undefined}
              selected={s === 'green'}
              onPress={() => undefined}
            />
          ))}
        </Svg>
      </Section>
      <Section title="HallCanvas">
        <HallCanvas tables={sampleTables} statuses={sampleStatuses} onTablePress={() => undefined} />
      </Section>
      <Section title="Toast">
        <Toast message="Стол 15: стряхните угли" tone="warning" actionTitle="Беру" onAction={() => undefined} />
        <Toast message="Нет связи" tone="danger" />
        <Toast message="Сохранено" tone="success" />
        <Toast message="Подсказка" tone="neutral" />
      </Section>
      <Section title="EmptyState">
        <EmptyState icon="list" title="Задач нет" message="Всё под контролем" actionTitle="Обновить" onAction={() => undefined} />
      </Section>
      <Section title="Spinner">
        <View style={styles.row}>
          <Spinner />
          <Spinner size="large" />
        </View>
      </Section>
      <Sheet visible={sheet} onClose={() => setSheet(false)} title="Стол 15" footer={<Button title="Завершить" variant="danger" size="lg" onPress={() => setSheet(false)} />}>
        <Text variant="body">Содержимое шита</Text>
      </Sheet>
      <ConfirmDialog
        visible={dialog}
        title="Завершить кальян?"
        message="Будущие задачи отменятся"
        confirmTitle="Завершить"
        destructive
        onConfirm={() => setDialog(false)}
        onCancel={() => setDialog(false)}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  section: { gap: space[2] },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: space[2], alignItems: 'center' },
});
