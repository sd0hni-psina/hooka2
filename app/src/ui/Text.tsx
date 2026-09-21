import { Text as RNText, StyleSheet, type TextStyle } from 'react-native';
import type { ReactNode } from 'react';

import { color as palette, font, maxFontScale, monoFamily, type ColorToken } from './tokens';

export type TextVariant = 'display' | 'h1' | 'h2' | 'body' | 'label' | 'mono';

type Props = {
  variant: TextVariant;
  color?: ColorToken;
  children: ReactNode;
  numberOfLines?: number;
  align?: TextStyle['textAlign'];
};

// Counters and headings stop scaling at maxFontScale: a master running a
// large system font must still see a hall that fits, tech.md section 19.15.
const capped: Record<TextVariant, boolean> = {
  display: true, h1: true, h2: true, body: false, label: false, mono: true,
};

export function Text({ variant, color = 'text', children, numberOfLines, align }: Props) {
  return (
    <RNText
      numberOfLines={numberOfLines}
      maxFontSizeMultiplier={capped[variant] ? maxFontScale : undefined}
      style={[styles[variant], { color: palette[color] }, align ? { textAlign: align } : null]}
    >
      {children}
    </RNText>
  );
}

const numeric: TextStyle = { fontVariant: ['tabular-nums'] };

const styles = StyleSheet.create({
  display: { fontSize: font.display.size, fontWeight: font.display.weight, lineHeight: font.display.line, ...numeric },
  h1: { fontSize: font.h1.size, fontWeight: font.h1.weight, lineHeight: font.h1.line },
  h2: { fontSize: font.h2.size, fontWeight: font.h2.weight, lineHeight: font.h2.line },
  body: { fontSize: font.body.size, fontWeight: font.body.weight, lineHeight: font.body.line },
  label: { fontSize: font.label.size, fontWeight: font.label.weight, lineHeight: font.label.line },
  mono: {
    fontSize: font.mono.size, fontWeight: font.mono.weight, lineHeight: font.mono.line,
    fontFamily: monoFamily, ...numeric,
  },
});
