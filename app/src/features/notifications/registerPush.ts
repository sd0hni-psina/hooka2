import Constants from 'expo-constants';
import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';

import { registerPushToken } from '../../api/endpoints';

// Reason tells why this device has no push token, so a silent skip can be
// seen in the log instead of guessed at.
export type PushSkipReason = 'registered' | 'simulator' | 'denied' | 'no_project_id';

// registerPush asks for permission and sends the Expo token of this device to
// the server. Every skip is harmless: the app still works over WS while open.
export async function registerPush(): Promise<PushSkipReason> {
  const skip = (reason: PushSkipReason) => {
    if (__DEV__) console.warn(`push notifications off: ${reason}`);
    return reason;
  };
  if (!Device.isDevice) return skip('simulator');
  const current = await Notifications.getPermissionsAsync();
  const granted = current.granted || (await Notifications.requestPermissionsAsync()).granted;
  if (!granted) return skip('denied');
  const projectId = Constants.expoConfig?.extra?.eas?.projectId as string | undefined;
  // Without an EAS project Expo cannot issue a token, see tech.md section 19.2.
  if (!projectId) return skip('no_project_id');
  const { data } = await Notifications.getExpoPushTokenAsync({ projectId });
  await registerPushToken(data);
  return 'registered';
}
