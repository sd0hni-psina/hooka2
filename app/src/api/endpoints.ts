import { request } from './client';
import {
  decodeAuthResult, decodeJoinCode, decodeJoinResult, decodeMe, decodeNothing, decodeRefresh,
  type AuthResult, type JoinResult,
} from './decode';
import type { Me, Platform, Tokens } from './types';

export type DeviceInfo = { device_id: string; platform: Platform };

export function getMe(): Promise<Me> {
  return request('GET', '/me', decodeMe);
}

export function join(code: string, displayName: string, device: DeviceInfo): Promise<JoinResult> {
  return request('POST', '/join', decodeJoinResult, { auth: false, body: { code, display_name: displayName, ...device } });
}

export function register(
  body: { email: string; password: string; display_name: string; venue_name: string },
  device: DeviceInfo,
): Promise<AuthResult> {
  return request('POST', '/auth/register', decodeAuthResult, { auth: false, body: { ...body, ...device } });
}

export function login(email: string, password: string, device: DeviceInfo): Promise<AuthResult> {
  return request('POST', '/auth/login', decodeAuthResult, { auth: false, body: { email, password, ...device } });
}

export function refreshTokens(refreshToken: string): Promise<Tokens> {
  return request('POST', '/auth/refresh', decodeRefresh, { auth: false, body: { refresh_token: refreshToken } });
}

export function logout(): Promise<void> {
  return request('POST', '/auth/logout', decodeNothing);
}

export function registerPushToken(token: string): Promise<void> {
  return request('POST', '/sessions/push-token', decodeNothing, { body: { expo_push_token: token } });
}

export function rotateCode(): Promise<string> {
  return request('POST', '/venues/current/rotate-code', decodeJoinCode);
}
