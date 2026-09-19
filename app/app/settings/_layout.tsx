import { Redirect, Stack } from 'expo-router';

import { useAuthStore } from '../../src/store/auth';
import { color } from '../../src/ui/tokens';

// Settings are for owner and admin only, tech.md section 19.2.
export default function SettingsLayout() {
  const role = useAuthStore((s) => s.me?.staff.role);
  if (role !== 'owner' && role !== 'admin') return <Redirect href="/" />;
  return <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: color.bg } }} />;
}
