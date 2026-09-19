import me from '../../../../contracts/fixtures/me.json';
import { decodeMe } from '../decode';

describe('GET /me against the shared fixture', () => {
  it('decodes every field of the fixture', () => {
    expect(decodeMe(me)).toEqual(me);
  });

  it('rejects an unknown role', () => {
    expect(() => decodeMe({ ...me, staff: { ...me.staff, role: 'root' } })).toThrow(/role/);
  });
});
