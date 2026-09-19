import * as Crypto from 'expo-crypto';
import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

import type { DeviceInfo } from './endpoints';

const KEY = 'hookah.device.v1';

// deviceInfo returns a random id kept for the life of the install, so a
// repeated join from this phone reuses its session (tech.md section 19.1).
export async function deviceInfo(): Promise<DeviceInfo> {
  let id = await SecureStore.getItemAsync(KEY);
  if (!id) {
    id = Crypto.randomUUID();
    await SecureStore.setItemAsync(KEY, id);
  }
  return { device_id: id, platform: Platform.OS === 'ios' ? 'ios' : 'android' };
}
