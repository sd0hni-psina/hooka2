import { useRouter } from 'expo-router';

import { t } from '../../src/i18n/ru';
import { AppBar, IconButton, ListRow, Screen } from '../../src/ui';

export default function SettingsScreen() {
  const router = useRouter();
  return (
    <Screen scroll>
      <AppBar title={t.settings.title} left={<IconButton icon="chevron-left" accessibilityLabel={t.ui.close} onPress={() => router.back()} />} />
      <ListRow title={t.settings.layout} subtitle={t.layout.hint} onPress={() => router.push('/settings/layout')} />
      <ListRow title={t.settings.plan} subtitle={t.plan.hint} onPress={() => router.push('/settings/plan')} />
      <ListRow title={t.settings.staff} subtitle={t.staff.emptyHint} onPress={() => router.push('/settings/staff')} />
      {__DEV__ ? <ListRow title={t.kitchen.title} onPress={() => router.push('/kitchen-sink')} /> : null}
    </Screen>
  );
}
