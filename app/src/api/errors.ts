import { obj, str } from './decode';

// Closed list of tech.md section 8.
export const errorCodes = [
  'validation_failed', 'unauthorized', 'forbidden', 'not_found', 'invalid_join_code', 'rate_limited',
  'table_occupied', 'table_busy', 'hookah_finished', 'event_not_actionable', 'plan_invalid', 'internal',
] as const;
export type ErrorCode = (typeof errorCodes)[number];

export class ApiError extends Error {
  constructor(
    readonly code: ErrorCode,
    readonly status: number,
    message: string,
    readonly details: Record<string, unknown> | null,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// NetworkError means the request never got an HTTP answer.
export class NetworkError extends Error {
  constructor(cause: unknown) {
    super(cause instanceof Error ? cause.message : 'network error');
    this.name = 'NetworkError';
  }
}

export function parseApiError(status: number, body: unknown): ApiError {
  try {
    const e = obj(obj(body, 'body').error, 'error');
    const raw = str(e, 'code', 'error');
    const code: ErrorCode = (errorCodes as readonly string[]).includes(raw) ? (raw as ErrorCode) : 'internal';
    const details = e.details === undefined || e.details === null ? null : obj(e.details, 'error.details');
    return new ApiError(code, status, str(e, 'message', 'error'), details);
  } catch {
    return new ApiError('internal', status, `HTTP ${status}`, null);
  }
}
