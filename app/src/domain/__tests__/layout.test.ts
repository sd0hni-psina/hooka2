import type { Table } from '../../api/types';
import {
  applyPreset, changedPlacements, clampPosition, overlappingIds, overlaps, presetOf, resize, snapToGrid,
  type Placement,
} from '../layout';

const at = (id: string, x: number, y: number, w = 10, h = 10, rotation = 0): Placement => ({ id, x, y, w, h, rotation });

describe('snapToGrid', () => {
  it('rounds to whole units', () => {
    expect(snapToGrid(12.4)).toBe(12);
    expect(snapToGrid(12.5)).toBe(13);
    expect(snapToGrid(-0.4)).toBe(-0);
  });
});

describe('clampPosition', () => {
  it('keeps a table inside the grid', () => {
    expect(clampPosition(-5, 3.6, 10, 6)).toEqual({ x: 0, y: 4 });
    expect(clampPosition(97, 99, 10, 6)).toEqual({ x: 90, y: 94 });
  });
});

describe('overlaps', () => {
  it('sees two tables sharing area', () => {
    expect(overlaps(at('a', 0, 0), at('b', 5, 5))).toBe(true);
  });
  it('lets tables touch edge to edge', () => {
    expect(overlaps(at('a', 0, 0), at('b', 10, 0))).toBe(false);
  });
  it('sees an overlap that appears only after rotation', () => {
    // b sits right below a, edge to edge; turned 90 degrees it reaches into a.
    expect(overlaps(at('a', 0, 0, 10, 4), at('b', 4, 4, 10, 4))).toBe(false);
    expect(overlaps(at('a', 0, 0, 10, 4), at('b', 4, 4, 10, 4, 90))).toBe(true);
  });
  it('lists every table of an overlapping pair', () => {
    const ids = overlappingIds([at('a', 0, 0), at('b', 5, 5), at('c', 60, 60)]);
    expect([...ids].sort()).toEqual(['a', 'b']);
  });
});

describe('changedPlacements', () => {
  const saved: Table[] = [
    { id: 'a', hall_id: 'h', label: '1', shape: 'rect', x: 0, y: 0, w: 10, h: 10, rotation: 0, seats: 4 },
    { id: 'b', hall_id: 'h', label: '2', shape: 'rect', x: 20, y: 0, w: 10, h: 10, rotation: 0, seats: 4 },
  ];
  it('sends only the tables that moved', () => {
    const current = [at('a', 0, 0), at('b', 25, 0)];
    expect(changedPlacements(current, saved).map((p) => p.id)).toEqual(['b']);
  });
  it('sends a table that is not on the server yet', () => {
    expect(changedPlacements([at('c', 0, 0)], saved).map((p) => p.id)).toEqual(['c']);
  });
});

describe('shape presets', () => {
  const rect = { shape: 'rect' as const, w: 12, h: 8 };

  it('names what the table looks like', () => {
    expect(presetOf(rect)).toBe('rect');
    expect(presetOf({ shape: 'rect', w: 10, h: 10 })).toBe('square');
    expect(presetOf({ shape: 'circle', w: 10, h: 10 })).toBe('circle');
  });

  it('keeps the longer side when the shape changes', () => {
    expect(applyPreset(rect, 'circle')).toEqual({ shape: 'circle', w: 12, h: 12 });
    expect(applyPreset(rect, 'square')).toEqual({ shape: 'rect', w: 12, h: 12 });
    expect(applyPreset({ shape: 'circle', w: 10, h: 10 }, 'rect')).toEqual({ shape: 'rect', w: 10, h: 6 });
  });

  it('resizes a circle and a square symmetrically', () => {
    expect(resize({ shape: 'circle', w: 10, h: 10 }, 'w', 14)).toEqual({ shape: 'circle', w: 14, h: 14 });
    expect(resize({ shape: 'rect', w: 10, h: 10 }, 'h', 6)).toEqual({ shape: 'rect', w: 6, h: 6 });
    expect(resize(rect, 'h', 5)).toEqual({ shape: 'rect', w: 12, h: 5 });
  });

  it('keeps sides inside the allowed range', () => {
    expect(resize(rect, 'w', 0)).toEqual({ shape: 'rect', w: 1, h: 8 });
    expect(resize(rect, 'w', 99)).toEqual({ shape: 'rect', w: 40, h: 8 });
  });
});
