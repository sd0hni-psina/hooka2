import { StyleSheet, View } from 'react-native';

import { useHallStore } from '../../store/hall';
import { Badge } from '../../ui';
import { space } from '../../ui/tokens';
import { connectionLine } from './connectionLine';

// One line that answers: can I trust what I see, and did my last tap land.
export function ConnectionBar() {
  const connection = useHallStore((s) => s.connection);
  const stale = useHallStore((s) => s.stale);
  const savedAt = useHallStore((s) => s.savedAt);
  const queued = useHallStore((s) => s.queued);

  const time = savedAt ? new Date(savedAt).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : null;

  return (
    <View style={styles.row}>
      {connectionLine({ connection, stale, time, queued }).map((line) => (
        <Badge key={line.key} title={line.title} tone={line.tone} icon={line.icon} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: space[2], flexWrap: 'wrap' },
});
