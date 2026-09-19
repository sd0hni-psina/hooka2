import { t } from '../../src/i18n/ru';
import { AppBar, EmptyState, Screen } from '../../src/ui';

export default function TasksScreen() {
  return (
    <Screen>
      <AppBar title={t.tabs.tasks} />
      <EmptyState icon="list" title={t.tasks.empty} message={t.tasks.emptyHint} />
    </Screen>
  );
}
