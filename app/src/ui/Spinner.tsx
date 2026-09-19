import { ActivityIndicator } from 'react-native';

import { color } from './tokens';

type Props = { size?: 'small' | 'large' };

export function Spinner({ size = 'small' }: Props) {
  return <ActivityIndicator size={size} color={color.accent} />;
}
