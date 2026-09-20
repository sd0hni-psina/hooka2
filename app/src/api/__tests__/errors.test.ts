import { ApiError, describeError, explain, NetworkError } from '../errors';
import { t } from '../../i18n/ru';

describe('describeError', () => {
  it('puts a wrong code on the code field', () => {
    expect(describeError(new ApiError('invalid_join_code', 404, 'x', null)).fields.code).toBe(t.errors.invalidCode);
  });
  it('shows the wait from retry_after_sec', () => {
    expect(describeError(new ApiError('rate_limited', 429, 'x', { retry_after_sec: 42 })).message).toBe(t.errors.rateLimited(42));
  });
  it('maps validation reasons per field', () => {
    const e = new ApiError('validation_failed', 400, 'x', { fields: { email: 'taken', password: 'weird' } });
    expect(describeError(e).fields).toEqual({ email: t.fields.taken, password: t.fields.invalid });
  });
  it('explains a missing network', () => {
    expect(describeError(new NetworkError(new Error('down'))).message).toBe(t.errors.network);
  });
});

describe('explain', () => {
  it('always returns a line to show', () => {
    expect(explain(new NetworkError(new Error('down')))).toBe(t.errors.network);
    expect(explain(new ApiError('invalid_join_code', 404, 'x', null))).toBe(t.errors.invalidCode);
    expect(explain({ nothing: true })).toBe(t.errors.unknown);
  });
});
