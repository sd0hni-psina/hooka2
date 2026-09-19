import { request } from './client';
import { decodeMe } from './decode';
import type { Me } from './types';

export function getMe(): Promise<Me> {
  return request('GET', '/me', decodeMe);
}
