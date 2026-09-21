import { memo } from 'react';
import { Ellipse, G, Rect, Text as SvgText } from 'react-native-svg';

import type { Table, TableStatus } from '../api/types';
import { statusStyle } from './status';
import { alpha, color, tone } from './tokens';

type Props = {
  table: Table;
  status: TableStatus;
  label: string;
  // sublabel is the line under the number: who took the task, or how long
  // the hookah has been running. It changes once a minute at most.
  sublabel?: string;
  // progress fills the table outline as the hookah burns its resource, 0..1.
  progress?: number;
  selected?: boolean;
  // warn marks a table that overlaps another one in the editor.
  warn?: boolean;
  // urgent draws a halo, so a red table is findable without reading colors.
  urgent?: boolean;
  // dimmed pushes back a table the search filtered out.
  dimmed?: boolean;
  onPress: () => void;
  onLongPress?: () => void;
};

// Perimeter of the outline, so strokeDasharray can fill exactly the share of
// the resource already burned. Ramanujan's approximation for the ellipse.
function perimeter(table: Table): number {
  if (table.shape === 'circle') {
    const a = table.w / 2;
    const b = table.h / 2;
    return Math.PI * (3 * (a + b) - Math.sqrt((3 * a + b) * (a + 3 * b)));
  }
  return 2 * (table.w + table.h);
}

// TableShape draws one table in hall grid units (tech.md section 7). It
// renders inside an Svg whose viewBox is the 100x100 hall grid.
function TableShapeView({
  table, status, label, sublabel, progress, selected, warn, urgent, dimmed, onPress, onLongPress,
}: Props) {
  const { x, y, w, h, rotation } = table;
  const cx = x + w / 2;
  const cy = y + h / 2;
  const skin = tone[status];
  const stroke = warn ? color.red : selected ? color.accent : skin.ring;
  const strokeWidth = selected || warn ? 0.7 : 0.35;
  const labelSize = Math.max(2.4, Math.min(w, h) * 0.4);
  const subSize = Math.max(1.5, labelSize * 0.42);
  const track = perimeter(table);
  const filled = progress === undefined ? 0 : track * Math.min(1, Math.max(0, progress));
  const a11y = `${label}: ${statusStyle[status].label}${sublabel ? `, ${sublabel}` : ''}`;
  const common = { fill: skin.fill, stroke, strokeWidth };

  return (
    <G
      onPress={onPress}
      onLongPress={onLongPress}
      accessibilityLabel={a11y}
      accessibilityRole="button"
      opacity={dimmed ? 0.22 : 1}
      transform={rotation ? `rotate(${rotation} ${cx} ${cy})` : undefined}
    >
      {urgent ? (
        <Ellipse cx={cx} cy={cy} rx={w / 2 + 1.6} ry={h / 2 + 1.6} fill={alpha('red', 0.12)} />
      ) : null}

      {table.shape === 'circle' ? (
        <Ellipse cx={cx} cy={cy} rx={w / 2} ry={h / 2} {...common} />
      ) : (
        <Rect x={x} y={y} width={w} height={h} rx={1.2} {...common} />
      )}

      {filled > 0 ? (
        // The outline itself is the gauge: a full border means a spent hookah.
        table.shape === 'circle' ? (
          <Ellipse
            cx={cx} cy={cy} rx={w / 2} ry={h / 2} fill="none"
            stroke={color[skin.fg]} strokeWidth={0.8} strokeLinecap="round"
            strokeDasharray={`${filled} ${track}`}
          />
        ) : (
          <Rect
            x={x} y={y} width={w} height={h} rx={1.2} fill="none"
            stroke={color[skin.fg]} strokeWidth={0.8} strokeLinecap="round"
            strokeDasharray={`${filled} ${track}`}
          />
        )
      ) : null}

      <SvgText
        x={cx}
        y={sublabel ? cy + labelSize * 0.1 : cy + labelSize * 0.35}
        fontSize={labelSize}
        fontWeight="700"
        fill={color.text}
        textAnchor="middle"
      >
        {label}
      </SvgText>
      {sublabel ? (
        <SvgText x={cx} y={cy + labelSize * 0.1 + subSize * 1.25} fontSize={subSize} fill={color[skin.fg]} textAnchor="middle">
          {sublabel}
        </SvgText>
      ) : null}
    </G>
  );
}

// Forty tables redraw only when their own props change: the map holds still
// while a single counter ticks, tech.md section 19.15.
export const TableShape = memo(TableShapeView);
