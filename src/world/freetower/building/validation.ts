import {
  terrainHeight,
  worldConfig,
  riverCenter,
  coastOffset,
  trees,
  type Point,
  type MovementMode,
} from "../model.ts";
import { nodes } from "../gathering/model.ts";
import {
  buildingConfig,
  buildingDefinitions,
  type BuildingInstance,
} from "./types.ts";
import {
  circleOverlap,
  buildingPenetration,
  footprintSamples,
  rectanglesOverlap,
} from "./buildingMath.ts";
export interface PlacementContext {
  actor: Point;
  players: { id: string; position: Point }[];
  inventory: Record<string, number>;
  buildings: Record<string, BuildingInstance>;
  mode: MovementMode;
  ignoreId?: string;
}
export interface PlacementValidation {
  valid: boolean;
  code: string;
  reason: string;
  y: number;
  slope: number;
  overlapIds: string[];
}
export function validatePlacement(
  definitionId: string,
  p: Point,
  r: number,
  ctx: PlacementContext,
): PlacementValidation {
  const result: PlacementValidation = {
    valid: false,
    code: "",
    reason: "",
    y: terrainHeight(p),
    slope: 0,
    overlapIds: [],
  };
  const fail = (
    code: string,
    reason: string,
    ids: string[] = [],
  ): PlacementValidation => ({ ...result, code, reason, overlapIds: ids });
  const d = buildingDefinitions[definitionId];
  if (!d) return fail("definition", "Unknown building");
  if (!Number.isFinite(p.x) || !Number.isFinite(p.z) || !Number.isFinite(r))
    return fail("position", "Invalid position");
  const step = (d.rotationStep * Math.PI) / 180;
  if (Math.abs(r / step - Math.round(r / step)) > 1e-6)
    return fail("rotation", "Invalid rotation step");
  if (ctx.mode !== "land")
    return fail("swimming", "Return to dry land · 请先上岸");
  for (const zone of buildingConfig.protectedZones)
    if (circleOverlap(p, r, d.footprint, zone, zone.radius))
      return fail("protected", `Protected zone · ${zone.id}`, [zone.id]);
  const samples = footprintSamples(p, r, d.footprint),
    rules = d.placementRules;
  if (
    samples.some(
      (q) =>
        Math.abs(q.x) > worldConfig.bound || Math.abs(q.z) > worldConfig.bound,
    )
  )
    return fail("bounds", "Outside world boundary");
  if (
    samples.some(
      (q) =>
        Math.abs(q.z - riverCenter(q.x)) <
          worldConfig.river.halfWidth + rules.riverBuffer ||
        q.z >= worldConfig.ocean.shore + coastOffset(q.x) - rules.coastBuffer ||
        terrainHeight(q) < rules.minElevation,
    )
  )
    return fail("water", "Water / coast · 水域不可建造");
  let min = Infinity,
    max = -Infinity;
  for (const q of samples) {
    const h = terrainHeight(q);
    min = Math.min(min, h);
    max = Math.max(max, h);
    const dx =
        (terrainHeight({ x: q.x + 0.25, z: q.z }) -
          terrainHeight({ x: q.x - 0.25, z: q.z })) /
        0.5,
      dz =
        (terrainHeight({ x: q.x, z: q.z + 0.25 }) -
          terrainHeight({ x: q.x, z: q.z - 0.25 })) /
        0.5;
    result.slope = Math.max(result.slope, Math.hypot(dx, dz));
  }
  if (result.slope > rules.maxSlope || max - min > rules.maxHeightDelta)
    return fail("slope", "Ground too steep · 坡度过大");
  const overlaps = Object.values(ctx.buildings)
    .filter(
      (b) =>
        b.id !== ctx.ignoreId &&
        buildingDefinitions[b.definitionId] &&
        rectanglesOverlap(
          p,
          r,
          d.footprint,
          b.position,
          b.rotationY,
          buildingDefinitions[b.definitionId].footprint,
        ),
    )
    .map((b) => b.id);
  if (overlaps.length)
    return fail("building", "Building overlap · 建筑重叠", overlaps);
  for (const n of nodes) {
    const radius =
      n.definitionId === "tree-basic"
        ? 0.48 * n.scale + 0.4
        : 1.2 * n.scale + 0.3;
    if (circleOverlap(p, r, d.footprint, n.position, radius))
      return fail("resource", "Resource reserved · 资源节点占用", [n.id]);
  }
  for (const [i, t] of trees.entries())
    if (circleOverlap(p, r, d.footprint, t, 0.48 * t.scale + 0.3))
      return fail("tree", "Tree overlap · 树木占用", [`tree-decoration-${i}`]);
  const occupied = ctx.players
    .filter((v) =>
      circleOverlap(p, r, d.footprint, v.position, buildingConfig.playerRadius),
    )
    .map((v) => v.id);
  if (occupied.length)
    return fail("player", "Player nearby · 玩家安全区域", occupied);
  if (
    Math.hypot(p.x - ctx.actor.x, p.z - ctx.actor.z) >
    buildingConfig.placementRange
  )
    return fail("distance", "Move closer · 请靠近至 14 m 内");
  if (
    !ctx.ignoreId &&
    d.cost.some((c) => (ctx.inventory[c.itemId] ?? 0) < c.quantity)
  )
    return fail("resources", "Not enough resources · 资源不足");
  return {
    ...result,
    valid: true,
    code: "ok",
    reason: "Ready to place · 可以放置",
  };
}
export function blocksMovement(p: Point, buildings: BuildingInstance[], old?: Point) {
  const start = old ?? p;
  const steps = Math.max(1, Math.ceil(Math.hypot(p.x - start.x, p.z - start.z) / 0.2));
  const at = (i: number) => ({ x: start.x + (p.x - start.x) * i / steps, z: start.z + (p.z - start.z) * i / steps });
  for (let i = 1; i <= steps; i++) {
    const q = at(i), previous = at(i - 1);
    for (const b of buildings) {
      const d = buildingDefinitions[b.definitionId];
      if (b.definitionId === "farm-plot" || !d || (b.status !== "completed" && b.status !== "moving")) continue;
      const overlaps = (point: Point) => circleOverlap(b.position, b.rotationY, d.footprint, point, buildingConfig.playerRadius);
      if (!overlaps(q)) continue;
      if (old && overlaps(previous) && buildingPenetration(q, b.position, b.rotationY, d.footprint, buildingConfig.playerRadius) < buildingPenetration(previous, b.position, b.rotationY, d.footprint, buildingConfig.playerRadius) - 1e-9) continue;
      return true;
    }
  }
  return false;
}
