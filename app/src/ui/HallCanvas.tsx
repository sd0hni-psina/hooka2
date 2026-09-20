import { useMemo, useState } from 'react';
import { StyleSheet, View, type LayoutChangeEvent } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, { useAnimatedStyle, useSharedValue, withSpring, withTiming } from 'react-native-reanimated';
import Svg, { Rect } from 'react-native-svg';

import type { Table, TableStatus } from '../api/types';
import { clampPosition, GRID } from '../domain/layout';
import { spring, timing } from './motion';
import { TableShape } from './TableShape';
import { color, radius, space } from './tokens';

const MIN_ZOOM = 1;
const MAX_ZOOM = 4;
const FOCUS_ZOOM = 2.4;
// Breathing room around the tables, in grid units.
const PADDING = 4;

type Props = {
  tables: Table[];
  statuses: Record<string, TableStatus>;
  onTablePress: (table: Table) => void;
  onTableLongPress?: (table: Table) => void;
  editable?: boolean;
  onTableMove?: (tableId: string, x: number, y: number) => void;
  // labels is the line under each table number, minute granularity.
  labels?: Record<string, string>;
  // progress is the burned share of each hookah resource, 0..1.
  progress?: Record<string, number>;
  warnIds?: Set<string>;
  urgentIds?: Set<string>;
  // dimIds pushes back the tables a search filtered out.
  dimIds?: Set<string>;
  selectedId?: string | null;
  // focusId makes the canvas fly to one table and zoom in on it.
  focusId?: string | null;
  // fill lets the canvas take all the space its parent gives it instead of
  // staying a square.
  fill?: boolean;
};

type Box = { x: number; y: number; w: number; h: number };

// visibleBox frames the tables that exist, so a hall placed in one corner of
// the grid still fills the screen. An empty hall shows the whole grid.
function visibleBox(tables: Table[], editable: boolean): Box {
  if (tables.length === 0 || editable) return { x: 0, y: 0, w: GRID, h: GRID };
  let left = GRID;
  let top = GRID;
  let right = 0;
  let bottom = 0;
  for (const t of tables) {
    left = Math.min(left, t.x);
    top = Math.min(top, t.y);
    right = Math.max(right, t.x + t.w);
    bottom = Math.max(bottom, t.y + t.h);
  }
  const x = Math.max(0, left - PADDING);
  const y = Math.max(0, top - PADDING);
  return { x, y, w: Math.min(GRID - x, right - x + PADDING), h: Math.min(GRID - y, bottom - y + PADDING) };
}

export function HallCanvas({
  tables, statuses, onTablePress, onTableLongPress, editable, onTableMove, labels, progress,
  warnIds, urgentIds, dimIds, selectedId, focusId, fill,
}: Props) {
  const [size, setSize] = useState({ w: 0, h: 0 });
  // The grabbed table and where it stood when the finger landed. Gesture
  // callbacks are rebuilt on every render, so plain state is enough.
  const [grabbed, setGrabbed] = useState<{ id: string; x: number; y: number } | null>(null);

  const scale = useSharedValue(1);
  const savedScale = useSharedValue(1);
  const tx = useSharedValue(0);
  const ty = useSharedValue(0);
  const savedX = useSharedValue(0);
  const savedY = useSharedValue(0);

  const box = visibleBox(tables, !!editable);
  // The SVG keeps the aspect ratio, so the drawing sits centered in the frame.
  const unit = size.w > 0 && size.h > 0 ? Math.min(size.w / box.w, size.h / box.h) : 0;
  const offsetX = (size.w - box.w * unit) / 2;
  const offsetY = (size.h - box.h * unit) / 2;
  const toGrid = (px: number, py: number) => ({ x: box.x + (px - offsetX) / unit, y: box.y + (py - offsetY) / unit });

  const reset = () => {
    'worklet';
    scale.value = withSpring(1, spring);
    savedScale.value = 1;
    tx.value = withSpring(0, spring);
    ty.value = withSpring(0, spring);
    savedX.value = 0;
    savedY.value = 0;
  };

  // Focus centers one table and zooms in on it. It is declarative on
  // purpose: the transform reads the target straight off the props, so the
  // canvas never writes a shared value outside a gesture.
  const focus = useMemo(() => {
    if (unit <= 0 || editable) return null;
    const table = focusId ? tables.find((t) => t.id === focusId) : null;
    if (!table) return null;
    const px = offsetX + (table.x + table.w / 2 - box.x) * unit;
    const py = offsetY + (table.y + table.h / 2 - box.y) * unit;
    // The transform applies around the view centre, so the offset is the
    // scaled distance from the table to it.
    return { x: -FOCUS_ZOOM * (px - size.w / 2), y: -FOCUS_ZOOM * (py - size.h / 2) };
  }, [focusId, tables, unit, editable, offsetX, offsetY, box.x, box.y, size.w, size.h]);

  // In edit mode the pan gesture drags a table instead of the canvas, so the
  // finger stays on the table it grabbed.
  const drag = Gesture.Pan()
    .enabled(!!editable && unit > 0)
    .runOnJS(true)
    .onStart((e) => {
      const point = toGrid(e.x, e.y);
      const hit = [...tables].reverse().find(
        (t) => point.x >= t.x && point.x <= t.x + t.w && point.y >= t.y && point.y <= t.y + t.h,
      );
      setGrabbed(hit ? { id: hit.id, x: hit.x, y: hit.y } : null);
    })
    .onUpdate((e) => {
      if (!grabbed) return;
      const t = tables.find((x) => x.id === grabbed.id);
      if (!t) return;
      const next = clampPosition(grabbed.x + e.translationX / unit, grabbed.y + e.translationY / unit, t.w, t.h);
      onTableMove?.(t.id, next.x, next.y);
    })
    .onFinalize(() => setGrabbed(null));

  const pinch = Gesture.Pinch()
    .enabled(!editable)
    .onUpdate((e) => {
      scale.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, savedScale.value * e.scale));
    })
    .onEnd(() => {
      savedScale.value = scale.value;
    });
  const pan = Gesture.Pan()
    .enabled(!editable)
    .minDistance(8)
    .onUpdate((e) => {
      tx.value = savedX.value + e.translationX;
      ty.value = savedY.value + e.translationY;
    })
    .onEnd(() => {
      savedX.value = tx.value;
      savedY.value = ty.value;
    });
  const doubleTap = Gesture.Tap()
    .enabled(!editable)
    .numberOfTaps(2)
    .onEnd(reset);

  const animated = useAnimatedStyle(() =>
    focus
      ? {
          flex: 1,
          transform: [
            { translateX: withTiming(focus.x, timing.slow) },
            { translateY: withTiming(focus.y, timing.slow) },
            { scale: withTiming(FOCUS_ZOOM, timing.slow) },
          ],
        }
      : {
          flex: 1,
          transform: [{ translateX: tx.value }, { translateY: ty.value }, { scale: scale.value }],
        },
  );

  const onLayout = (e: LayoutChangeEvent) =>
    setSize({ w: e.nativeEvent.layout.width, h: e.nativeEvent.layout.height });

  return (
    <View style={[styles.frame, fill ? styles.fill : styles.square]} onLayout={onLayout}>
      <GestureDetector gesture={Gesture.Simultaneous(drag, pinch, pan, doubleTap)}>
        <Animated.View style={animated}>
          {unit > 0 ? (
            <Svg
              width={size.w}
              height={size.h}
              viewBox={`${box.x} ${box.y} ${box.w} ${box.h}`}
              preserveAspectRatio="xMidYMid meet"
            >
              <Rect x={box.x} y={box.y} width={box.w} height={box.h} fill={color.bg} />
              {tables.map((t) => (
                <TableShape
                  key={t.id}
                  table={t}
                  status={statuses[t.id] ?? 'free'}
                  label={t.label}
                  sublabel={labels?.[t.id]}
                  progress={progress?.[t.id]}
                  selected={selectedId === t.id || grabbed?.id === t.id || focusId === t.id}
                  warn={warnIds?.has(t.id)}
                  urgent={urgentIds?.has(t.id)}
                  dimmed={dimIds?.has(t.id)}
                  onPress={() => onTablePress(t)}
                  onLongPress={onTableLongPress ? () => onTableLongPress(t) : undefined}
                />
              ))}
            </Svg>
          ) : null}
        </Animated.View>
      </GestureDetector>
    </View>
  );
}

const styles = StyleSheet.create({
  frame: {
    width: '100%',
    overflow: 'hidden',
    borderRadius: radius.lg,
    borderWidth: 1,
    borderColor: color.border,
    backgroundColor: color.bg,
    padding: space[1],
  },
  square: { aspectRatio: 1 },
  fill: { flex: 1 },
});
