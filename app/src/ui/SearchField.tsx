import { StyleSheet, TextInput, View, type KeyboardTypeOptions } from 'react-native';

import { Icon } from './Icon';
import { IconButton } from './IconButton';
import { color, font, hit, radius, space } from './tokens';
import { t } from '../i18n/ru';

type Props = {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  keyboardType?: KeyboardTypeOptions;
  autoFocus?: boolean;
};

// Finding table 22 among forty by eye takes longer than the whole service
// action. SearchField is the shortcut, tech.md section 19.15.
export function SearchField({ value, onChange, placeholder, keyboardType, autoFocus }: Props) {
  return (
    <View style={styles.row}>
      <Icon name="search" size={20} color="textMuted" />
      <TextInput
        accessibilityLabel={placeholder}
        value={value}
        onChangeText={onChange}
        placeholder={placeholder}
        placeholderTextColor={color.textMuted}
        keyboardType={keyboardType}
        autoFocus={autoFocus}
        autoCorrect={false}
        style={styles.input}
      />
      {value.length > 0 ? (
        <IconButton icon="close" accessibilityLabel={t.ui.clear} onPress={() => onChange('')} />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: space[2],
    paddingLeft: space[3],
    minHeight: hit.min,
    borderRadius: radius.md,
    borderWidth: 1,
    borderColor: color.border,
    backgroundColor: color.surface,
  },
  input: { flex: 1, color: color.text, fontSize: font.body.size, paddingVertical: space[2] },
});
