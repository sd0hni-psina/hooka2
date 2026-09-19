import { t } from '../src/i18n/ru';
import { AppBar, EmptyState, Screen } from '../src/ui';

export default function JoinScreen() {
  return (
    <Screen>
      <AppBar title={t.join.title} />
      <EmptyState icon="user" title={t.join.title} message={t.join.soon} />
    </Screen>
  );
}
