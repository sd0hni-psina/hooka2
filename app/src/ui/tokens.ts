import { Platform } from 'react-native';

export const color = {
  bg: '#0E0F13',
  surface: '#171A21',
  surfaceAlt: '#1F2430',
  border: '#2A3040',
  text: '#F2F4F8',
  textMuted: '#9AA3B2',
  green: '#2ECC71',
  yellow: '#F1C40F',
  red: '#E74C3C',
  accent: '#4C8DFF',
  danger: '#E74C3C',
} as const;

export type ColorToken = keyof typeof color;

export const space = [0, 4, 8, 12, 16, 24, 32, 48] as const;
export const radius = { sm: 6, md: 12, lg: 20, pill: 999 } as const;

type FontWeight = '400' | '500' | '600' | '700';
export const font: Record<'h1' | 'h2' | 'body' | 'label' | 'mono', { size: number; weight: FontWeight }> = {
  h1: { size: 28, weight: '700' },
  h2: { size: 22, weight: '600' },
  body: { size: 16, weight: '400' },
  label: { size: 14, weight: '500' },
  mono: { size: 18, weight: '600' },
};

export const monoFamily = Platform.select({ ios: 'Menlo', default: 'monospace' });

export const hit = { min: 48 } as const;

// Scrim behind sheets and dialogs: bg at 70% opacity.
export const overlay = 'rgba(14, 15, 19, 0.7)';
