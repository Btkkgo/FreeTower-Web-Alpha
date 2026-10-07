import config from "../../../server/freetower_world.json" with { type: "json" };
export const worldConfig = config;
export type Point = { x: number; z: number };
export type MovementMode = "land" | "shallow-water" | "swimming";
export interface WaterZone {
  id: string;
  kind: "river" | "lake" | "ocean";
  level: number;
  contains: (p: Point) => boolean;
}
const smooth = (a: number, b: number, n: number) => {
  const t = Math.max(0, Math.min(1, (n - a) / (b - a)));
  return t * t * (3 - 2 * t);
};
export const coastOffset = (x: number) =>
  Math.sin(x * 0.06) * 3.5 + (Math.cos(x * 0.031) - 1) * 2;
export const cameraInput = (right: number, forward: number, yaw: number) => ({
  x: Math.cos(yaw) * right - Math.sin(yaw) * forward,
  z: -Math.sin(yaw) * right - Math.cos(yaw) * forward,
});
export const riverCenter = (x: number) =>
  config.river.center +
  Math.sin(x * config.river.frequency) * config.river.amplitude;
export const waterZones: WaterZone[] = [
  {
    id: "silver-river",
    kind: "river",
    level: config.waterLevel,
    contains: (p) => Math.abs(p.z - riverCenter(p.x)) < config.river.halfWidth,
  },
  {
    id: "ancient-sea",
    kind: "ocean",
    level: config.waterLevel,
    contains: (p) => p.z >= config.ocean.shore + coastOffset(p.x),
  },
];
export function terrainHeight(p: Point): number {
  const hill = 12 * Math.exp(-((p.x - 34) ** 2 / 260 + (p.z + 22) ** 2 / 380));
  const ruins = 6 * Math.exp(-(p.x ** 2 / 500 + (p.z + 57) ** 2 / 130));
  let h = 2 + hill + ruins + 0.22 * Math.sin(p.x * 0.13) * Math.cos(p.z * 0.11);
  const d = Math.abs(p.z - riverCenter(p.x));
  if (d < config.river.halfWidth)
    h = Math.min(
      h,
      -config.river.depth + 4.4 * smooth(0, config.river.halfWidth, d),
    );
  else if (d < config.river.halfWidth + 5)
    h =
      2 +
      (h - 2) * smooth(config.river.halfWidth, config.river.halfWidth + 5, d);
  const coastalZ = p.z - coastOffset(p.x);
  if (coastalZ > config.ocean.slopeStart)
    h = Math.min(
      h,
      2 *
        (1 - smooth(config.ocean.slopeStart, config.ocean.slopeEnd, coastalZ)) -
        Math.max(0, coastalZ - config.ocean.slopeEnd) * config.ocean.slope,
    );
  return h;
}
export function waterAt(p: Point) {
  const zone = waterZones.find(
    (z) => z.contains(p) && terrainHeight(p) < z.level,
  );
  return {
    id: zone?.id ?? "none",
    kind: zone?.kind ?? "none",
    level: zone?.level ?? 0,
    depth: zone ? Math.max(0, zone.level - terrainHeight(p)) : 0,
  };
}
export function locomotionAt(p: Point) {
  const water = waterAt(p);
  const mode: MovementMode =
    water.depth > config.movement.swimDepth
      ? "swimming"
      : water.depth > 0
        ? "shallow-water"
        : "land";
  return {
    mode,
    water,
    elevation:
      mode === "swimming"
        ? water.level - config.movement.floatOffset
        : terrainHeight(p),
  };
}
export const movementSpeed = (mode: MovementMode, run: boolean) =>
  mode === "swimming"
    ? config.movement.swim
    : mode === "shallow-water"
      ? config.movement.wade
      : run
        ? config.movement.run
        : config.movement.walk;
export const cancelNavigation = (route: Point[], input: Point) =>
  Math.hypot(input.x, input.z) > 0.01 ? [] : route;
export function constrainPosition(p: Point, fallback: Point) {
  return Number.isFinite(p.x) && Number.isFinite(p.z)
    ? {
        x: Math.max(-config.bound, Math.min(config.bound, p.x)),
        z: Math.max(-config.bound, Math.min(config.bound, p.z)),
      }
    : { ...fallback };
}
// The same deterministic tree colliders are used for the terrain and movement. Paths stay open.
export const trees = Array.from({ length: 170 }, (_, i) => {
  const r = Math.sqrt((i + 0.5) / 170) * 27,
    a = i * 2.399963;
  return {
    x: -37 + Math.cos(a) * r,
    z: -23 + Math.sin(a) * r,
    scale: 0.85 + (Math.sin(i * 7.31) + 1) * 0.45,
  };
})
  .concat(
    Array.from({ length: 26 }, (_, i) => ({
      x: (i % 2 === 0 ? -1 : 1) * (16 + (i % 7) * 3.7),
      z: 17 + (i / 26) * 16,
      scale: 0.8 + (i % 4) * 0.16,
    })),
  )
  .filter(
    (p) =>
      Math.abs(p.z + 19 + Math.sin(p.x * 0.08) * 5) > 2.8 &&
      Math.abs(p.z - riverCenter(p.x)) > 7,
  );
export function advanceMovement(
  p: Point,
  input: Point,
  run: boolean,
  dt: number,
) {
  const n = Math.hypot(input.x, input.z);
  if (!n) return p;
  const v = movementSpeed(locomotionAt(p).mode, run) * Math.min(dt, 0.05);
  let q = constrainPosition(
    { x: p.x + (input.x / n) * v, z: p.z + (input.z / n) * v },
    p,
  );
  for (const t of trees) {
    const dx = q.x - t.x,
      dz = q.z - t.z,
      d = Math.hypot(dx, dz),
      r = 0.48 * t.scale + 0.3;
    if (d < r) {
      q = { x: t.x + (d ? dx / d : 1) * r, z: t.z + (d ? dz / d : 0) * r };
    }
  }
  return constrainPosition(q, p);
}
export function movePayload(
  p: Point,
  facing: number,
  moving: boolean,
  running: boolean,
  jump: boolean,
) {
  const { mode } = locomotionAt(p);
  return {
    type: "move" as const,
    position: p,
    facing,
    movementMode: mode,
    moving,
    running: running && mode === "land",
    jump: jump && mode !== "swimming",
  };
}
export function regionAt(p: Point) {
  if (p.z > 40) return "Ancient Coast · 远古海岸";
  if (Math.abs(p.z - riverCenter(p.x)) < 6) return "Silver River · 银溪";
  if (p.z < -45) return "Ancient Ruins · 先民遗迹";
  if (p.x < -15 && p.z < 6) return "Whispering Forest · 风语森林";
  if (p.x > 17 && p.z < 4) return "Stone Hills · 岩石高地";
  return "Dawn Grassland · 晨光原野";
}

/** Preserve one-shot inputs until the server's move rate limit safely permits sending. */
export class MovementSender {
  pendingJump = false;
  lastAt = -Infinity;
  lastSignature = "";
  next(payload: ReturnType<typeof movePayload>, now: number) {
    this.pendingJump ||= payload.jump;
    if (now - this.lastAt < 0.1) return null;
    const message = {
      ...payload,
      jump: this.pendingJump && payload.movementMode !== "swimming",
    };
    const signature = JSON.stringify(message);
    if (signature === this.lastSignature) return null;
    this.pendingJump = false;
    this.lastAt = now;
    this.lastSignature = signature;
    return message;
  }
}

export interface Point3 extends Point {
  y: number;
}
export interface CameraObstacle extends Point3 {
  radius: number;
}
/** Conservative canopy proxies keep the follow camera on the near side of trees. */
export function cameraStop(
  from: Point3,
  to: Point3,
  obstacles: CameraObstacle[],
): Point3 {
  const dx = to.x - from.x,
    dy = to.y - from.y,
    dz = to.z - from.z,
    length = Math.hypot(dx, dy, dz);
  if (!length) return { ...to };
  const ux = dx / length,
    uy = dy / length,
    uz = dz / length;
  let reach = length;
  for (const o of obstacles) {
    const x = o.x - from.x,
      y = o.y - from.y,
      z = o.z - from.z,
      along = x * ux + y * uy + z * uz;
    const sideSquared = x * x + y * y + z * z - along * along;
    if (
      along <= 0 ||
      along > length + o.radius ||
      sideSquared > o.radius * o.radius
    )
      continue;
    const near = along - Math.sqrt(o.radius * o.radius - sideSquared);
    reach = Math.min(reach, Math.max(1.5, near - 0.45));
  }
  if (reach === length) return { ...to };
  return {
    x: from.x + ux * reach,
    y: from.y + uy * reach,
    z: from.z + uz * reach,
  };
}
export const cameraObstacles: CameraObstacle[] = trees.flatMap((t) => [
  { x: t.x, z: t.z, y: terrainHeight(t) + 5 * t.scale, radius: 2.9 * t.scale },
  { x: t.x, z: t.z, y: terrainHeight(t) + 7 * t.scale, radius: 2.3 * t.scale },
]);
