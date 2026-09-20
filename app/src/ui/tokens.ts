import { Platform } from 'react-native';

import type { TableStatus } from '../api/types';

export const color = {
  bg: '#0B0D11',
  surface: '#14171E',
  surfaceAlt: '#1D2129',
  surfaceHigh: '#252B36',
  border: '#2B3140',
  text: '#F4F6FA',
  textMuted: '#98A1B3',
  green: '#34D399',
  yellow: '#FBBF24',
  red: '#F87171',
  accent: '#5B9DFF',
  danger: '#F87171',
} as const;

export type ColorToken = keyof typeof color;

// alpha turns a palette hex into a translucent fill. Components never write
// rgba() themselves, tech.md section 19.15.
export function alpha(token: ColorToken, value: number): string {
  const hex = color[token];
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `rgba(${r}, ${g}, ${b}, ${value})`;
}

// tone is the single source for rendering a status: the word, the fill behind
// it and the ring around it all come from here.
export const tone: Record<TableStatus, { fg: ColorToken; fill: string; ring: string }> = {
  free: { fg: 'textMuted', fill: color.surfaceAlt, ring: color.border },
  green: { fg: 'green', fill: alpha('green', 0.16), ring: alpha('green', 0.4) },
  yellow: { fg: 'yellow', fill: alpha('yellow', 0.16), ring: alpha('yellow', 0.4) },
  red: { fg: 'red', fill: alpha('red', 0.16), ring: alpha('red', 0.4) },
};

export const space = [0, 4, 8, 12, 16, 24, 32, 48] as const;
export const radius = { sm: 6, md: 12, lg: 20, xl: 28, pill: 999 } as const;

type FontWeight = '400' | '500' | '600' | '700';
type FontSpec = { size: number; weight: FontWeight; line: number };

export const font: Record<'display' | 'h1' | 'h2' | 'body' | 'label' | 'mono', FontSpec> = {
  display: { size: 44, weight: '700', line: 48 },
  h1: { size: 28, weight: '700', line: 34 },
  h2: { size: 22, weight: '600', line: 28 },
  body: { size: 16, weight: '400', line: 22 },
  label: { size: 14, weight: '500', line: 18 },
  mono: { size: 18, weight: '600', line: 24 },
};

export const monoFamily = Platform.select({ ios: 'Menlo', default: 'monospace' });

// A master may run a large system font. Countdowns and the hall map stop
// scaling here so the layout still holds, tech.md section 19.15.
export const maxFontScale = 1.3;

export const hit = { min: 48 } as const;

// Scrim behind sheets and dialogs.
export const overlay = alpha('bg', 0.72);

export const elevation = {
  sheet: { shadowColor: '#000', shadowOpacity: 0.45, shadowRadius: 24, shadowOffset: { width: 0, height: -8 }, elevation: 16 },
  dialog: { shadowColor: '#000', shadowOpacity: 0.5, shadowRadius: 28, shadowOffset: { width: 0, height: 12 }, elevation: 20 },
  toast: { shadowColor: '#000', shadowOpacity: 0.4, shadowRadius: 16, shadowOffset: { width: 0, height: 6 }, elevation: 10 },
} as const;
