import { Circle, Ellipse, G, Rect, Text as SvgText } from 'react-native-svg';

import type { Table, TableStatus } from '../api/types';
import { statusStyle } from './status';
import { color } from './tokens';

type Props = {
  table: Table;
  status: TableStatus;
  label: string;
  badge?: string;
  selected?: boolean;
  onPress: () => void;
  onLongPress?: () => void;
};

// TableShape draws one table in hall grid units (tech.md section 7). It
// renders inside an Svg whose viewBox is the 100x100 hall grid.
export function TableShape({ table, status, label, badge, selected, onPress, onLongPress }: Props) {
  const { x, y, w, h, rotation } = table;
  const cx = x + w / 2;
  const cy = y + h / 2;
  const tone = color[statusStyle[status].color];
  const fill = status === 'free' ? color.surfaceAlt : tone;
  const stroke = selected ? color.accent : tone;
  const strokeWidth = selected ? 0.8 : 0.4;
  const labelSize = Math.max(2.2, Math.min(w, h) * 0.38);
  const a11y = `${label}: ${statusStyle[status].label}${badge ? `, ${badge}` : ''}`;

  return (
    <G
      onPress={onPress}
      onLongPress={onLongPress}
      accessibilityLabel={a11y}
      accessibilityRole="button"
      transform={rotation ? `rotate(${rotation} ${cx} ${cy})` : undefined}
    >
      {table.shape === 'circle' ? (
        <Ellipse cx={cx} cy={cy} rx={w / 2} ry={h / 2} fill={fill} fillOpacity={status === 'free' ? 1 : 0.28} stroke={stroke} strokeWidth={strokeWidth} />
      ) : (
        <Rect x={x} y={y} width={w} height={h} rx={1} fill={fill} fillOpacity={status === 'free' ? 1 : 0.28} stroke={stroke} strokeWidth={strokeWidth} />
      )}
      {status === 'free' ? null : <Circle cx={x + w - 1.2} cy={y + 1.2} r={0.8} fill={tone} />}
      <SvgText x={cx} y={cy + labelSize * 0.35} fontSize={labelSize} fontWeight="700" fill={color.text} textAnchor="middle">
        {label}
      </SvgText>
      {badge ? (
        <SvgText x={cx} y={y + h - 0.8} fontSize={Math.max(1.6, labelSize * 0.45)} fill={color.text} textAnchor="middle">
          {badge}
        </SvgText>
      ) : null}
    </G>
  );
}
