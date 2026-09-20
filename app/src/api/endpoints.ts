import { request } from './client';
import {
  arr, decodeAuthResult, decodeEvent, decodeHall, decodeHallState, decodeHookah, decodeJoinCode, decodeJoinResult,
  decodeMe, decodeNothing, decodePlan, decodeRefresh, decodeStaff, decodeTable, type AuthResult, type JoinResult,
} from './decode';
import type { Placement } from '../domain/layout';
import type {
  FinishReason, Hall, HallState, Hookah, Me, Plan, Platform, Role, ServiceEvent, Shape, Staff, Table, Tokens,
} from './types';

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

export function createHall(name: string, sortOrder: number): Promise<Hall> {
  return request('POST', '/halls', decodeHall, { body: { name, sort_order: sortOrder } });
}

export function updateHall(hallId: string, patch: { name?: string; sort_order?: number }): Promise<Hall> {
  return request('PATCH', `/halls/${hallId}`, decodeHall, { body: patch });
}

export function deleteHall(hallId: string): Promise<void> {
  return request('DELETE', `/halls/${hallId}`, decodeNothing);
}

export type TableInput = {
  label: string;
  shape: Shape;
  x: number;
  y: number;
  w: number;
  h: number;
  rotation: number;
  seats: number;
};

export function createTable(hallId: string, table: TableInput): Promise<Table> {
  return request('POST', `/halls/${hallId}/tables`, decodeTable, { body: table });
}

export function patchTable(tableId: string, patch: Partial<TableInput> & { archived?: boolean }): Promise<Table> {
  return request('PATCH', `/tables/${tableId}`, decodeTable, { body: patch });
}

export function archiveTable(tableId: string): Promise<void> {
  return request('DELETE', `/tables/${tableId}`, decodeNothing);
}

export function setLayout(hallId: string, tables: Placement[]): Promise<Table[]> {
  return request('PUT', `/halls/${hallId}/layout`, (u) => arr({ tables: u }, 'tables', 'layout', decodeTable), { body: { tables } });
}

export function getHallState(hallId: string): Promise<HallState> {
  return request('GET', `/halls/${hallId}/state`, decodeHallState);
}

// startHookah must carry the same key on every retry, tech.md section 19.8.
export function startHookah(tableId: string, idempotencyKey: string): Promise<Hookah> {
  return request('POST', `/tables/${tableId}/hookah`, decodeHookah, { headers: { 'Idempotency-Key': idempotencyKey } });
}

export function transferHookah(hookahId: string, toTableId: string): Promise<Hookah> {
  return request('POST', `/hookahs/${hookahId}/transfer`, decodeHookah, { body: { to_table_id: toTableId } });
}

export function finishHookah(hookahId: string, reason: FinishReason): Promise<Hookah> {
  return request('POST', `/hookahs/${hookahId}/finish`, decodeHookah, { body: { reason } });
}

export function claimEvent(eventId: string): Promise<ServiceEvent> {
  return request('POST', `/service-events/${eventId}/claim`, decodeEvent);
}

export function confirmEvent(eventId: string): Promise<ServiceEvent> {
  return request('POST', `/service-events/${eventId}/confirm`, decodeEvent);
}

export function getServicePlan(): Promise<Plan> {
  return request('GET', '/venues/current/service-plan', decodePlan);
}

export function setServicePlan(plan: Plan): Promise<Plan> {
  return request('PUT', '/venues/current/service-plan', decodePlan, { body: plan });
}

export function getStaff(): Promise<Staff[]> {
  return request('GET', '/venues/current/staff', (u) => arr({ staff: u }, 'staff', 'staff', decodeStaff));
}

export function patchStaff(staffId: string, patch: { role?: Role; blocked?: boolean }): Promise<Staff> {
  return request('PATCH', `/staff/${staffId}`, decodeStaff, { body: patch });
}
