import { ApiError, NetworkError } from '../../api/errors';
import { t } from '../../i18n/ru';

export type FieldErrors = Record<string, string>;

const reasons = t.fields;
type Reason = keyof typeof reasons;

function isReason(r: unknown): r is Reason {
  return typeof r === 'string' && r in reasons;
}

// validateJoin mirrors the server rules of tech.md section 19.1 so the master
// sees mistakes before the request.
export function validateJoin(code: string, name: string): FieldErrors {
  const e: FieldErrors = {};
  if (!/^\d{4}$/.test(code)) e.code = reasons.invalid;
  const n = name.trim();
  if (n.length === 0) e.display_name = reasons.required;
  else if ([...n].length > 40) e.display_name = reasons.too_long;
  return e;
}

export function validateLogin(email: string, password: string): FieldErrors {
  const e: FieldErrors = {};
  if (!email.trim()) e.email = reasons.required;
  else if (!email.includes('@')) e.email = reasons.invalid;
  if (!password) e.password = reasons.required;
  return e;
}

export function validateRegister(v: { email: string; password: string; display_name: string; venue_name: string }): FieldErrors {
  const e = validateLogin(v.email, v.password);
  if (!e.password && v.password.length < 8) e.password = reasons.too_short;
  if (!v.display_name.trim()) e.display_name = reasons.required;
  if (!v.venue_name.trim()) e.venue_name = reasons.required;
  return e;
}

// describeError turns an API failure into a form-level message and field errors.
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
        for (const [k, r] of Object.entries(raw)) fields[k] = isReason(r) ? reasons[r] : reasons.invalid;
      }
      return { message: null, fields };
    }
    default:
      return { message: err.message, fields: {} };
  }
}
