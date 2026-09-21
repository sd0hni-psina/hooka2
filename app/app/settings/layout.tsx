import { useRouter } from 'expo-router';
import { useMemo, useState } from 'react';
import { StyleSheet, View } from 'react-native';

import { archiveTable, createHall, createTable, patchTable, setLayout } from '../../src/api/endpoints';
import type { Table, TableStatus } from '../../src/api/types';
import {
  applyPreset, clampPosition, GRID, overlappingIds, placementOf, presetOf, resize, type ShapePreset,
} from '../../src/domain/layout';
import { describeError, explain, type FieldErrors } from '../../src/api/errors';
import { t } from '../../src/i18n/ru';
import { useAuthStore } from '../../src/store/auth';
import { useHallStore } from '../../src/store/hall';
import { savedTables } from '../../src/store/selectors';
import {
  AppBar, Badge, Button, ConfirmDialog, EmptyState, HallCanvas, IconButton, Input, NumberField, Screen,
  SegmentedControl, Sheet, Text, toast,
} from '../../src/ui';
import { space } from '../../src/ui/tokens';

// The editor works on a local copy: dragging changes it, saving sends one
// batch request, tech.md section 19.6.
export default function LayoutEditor() {
  const router = useRouter();
  const hallId = useAuthStore((s) => s.hallId ?? s.me?.halls[0]?.id ?? null);
  const hallName = useAuthStore((s) => s.me?.halls.find((h) => h.id === s.hallId)?.name ?? '');
  const saved = useHallStore(savedTables);

  // Dragging only overrides positions; everything else comes from the server.
  const [moved, setMoved] = useState<Record<string, { x: number; y: number }>>({});
  const [selected, setSelected] = useState<Table | null>(null);
  const [busy, setBusy] = useState(false);
  const [fields, setFields] = useState<FieldErrors>({});
  const [confirmArchive, setConfirmArchive] = useState(false);

  const tables: Table[] = useMemo(
    () => saved.map((tb) => {
      const m = moved[tb.id];
      return m ? { ...tb, x: m.x, y: m.y } : tb;
    }),
    [saved, moved],
  );
  const dirty = useMemo(
    () => tables.filter((tb) => moved[tb.id] !== undefined && saved.some((s) => s.id === tb.id && (s.x !== tb.x || s.y !== tb.y))).map(placementOf),
    [tables, moved, saved],
  );
  const warnIds = useMemo(() => overlappingIds(tables.map(placementOf)), [tables]);
  const statuses = useMemo(() => Object.fromEntries(tables.map((tb) => [tb.id, 'free' as TableStatus])), [tables]);

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setFields({});
    try {
      await fn();
    } catch (e) {
      const d = describeError(e);
      toast({ key: 'layout', message: explain(e), tone: 'danger' });
      setFields(d.fields);
    } finally {
      setBusy(false);
    }
  };

  if (!hallId) {
    return (
      <Screen>
        <AppBar title={t.layout.title} left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />} />
        <EmptyState
          icon="grid"
          title={t.hall.noHalls}
          message={t.layout.createHallHint}
          actionTitle={t.layout.addHall}
          onAction={() => void run(async () => {
            const hall = await createHall(t.layout.defaultHallName, 0);
            await useAuthStore.getState().refreshMe();
            await useAuthStore.getState().selectHall(hall.id);
          })}
        />
      </Screen>
    );
  }

  const nextLabel = String(saved.length + 1);

  return (
    <Screen
      footer={
        <>
          <Button
            title={t.layout.save}
            variant="primary"
            size="lg"
            icon="check"
            loading={busy}
            disabled={dirty.length === 0}
            onPress={() => void run(async () => {
              await setLayout(hallId, dirty);
              setMoved({});
            })}
          />
          <Button
            title={t.layout.addTable}
            variant="secondary"
            size="md"
            icon="plus"
            loading={busy}
            onPress={() => void run(() => createTable(hallId, { label: nextLabel, shape: 'rect', x: 4, y: 4, w: 12, h: 8, rotation: 0, seats: 4 }))}
          />
        </>
      }
    >
      <AppBar
        title={t.layout.title}
        subtitle={hallName}
        left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />}
        right={dirty.length > 0 ? <Badge title={t.layout.unsaved(dirty.length)} tone="yellow" icon="edit" /> : undefined}
      />
      <Text variant="label" color="textMuted">
        {t.layout.hint}
      </Text>
      {warnIds.size > 0 ? (
        <Text variant="label" color="yellow">
          {t.layout.overlap}
        </Text>
      ) : null}
      <HallCanvas
        fill
        tables={tables}
        statuses={statuses}
        editable
        warnIds={warnIds}
        selectedId={selected?.id ?? null}
        onTablePress={(table) => setSelected(table)}
        onTableMove={(id, x, y) => setMoved((prev) => ({ ...prev, [id]: { x, y } }))}
      />

      <Sheet
        visible={selected !== null}
        onClose={() => setSelected(null)}
        title={selected ? `${t.layout.settings}: ${selected.label}` : t.layout.settings}
        footer={
          <>
            <Button
              title={t.layout.apply}
              variant="primary"
              size="lg"
              loading={busy}
              onPress={() => void run(async () => {
                if (!selected) return;
                await patchTable(selected.id, {
                  label: selected.label, shape: selected.shape, w: selected.w, h: selected.h,
                  x: selected.x, y: selected.y, rotation: selected.rotation, seats: selected.seats,
                });
                setMoved((prev) => {
                  const next = { ...prev };
                  delete next[selected.id];
                  return next;
                });
                setSelected(null);
              })}
            />
            <Button title={t.layout.archive} variant="danger" size="md" icon="trash" onPress={() => setConfirmArchive(true)} />
          </>
        }
      >
        {selected ? (
          <View style={styles.form}>
            <Input label={t.layout.label} value={selected.label} onChangeText={(label) => setSelected({ ...selected, label })} error={fields.label} />

            <Text variant="label" color="textMuted">
              {t.layout.shape}
            </Text>
            <SegmentedControl
              options={[
                { value: 'rect', label: t.layout.rect },
                { value: 'square', label: t.layout.square },
                { value: 'circle', label: t.layout.circle },
              ]}
              value={presetOf(selected)}
              onChange={(preset: ShapePreset) => setSelected({ ...selected, ...applyPreset(selected, preset) })}
            />

            <Text variant="label" color="textMuted">
              {presetOf(selected) === 'circle' ? t.layout.diameter : presetOf(selected) === 'square' ? t.layout.side : t.layout.width}
            </Text>
            <NumberField
              value={selected.w}
              min={1}
              max={40}
              step={1}
              onChange={(w) => setSelected({ ...selected, ...resize(selected, 'w', w) })}
            />
            {presetOf(selected) === 'rect' ? (
              <>
                <Text variant="label" color="textMuted">
                  {t.layout.height}
                </Text>
                <NumberField
                  value={selected.h}
                  min={1}
                  max={40}
                  step={1}
                  onChange={(h) => setSelected({ ...selected, ...resize(selected, 'h', h) })}
                />
              </>
            ) : null}

            <Text variant="label" color="textMuted">
              {t.layout.rotation}
            </Text>
            <SegmentedControl
              options={[
                { value: '0', label: '0°' },
                { value: '45', label: '45°' },
                { value: '90', label: '90°' },
                { value: '135', label: '135°' },
              ]}
              value={String(selected.rotation)}
              onChange={(v) => setSelected({ ...selected, rotation: Number(v) })}
            />
            <NumberField
              value={selected.rotation}
              min={0}
              max={345}
              step={15}
              suffix="°"
              onChange={(rotation) => setSelected({ ...selected, rotation })}
            />

            <Text variant="label" color="textMuted">
              {t.layout.position}
            </Text>
            <Text variant="label" color="textMuted">
              {t.layout.positionHint}
            </Text>
            <NumberField
              value={selected.x}
              min={0}
              max={GRID - selected.w}
              step={1}
              suffix={t.layout.x}
              onChange={(x) => setSelected({ ...selected, ...clampPosition(x, selected.y, selected.w, selected.h) })}
            />
            <NumberField
              value={selected.y}
              min={0}
              max={GRID - selected.h}
              step={1}
              suffix={t.layout.y}
              onChange={(y) => setSelected({ ...selected, ...clampPosition(selected.x, y, selected.w, selected.h) })}
            />

            <Text variant="label" color="textMuted">
              {t.layout.seats}
            </Text>
            <NumberField value={selected.seats} min={1} max={30} step={1} onChange={(seats) => setSelected({ ...selected, seats })} />
          </View>
        ) : null}
      </Sheet>

      <ConfirmDialog
        visible={confirmArchive}
        title={t.layout.archiveTitle}
        message={t.layout.archiveMessage}
        confirmTitle={t.layout.archive}
        destructive
        onCancel={() => setConfirmArchive(false)}
        onConfirm={() => {
          setConfirmArchive(false);
          const table = selected;
          setSelected(null);
          if (table) void run(() => archiveTable(table.id));
        }}
      />
    </Screen>
  );
}

const styles = StyleSheet.create({
  form: { gap: space[3] },
});
