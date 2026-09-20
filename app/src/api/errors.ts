import { obj, str } from './decode';
import { t } from '../i18n/ru';

export type FieldErrors = Record<string, string>;

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

const fieldReasons = t.fields;
type Reason = keyof typeof fieldReasons;

function isReason(r: unknown): r is Reason {
  return typeof r === 'string' && r in fieldReasons;
}

// describeError turns an API failure into a form-level message and field
// errors. It lives with the client that produced the failure, not with the
// join form that happened to need it first, tech.md section 19.15.
export function describeError(err: unknown): { message: string | null; fields: FieldErrors } {
  if (err instanceof NetworkError) return { message: t.errors.network, fields: {} };
  if (!(err instanceof ApiError)) return { message: t.errors.unknown, fields: {} };
  switch (err.code) {
    case 'invalid_join_code':
      return { message: null, fields: { code: t.errors.invalidCode } };
    case 'rate_limited': {
      const sec = typeof err.details?.retry_after_sec === 'number' ? err.details.retry_after_sec : 60;
      return { message: t.errors.rateLimited(sec), fields: {} };
    }
    case 'unauthorized':
      return { message: t.errors.wrongCredentials, fields: {} };
    case 'forbidden':
      return { message: t.errors.blocked, fields: {} };
    case 'validation_failed': {
      const fields: FieldErrors = {};
      const raw = err.details?.fields;
      if (typeof raw === 'object' && raw !== null) {
        for (const [k, r] of Object.entries(raw)) fields[k] = isReason(r) ? fieldReasons[r] : fieldReasons.invalid;
      }
      return { message: null, fields };
    }
    default:
      return { message: err.message, fields: {} };
  }
}

// explain is describeError for a toast: one line, always present.
export function explain(err: unknown): string {
  const d = describeError(err);
  return d.message ?? Object.values(d.fields)[0] ?? t.errors.unknown;
}
