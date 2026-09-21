import { StyleSheet, View } from 'react-native';

import { t } from '../../i18n/ru';
import { useHallStore } from '../../store/hall';
import { Badge } from '../../ui';
import { space } from '../../ui/tokens';

// One line that answers: can I trust what I see, and did my last tap land.
export function ConnectionBar() {
  const connection = useHallStore((s) => s.connection);
  const stale = useHallStore((s) => s.stale);
  const savedAt = useHallStore((s) => s.savedAt);
  const queued = useHallStore((s) => s.queued);

  const time = savedAt ? new Date(savedAt).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : null;

  return (
    <View style={styles.row}>
      {connection === 'online' ? (
        <Badge title={t.hall.online} tone="green" icon="check" />
      ) : connection === 'offline' ? (
        <Badge title={t.hall.offline} tone="red" icon="wifi-off" />
      ) : (
        <Badge title={t.hall.connecting} tone="neutral" icon="refresh" />
      )}
      {stale && time ? <Badge title={t.hall.stale(time)} tone="yellow" icon="clockOff" /> : null}
      {queued > 0 ? <Badge title={t.hall.queued(queued)} tone="accent" icon="refresh" /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: space[2], flexWrap: 'wrap' },
});
