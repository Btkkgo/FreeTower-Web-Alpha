import type { Point } from "../model.ts";
import type { Footprint } from "./types.ts";
export const normalizeRotation = (r: number) =>
  ((r % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);
export function localPoint(p: Point, c: Point, r: number): Point {
  const x = p.x - c.x,
    z = p.z - c.z;
  return {
    x: Math.cos(r) * x - Math.sin(r) * z,
    z: Math.sin(r) * x + Math.cos(r) * z,
  };
}
export function worldPoint(p: Point, c: Point, r: number): Point {
  return {
    x: c.x + Math.cos(r) * p.x + Math.sin(r) * p.z,
    z: c.z - Math.sin(r) * p.x + Math.cos(r) * p.z,
  };
}
export function footprintSamples(c: Point, r: number, f: Footprint) {
  const out: Point[] = [c];
  const nx = Math.ceil(f.width),
    nz = Math.ceil(f.depth);
  for (let x = 0; x <= nx; x++)
    for (let z = 0; z <= nz; z++)
      out.push(
        worldPoint(
          {
            x: -f.width / 2 + (x * f.width) / nx,
            z: -f.depth / 2 + (z * f.depth) / nz,
          },
          c,
          r,
        ),
      );
  return out;
}
export function circleOverlap(
  c: Point,
  r: number,
  f: Footprint,
  p: Point,
  radius: number,
) {
  const q = localPoint(p, c, r);
  const dx = Math.max(0, Math.abs(q.x) - f.width / 2),
    dz = Math.max(0, Math.abs(q.z) - f.depth / 2);
  return dx * dx + dz * dz <= radius * radius;
}
export function rectanglesOverlap(
  a: Point,
  ar: number,
  af: Footprint,
  b: Point,
  br: number,
  bf: Footprint,
) {
  const axes = (r: number) => [
    { x: Math.cos(r), z: -Math.sin(r) },
    { x: Math.sin(r), z: Math.cos(r) },
  ];
  const aa = axes(ar),
    ba = axes(br),
    dot = (a: Point, b: Point) => a.x * b.x + a.z * b.z;
  return [...aa, ...ba].every(
    (axis) =>
      Math.abs(dot({ x: b.x - a.x, z: b.z - a.z }, axis)) <
      (Math.abs(dot(aa[0], axis)) * af.width) / 2 +
        (Math.abs(dot(aa[1], axis)) * af.depth) / 2 +
        (Math.abs(dot(ba[0], axis)) * bf.width) / 2 +
        (Math.abs(dot(ba[1], axis)) * bf.depth) / 2 +
        0.15,
  );
}

export function buildingPenetration(p: Point, c: Point, r: number, f: Footprint, radius: number) {
  const v = localPoint(p, c, r);
  return Math.min(f.width / 2 + radius - Math.abs(v.x), f.depth / 2 + radius - Math.abs(v.z));
}
