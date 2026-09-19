import type { Table } from '../api/types';

// The hall is a GRID x GRID unit square, tech.md section 7.
export const GRID = 100;
export const MAX_SIDE = 40;

export type Placement = { id: string; x: number; y: number; w: number; h: number; rotation: number };

export function snapToGrid(value: number): number {
  return Math.round(value);
}

// clampPosition keeps a table fully inside the grid, tech.md section 19.6.
export function clampPosition(x: number, y: number, w: number, h: number): { x: number; y: number } {
  return {
    x: Math.min(GRID - w, Math.max(0, snapToGrid(x))),
    y: Math.min(GRID - h, Math.max(0, snapToGrid(y))),
  };
}

export function placementOf(t: Table): Placement {
  return { id: t.id, x: t.x, y: t.y, w: t.w, h: t.h, rotation: t.rotation };
}

type Corner = { x: number; y: number };

// corners returns the four corners of a table after its rotation. A circle is
// replaced by the square around it, which is enough for a warning.
function corners(t: Placement): Corner[] {
  const cx = t.x + t.w / 2;
  const cy = t.y + t.h / 2;
  const rad = (t.rotation * Math.PI) / 180;
  const cos = Math.cos(rad);
  const sin = Math.sin(rad);
  return [
    [-t.w / 2, -t.h / 2],
    [t.w / 2, -t.h / 2],
    [t.w / 2, t.h / 2],
    [-t.w / 2, t.h / 2],
  ].map(([dx, dy]) => ({ x: cx + (dx as number) * cos - (dy as number) * sin, y: cy + (dx as number) * sin + (dy as number) * cos }));
}

function project(points: Corner[], axis: Corner): { min: number; max: number } {
  let min = Infinity;
  let max = -Infinity;
  for (const p of points) {
    const v = p.x * axis.x + p.y * axis.y;
    min = Math.min(min, v);
    max = Math.max(max, v);
  }
  return { min, max };
}

// overlaps reports whether two tables share any area, by the separating axis
// theorem. Touching edges do not count.
export function overlaps(a: Placement, b: Placement): boolean {
  const ca = corners(a);
  const cb = corners(b);
  for (const [p, q] of [
    [ca, cb],
    [cb, ca],
  ] as const) {
    for (let i = 0; i < p.length; i++) {
      const cur = p[i] as Corner;
      const next = p[(i + 1) % p.length] as Corner;
      const axis = { x: -(next.y - cur.y), y: next.x - cur.x };
      const len = Math.hypot(axis.x, axis.y);
      if (len === 0) continue;
      const unit = { x: axis.x / len, y: axis.y / len };
      const pa = project(p, unit);
      const pb = project(q, unit);
      if (pa.max <= pb.min + 1e-9 || pb.max <= pa.min + 1e-9) return false;
    }
  }
  return true;
}

// overlappingIds lists the tables the editor highlights as a warning.
export function overlappingIds(tables: Placement[]): Set<string> {
  const out = new Set<string>();
  for (let i = 0; i < tables.length; i++) {
    for (let j = i + 1; j < tables.length; j++) {
      const a = tables[i] as Placement;
      const b = tables[j] as Placement;
      if (overlaps(a, b)) {
        out.add(a.id);
        out.add(b.id);
      }
    }
  }
  return out;
}

// changedPlacements lists the tables whose geometry differs from the server.
export function changedPlacements(current: Placement[], saved: Table[]): Placement[] {
  const bySaved = new Map(saved.map((t) => [t.id, t]));
  return current.filter((p) => {
    const s = bySaved.get(p.id);
    return !s || s.x !== p.x || s.y !== p.y || s.w !== p.w || s.h !== p.h || s.rotation !== p.rotation;
  });
}
