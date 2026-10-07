import test from "node:test";
import assert from "node:assert/strict";
import {
  validatePlacement,
  blocksMovement,
} from "../src/world/freetower/building/validation.ts";
import {
  rectanglesOverlap,
  circleOverlap,
  normalizeRotation,
} from "../src/world/freetower/building/buildingMath.ts";
import { BuildingController } from "../src/world/freetower/building/controller.ts";
import { buildingDefinitions } from "../src/world/freetower/building/types.ts";
const actor = { x: 0, z: 18 };
const context = () => ({
  actor,
  players: [{ id: "a", position: actor }],
  inventory: { wood: 100, stone: 100 },
  buildings: {},
  mode: "land" as const,
});
const valid = { x: 5, z: 20 };
const placed = (id = "b", position = valid) => ({
  id,
  definitionId: "campfire",
  position: { ...position, y: 2 },
  rotationY: 0,
  status: "completed" as const,
  ownerId: "a",
  constructionStartedAt: 0,
  constructionEndsAt: 5000,
});
test("building definitions keep costs, timers, footprint and asset IDs in config", () => {
  assert.deepEqual(
    buildingDefinitions.campfire.cost.map((x) => x.quantity),
    [4, 3],
  );
  assert.equal(buildingDefinitions["small-shelter"].constructionSeconds, 20);
  assert.equal(
    buildingDefinitions.workbench.assetId,
    "building.workbench.basic",
  );
});
test("valid free placement uses terrain elevation and does not snap position", () => {
  const r = validatePlacement(
    "campfire",
    { x: 5.123, z: 20.321 },
    0,
    context(),
  );
  assert.equal(r.valid, true);
  assert.ok(r.y > 1);
  assert.ok(r.slope < 0.35);
});
test("reject bounds, water, protected ruins, tower and slope", () => {
  assert.equal(
    validatePlacement("campfire", { x: 73, z: 20 }, 0, context()).code,
    "bounds",
  );
  assert.equal(
    validatePlacement("campfire", { x: 0, z: 43 }, 0, context()).code,
    "water",
  );
  assert.equal(
    validatePlacement("campfire", { x: 0, z: -58 }, 0, context()).code,
    "protected",
  );
  assert.equal(
    validatePlacement("campfire", { x: 16, z: 340 }, 0, context()).code,
    "protected",
  );
  assert.equal(
    validatePlacement("campfire", { x: 22, z: -22 }, 0, {
      ...context(),
      actor: { x: 20, z: -20 },
      players: [],
    }).code,
    "slope",
  );
});
test("reject resources even depleted, players, costs and range", () => {
  assert.equal(
    validatePlacement("campfire", { x: -16, z: 17 }, 0, context()).code,
    "resource",
  );
  assert.equal(
    validatePlacement("campfire", actor, 0, context()).code,
    "player",
  );
  assert.equal(
    validatePlacement("campfire", valid, 0, {
      ...context(),
      inventory: { wood: 0, stone: 0 },
    }).code,
    "resources",
  );
  assert.equal(
    validatePlacement("campfire", { x: 25, z: 20 }, 0, context()).code,
    "distance",
  );
  assert.equal(
    validatePlacement("campfire", valid, 0, { ...context(), mode: "swimming" })
      .code,
    "swimming",
  );
});
test("oriented footprint overlap and move ignores only source building", () => {
  const buildings = { b: placed() };
  assert.equal(
    validatePlacement("campfire", valid, Math.PI / 6, {
      ...context(),
      buildings,
    }).code,
    "building",
  );
  assert.equal(
    validatePlacement("campfire", valid, Math.PI / 6, {
      ...context(),
      buildings,
      ignoreId: "b",
    }).valid,
    true,
  );
  assert.equal(
    rectanglesOverlap(
      { x: 0, z: 0 },
      0,
      { width: 5, depth: 1 },
      { x: 2, z: 2 },
      Math.PI / 2,
      { width: 5, depth: 1 },
    ),
    true,
  );
  assert.equal(
    circleOverlap(
      { x: 0, z: 0 },
      Math.PI / 2,
      { width: 4, depth: 1 },
      { x: 0, z: 2.2 },
      0.3,
    ),
    true,
  );
  assert.equal(normalizeRotation(Math.PI * 2), 0);
});
test("completed building blocks core footprint and construction is not solid", () => {
  assert.equal(blocksMovement(valid, [placed()]), true);
  assert.equal(blocksMovement({ x: 12, z: 18 }, [placed()]), false);
  assert.equal(
    blocksMovement(valid, [{ ...placed(), status: "constructing" }]),
    false,
  );
});
function setup() {
  let now = 1000;
  let id = 0;
  const sent: any[] = [];
  const c = new BuildingController(
    (m) => sent.push(m),
    () => now,
    () => `request-${++id}`,
  );
  c.ready = true;
  c.actor = actor;
  c.players = [{ id: "a", position: actor }];
  c.selfId = "a";
  c.applyState({
    version: 1,
    nodes: {},
    inventory: { wood: 100, stone: 100 },
    buildings: {},
  });
  return { c, sent, time: (n: number) => (now = n) };
}
test("place sends idempotent intent once; no local inventory/building mutation", () => {
  const { c, sent, time } = setup();
  c.choose("campfire");
  c.setGhost(valid);
  c.rotate();
  assert.ok(c.rotationY > 0);
  c.confirm();
  c.confirm();
  assert.equal(sent.length, 1);
  assert.equal(sent[0].type, "place-building");
  assert.equal(c.state?.inventory.wood, 100);
  assert.equal(Object.keys(c.state?.buildings ?? {}).length, 0);
  time(4001);
  c.tick();
  assert.equal(sent[1].requestId, sent[0].requestId);
});
test("cancel unsubmitted placement has no server side effects; swimming cancels", () => {
  const { c, sent } = setup();
  c.choose("campfire");
  c.cancel();
  assert.equal(c.mode, "gameplay");
  assert.equal(sent.length, 0);
  c.choose("campfire");
  c.movementMode = "swimming";
  c.tick();
  assert.equal(c.mode, "gameplay");
  c.openMenu();
  assert.equal(c.mode, "gameplay");
});
test("move starts only after lock grant, keeps old position, cancel returns token", () => {
  const { c, sent } = setup();
  c.applyState({
    version: 2,
    nodes: {},
    inventory: { wood: 0, stone: 0 },
    buildings: { b: placed() },
  });
  c.selectedId = "b";
  c.beginMove();
  assert.equal(sent[0].type, "begin-building-move");
  c.handle({
    type: "building-result",
    requestId: sent[0].requestId,
    buildingId: "b",
    moveToken: "lock",
  });
  assert.equal(c.mode, "moving");
  assert.deepEqual(c.ghost, valid);
  c.cancel();
  assert.equal(sent[1].type, "cancel-building-move");
  assert.equal(sent[1].moveToken, "lock");
  assert.deepEqual(c.state?.buildings?.b.position, placed().position);
});
test("move conflict leaves world untouched and produces feedback", () => {
  const { c, sent } = setup();
  c.applyState({
    version: 2,
    nodes: {},
    inventory: { wood: 0 },
    buildings: { b: placed() },
  });
  c.selectedId = "b";
  c.beginMove();
  c.handle({
    type: "building-error",
    requestId: sent[0].requestId,
    code: "move_locked",
    message: "Building currently being moved",
  });
  assert.equal(c.mode, "gameplay");
  assert.match(c.message, /currently/);
  assert.equal(c.pending, null);
});
test("renew/cancel acknowledgements cannot consume an in-flight placement", () => {
  const { c, sent } = setup();
  c.choose("campfire");
  c.setGhost(valid);
  c.confirm();
  c.handle({ type: "building-result", requestId: "", buildingId: "other" });
  assert.equal((c.pending as any)?.requestId, sent[0].requestId);
});
test("off-step rotations are rejected and occupants can exit newly completed site", () => {
  assert.equal(
    validatePlacement("campfire", valid, 0.123, context()).code,
    "rotation",
  );
  assert.equal(blocksMovement({ x: 5.1, z: 20 }, [placed()], valid), false);
});

function movingSetup() {
  const s = setup();
  s.c.applyState({ version: 2, nodes: {}, inventory: {}, buildings: { b: placed() } });
  s.c.selectedId = "b";
  s.c.beginMove();
  s.c.handle({ type: "building-result", requestId: s.sent[0].requestId, buildingId: "b", moveToken: "lock" });
  return s;
}
for (const code of ["rate_limit", "persistence"]) test(`move ${code} retains retry and cancel authority`, () => {
  const { c, sent } = movingSetup();
  c.setGhost({ x: 7, z: 20 });
  c.confirm();
  c.handle({ type: "building-error", requestId: sent[1].requestId, code, message: code });
  assert.equal(c.mode, "moving");
  assert.equal(c.moveToken, "lock");
  c.confirm();
  assert.equal(sent[2].moveToken, "lock");
  c.handle({ type: "building-error", requestId: sent[2].requestId, code, message: code });
  c.cancel();
  assert.equal(sent[3].type, "cancel-building-move");
});
test("cancel retries on error or dropped acknowledgement until authoritative completion", () => {
  const { c, sent, time } = movingSetup();
  c.cancel();
  c.handle({ type: "building-error", code: "persistence", message: "save failed" });
  assert.equal(c.moveToken, "lock");
  time(4001); c.tick();
  assert.equal(sent.at(-1).type, "cancel-building-move");
  c.handle({ type: "building-result", requestId: "", buildingId: "b" });
  assert.equal(c.moveToken, "lock");
  time(8002); c.tick();
  assert.equal(sent.at(-1).type, "cancel-building-move");
  c.applyState({ version: 3, nodes: {}, inventory: {}, buildings: { b: placed() } });
  assert.equal(c.moveToken, "");
});
test("anonymous renewal acknowledgement does not consume pending move", () => {
  const { c, sent } = movingSetup();
  c.setGhost({ x: 7, z: 20 }); c.confirm();
  c.handle({ type: "building-result", requestId: "", buildingId: "b" });
  assert.equal((c.pending as any)?.requestId, sent[1].requestId);
  assert.equal(c.moveToken, "lock");
});
test("movement escape reduces depth at every swept step including rotated footprints", () => {
  assert.equal(blocksMovement({ x: 7, z: 20 }, [placed()], { x: 6, z: 20 }), false);
  assert.equal(blocksMovement(valid, [placed()], { x: 6, z: 20 }), true);
  assert.equal(blocksMovement({ x: 8, z: 20 }, [placed()], { x: 2, z: 20 }), true);
  const rotated = { ...placed(), definitionId: "small-shelter", rotationY: Math.PI / 2 };
  assert.equal(blocksMovement({ x: 5, z: 24 }, [rotated], { x: 5, z: 21 }), false);
  assert.equal(blocksMovement(valid, [rotated], { x: 5, z: 21 }), true);
});

test("cancel during pending move survives error and disconnect until release", () => {
  const { c, sent, time } = movingSetup();
  c.setGhost({ x: 7, z: 20 }); c.confirm();
  c.cancel();
  assert.equal(c.moveToken, "lock");
  assert.equal(sent.length, 2);
  c.handle({ type: "building-error", requestId: sent[1].requestId, code: "rate_limit", message: "wait" });
  assert.equal(sent.at(-1).type, "cancel-building-move");
  c.connection(false);
  assert.equal(c.moveToken, "lock");
  c.connection(true); time(5001); c.tick();
  assert.equal(sent.at(-1).type, "cancel-building-move");
  c.beginMove();
  assert.equal(sent.at(-1).type, "cancel-building-move");
});
test("missing ground hit invalidates confirmation instead of reusing ghost", () => {
  const { c, sent } = setup();
  c.choose("campfire"); c.setGhost(valid);
  assert.equal(c.validation().valid, true);
  c.setGhost(null); c.confirm();
  assert.equal(c.validation().code, "pointer");
  assert.equal(sent.length, 0);
});
