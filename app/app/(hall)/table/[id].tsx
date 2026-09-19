import { useLocalSearchParams, useRouter } from 'expo-router';

import { t } from '../../../src/i18n/ru';
import { useHallStore } from '../../../src/store/hall';
import { AppBar, IconButton, Screen } from '../../../src/ui';

export default function TableScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const table = useHallStore((s) => s.state?.tables.find((x) => x.id === id));
  return (
    <Screen>
      <AppBar
        title={t.table.title(table?.label ?? '')}
        left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />}
      />
    </Screen>
  );
}
