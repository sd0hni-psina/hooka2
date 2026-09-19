import { useState } from 'react';
import { StyleSheet, View, type LayoutChangeEvent } from 'react-native';
import { Gesture, GestureDetector } from 'react-native-gesture-handler';
import Animated, { useAnimatedStyle, useSharedValue } from 'react-native-reanimated';
import Svg, { Rect } from 'react-native-svg';

import type { Table, TableStatus } from '../api/types';
import { TableShape } from './TableShape';
import { color, radius } from './tokens';

// The hall is a 100x100 grid, tech.md section 7.
export const GRID = 100;
const MIN_ZOOM = 1;
const MAX_ZOOM = 4;

type Props = {
  tables: Table[];
  statuses: Record<string, TableStatus>;
  onTablePress: (table: Table) => void;
  editable?: boolean;
  onTableMove?: (tableId: string, x: number, y: number) => void;
  badges?: Record<string, string>;
  selectedId?: string | null;
};

export function HallCanvas({ tables, statuses, onTablePress, badges, selectedId }: Props) {
  const [size, setSize] = useState(0);
  const scale = useSharedValue(1);
  const savedScale = useSharedValue(1);
  const tx = useSharedValue(0);
  const ty = useSharedValue(0);
  const savedX = useSharedValue(0);
  const savedY = useSharedValue(0);

  const pinch = Gesture.Pinch()
    .onUpdate((e) => {
      scale.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, savedScale.value * e.scale));
    })
    .onEnd(() => {
      savedScale.value = scale.value;
    });
  const pan = Gesture.Pan()
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
      <GestureDetector gesture={Gesture.Simultaneous(pinch, pan, reset)}>
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
                  selected={selectedId === t.id}
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
