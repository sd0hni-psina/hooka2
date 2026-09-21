import { t } from '../../i18n/ru';

export type FieldErrors = Record<string, string>;

const reasons = t.fields;

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
