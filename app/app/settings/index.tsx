import { useRouter } from 'expo-router';

import { t } from '../../src/i18n/ru';
import { AppBar, IconButton, ListRow, Screen } from '../../src/ui';

export default function SettingsScreen() {
  const router = useRouter();
  return (
    <Screen scroll>
      <AppBar title={t.settings.title} left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />} />
      {__DEV__ ? <ListRow title={t.kitchen.title} onPress={() => router.push('/kitchen-sink')} /> : null}
    </Screen>
  );
}
