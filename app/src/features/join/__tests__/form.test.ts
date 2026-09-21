import { t } from '../../../i18n/ru';
import { validateJoin, validateRegister } from '../form';

describe('validateJoin', () => {
  it('accepts six digits, leading zeros included, and a name', () => {
    expect(validateJoin('042100', ' Ерлан ')).toEqual({});
  });
  it.each(['1234', '12345', '1234567', '12345a', '12 345', ''])('rejects the code "%s"', (code) => {
    expect(validateJoin(code, 'Ерлан')).toEqual({ code: t.fields.invalid });
  });
  it('flags a short code and an empty name', () => {
    expect(validateJoin('12', '  ')).toEqual({ code: t.fields.invalid, display_name: t.fields.required });
  });
  it('flags a name over 40 characters', () => {
    expect(validateJoin('123456', 'я'.repeat(41)).display_name).toBe(t.fields.too_long);
  });
});

describe('validateRegister', () => {
  it('needs an 8 character password', () => {
    expect(validateRegister({ email: 'a@b.kz', password: 'short', display_name: 'А', venue_name: 'Б' })).toEqual({
      password: t.fields.too_short,
    });
  });
});
