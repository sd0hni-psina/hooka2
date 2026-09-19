import { useState } from 'react';
import { StyleSheet, View, type LayoutChangeEvent } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, { useAnimatedStyle, useSharedValue } from 'react-native-reanimated';
import Svg, { Rect } from 'react-native-svg';

import type { Table, TableStatus } from '../api/types';
import { clampPosition, GRID } from '../domain/layout';
import { TableShape } from './TableShape';
import { color, radius } from './tokens';

const MIN_ZOOM = 1;
const MAX_ZOOM = 4;

type Props = {
  tables: Table[];
  statuses: Record<string, TableStatus>;
  onTablePress: (table: Table) => void;
  editable?: boolean;
  onTableMove?: (tableId: string, x: number, y: number) => void;
  badges?: Record<string, string>;
  warnIds?: Set<string>;
  selectedId?: string | null;
};

export function HallCanvas({ tables, statuses, onTablePress, editable, onTableMove, badges, warnIds, selectedId }: Props) {
  const [size, setSize] = useState(0);
  // The grabbed table and where it stood when the finger landed. Gesture
  // callbacks are rebuilt on every render, so plain state is enough.
  const [grabbed, setGrabbed] = useState<{ id: string; x: number; y: number } | null>(null);

  const scale = useSharedValue(1);
  const savedScale = useSharedValue(1);
  const tx = useSharedValue(0);
  const ty = useSharedValue(0);
  const savedX = useSharedValue(0);
  const savedY = useSharedValue(0);

  const unit = size > 0 ? size / GRID : 0;

  // In edit mode the pan gesture drags a table instead of the canvas, so the
  // finger stays on the table it grabbed.
  const drag = Gesture.Pan()
    .enabled(!!editable && unit > 0)
    .runOnJS(true)
    .onStart((e) => {
      const gx = e.x / unit;
      const gy = e.y / unit;
      const hit = [...tables].reverse().find((t) => gx >= t.x && gx <= t.x + t.w && gy >= t.y && gy <= t.y + t.h);
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
  const reset = Gesture.Tap()
    .enabled(!editable)
    .numberOfTaps(2)
    .onEnd(() => {
      scale.value = 1;
      savedScale.value = 1;
      tx.value = 0;
      ty.value = 0;
      savedX.value = 0;
      savedY.value = 0;
    });

  const animated = useAnimatedStyle(() => ({
    transform: [{ translateX: tx.value }, { translateY: ty.value }, { scale: scale.value }],
  }));

  const onLayout = (e: LayoutChangeEvent) => setSize(e.nativeEvent.layout.width);

  return (
    <View style={styles.frame} onLayout={onLayout}>
      <GestureDetector gesture={Gesture.Simultaneous(drag, pinch, pan, reset)}>
        <Animated.View style={animated}>
          {size > 0 ? (
            <Svg width={size} height={size} viewBox={`0 0 ${GRID} ${GRID}`}>
              <Rect x={0} y={0} width={GRID} height={GRID} fill={color.bg} />
              {tables.map((t) => (
                <TableShape
                  key={t.id}
                  table={t}
                  status={statuses[t.id] ?? 'free'}
                  label={t.label}
                  badge={badges?.[t.id]}
                  selected={selectedId === t.id || grabbed?.id === t.id}
                  warn={warnIds?.has(t.id)}
                  onPress={() => onTablePress(t)}
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
    aspectRatio: 1,
    overflow: 'hidden',
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: color.border,
    backgroundColor: color.bg,
  },
});
