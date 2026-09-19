import { ApiError, NetworkError } from '../../../api/errors';
import { t } from '../../../i18n/ru';
import { describeError, validateJoin, validateRegister } from '../form';

describe('validateJoin', () => {
  it('accepts four digits and a name', () => {
    expect(validateJoin('0421', ' Ерлан ')).toEqual({});
  });
  it('flags a short code and an empty name', () => {
    expect(validateJoin('12', '  ')).toEqual({ code: t.fields.invalid, display_name: t.fields.required });
  });
  it('flags a name over 40 characters', () => {
    expect(validateJoin('1234', 'я'.repeat(41)).display_name).toBe(t.fields.too_long);
  });
});

describe('validateRegister', () => {
  it('needs an 8 character password', () => {
    expect(validateRegister({ email: 'a@b.kz', password: 'short', display_name: 'А', venue_name: 'Б' })).toEqual({
      password: t.fields.too_short,
    });
  });
});

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
