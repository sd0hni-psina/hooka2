import { Text as RNText, StyleSheet } from 'react-native';
import type { ReactNode } from 'react';

import { color as palette, font, monoFamily, type ColorToken } from './tokens';

export type TextVariant = 'h1' | 'h2' | 'body' | 'label' | 'mono';

type Props = {
  variant: TextVariant;
  color?: ColorToken;
  children: ReactNode;
  numberOfLines?: number;
};

export function Text({ variant, color = 'text', children, numberOfLines }: Props) {
  return (
    <RNText numberOfLines={numberOfLines} style={[styles[variant], { color: palette[color] }]}>
      {children}
    </RNText>
  );
}

const styles = StyleSheet.create({
  h1: { fontSize: font.h1.size, fontWeight: font.h1.weight },
  h2: { fontSize: font.h2.size, fontWeight: font.h2.weight },
  body: { fontSize: font.body.size, fontWeight: font.body.weight },
  label: { fontSize: font.label.size, fontWeight: font.label.weight },
  mono: { fontSize: font.mono.size, fontWeight: font.mono.weight, fontFamily: monoFamily, fontVariant: ['tabular-nums'] },
});
