import Svg, { Circle, Path } from 'react-native-svg';

import { color as palette, type ColorToken } from './tokens';

// Stroke icons on a 24 grid. The set grows with the screens that need it.
const paths = {
  check: ['M5 12.5l4.5 4.5L19 7.5'],
  close: ['M6 6l12 12', 'M18 6L6 18'],
  plus: ['M12 5v14', 'M5 12h14'],
  minus: ['M5 12h14'],
  'chevron-left': ['M15 5l-7 7 7 7'],
  'chevron-right': ['M9 5l7 7-7 7'],
  flame: ['M12 3c1 3.5 5 5.5 5 10a5 5 0 0 1-10 0c0-2.5 1.5-4 2.5-5 .3 1.7 1.2 2.7 2.5 3-1-3 0-6 0-8z'],
  clock: ['M12 7v5l3 2', 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18z'],
  alert: ['M12 8v5', 'M12 16.5v.5', 'M10.3 3.9L2.5 17.5A2 2 0 0 0 4.2 20.5h15.6a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z'],
  hand: ['M8 12V6.5a1.5 1.5 0 0 1 3 0V11', 'M11 10.5V5a1.5 1.5 0 0 1 3 0v6', 'M14 10.5V7a1.5 1.5 0 0 1 3 0v7a7 7 0 0 1-7 7h-.5A6.5 6.5 0 0 1 4 16.5L5 13a1.5 1.5 0 0 1 3-.5'],
  swap: ['M4 8h14l-3-3', 'M20 16H6l3 3'],
  stop: ['M7 7h10v10H7z'],
  settings: ['M12 9a3 3 0 1 1 0 6 3 3 0 0 1 0-6z', 'M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.9 1.2V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-2.9-1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.7 1.7 0 0 0 3 14.1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.2-2.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.7 1.7 0 0 0 10 3.2V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0 1.2 2.9h.1a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z'],
  users: ['M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z', 'M2 21v-1a6 6 0 0 1 12 0v1', 'M16 3.5a4 4 0 0 1 0 7', 'M18 14.5a6 6 0 0 1 4 5.5v1'],
  user: ['M12 12a4.5 4.5 0 1 0 0-9 4.5 4.5 0 0 0 0 9z', 'M4 21a8 8 0 0 1 16 0'],
  grid: ['M4 4h7v7H4z', 'M13 4h7v7h-7z', 'M4 13h7v7H4z', 'M13 13h7v7h-7z'],
  list: ['M8 6h12', 'M8 12h12', 'M8 18h12', 'M4 6h.5', 'M4 12h.5', 'M4 18h.5'],
  'wifi-off': ['M3 3l18 18', 'M8.5 16.5a5 5 0 0 1 7 0', 'M5 12.5a10 10 0 0 1 5-2.6', 'M19 12.5a10 10 0 0 0-2.3-1.6', 'M2 8.8a15 15 0 0 1 4.3-2.6', 'M22 8.8A15 15 0 0 0 11 5', 'M12 20h.01'],
  refresh: ['M20 11a8 8 0 1 0-2.3 5.7', 'M20 5v6h-6'],
  edit: ['M4 20h4L19 9l-4-4L4 16z'],
  trash: ['M4 7h16', 'M9 7V4h6v3', 'M6 7l1 13h10l1-13'],
  info: ['M12 11v6', 'M12 7.5v.5', 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18z'],
} as const;

export type IconName = keyof typeof paths | 'dot';

type Props = { name: IconName; size?: number; color?: ColorToken };

export function Icon({ name, size = 24, color = 'text' }: Props) {
  const stroke = palette[color];
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      {name === 'dot' ? (
        <Circle cx={12} cy={12} r={5} fill={stroke} />
      ) : (
        paths[name].map((d) => (
          <Path key={d} d={d} stroke={stroke} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
        ))
      )}
    </Svg>
  );
}
