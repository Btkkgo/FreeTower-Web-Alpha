import test from "node:test";
import assert from "node:assert/strict";
import {
  worldConfig,
  terrainHeight,
  waterAt,
  locomotionAt,
  movementSpeed,
  advanceMovement,
  cancelNavigation,
  movePayload,
  constrainPosition,
} from "../src/world/freetower/model.ts";

test("spawn is dry open land inside a 150m world", () => {
  assert.equal(worldConfig.size, 150);
  assert.equal(locomotionAt(worldConfig.spawn).mode, "land");
  assert.ok(terrainHeight(worldConfig.spawn) > 0);
});
test("river has shallow and deep water, not a visual-only boundary", () => {
  assert.equal(waterAt({ x: 0, z: -2 }).kind, "river");
  assert.equal(locomotionAt({ x: 0, z: -2 }).mode, "shallow-water");
  assert.equal(locomotionAt({ x: 0, z: -4 }).mode, "swimming");
});
test("ocean supports complete land-shallow-swim-shallow-land transitions", () => {
  const positions = [38, 44, 50, 44, 38].map((z) => ({ x: 0, z }));
  assert.deepEqual(
    positions.map((p) => locomotionAt(p).mode),
    ["land", "shallow-water", "swimming", "shallow-water", "land"],
  );
  assert.equal(waterAt(positions[2]).kind, "ocean");
});
test("swimmer floats at water surface rather than walking on the bed", () => {
  const p = { x: 0, z: 60 },
    state = locomotionAt(p);
  assert.ok(state.elevation > terrainHeight(p) + 2);
  assert.equal(
    state.elevation,
    worldConfig.waterLevel - worldConfig.movement.floatOffset,
  );
});
test("run is faster than walk; swim ignores sprint and wading slows down", () => {
  assert.ok(movementSpeed("land", true) > movementSpeed("land", false));
  assert.ok(
    movementSpeed("shallow-water", false) < movementSpeed("land", false),
  );
  assert.equal(
    movementSpeed("swimming", true),
    movementSpeed("swimming", false),
  );
  assert.ok(movementSpeed("swimming", true) < movementSpeed("land", true));
});
test("manual input cancels the previous click route", () => {
  assert.deepEqual(cancelNavigation([{ x: 3, z: 8 }], { x: 1, z: 0 }), []);
  assert.deepEqual(cancelNavigation([{ x: 3, z: 8 }], { x: 0, z: 0 }), [
    { x: 3, z: 8 },
  ]);
});
test("movement crosses shore without teleporting or getting stuck", () => {
  let p = { x: 0, z: 38 };
  const modes = new Set<string>();
  for (let i = 0; i < 150; i++) {
    const next = advanceMovement(p, { x: 0, z: 1 }, false, 0.05);
    assert.ok(Math.hypot(next.x - p.x, next.z - p.z) < 0.5);
    p = next;
    modes.add(locomotionAt(p).mode);
  }
  assert.ok(modes.has("swimming"));
  for (let i = 0; i < 160; i++)
    p = advanceMovement(p, { x: 0, z: -1 }, false, 0.05);
  assert.equal(locomotionAt(p).mode, "land");
});
test("movement payload preserves x/z serialization and adds remote animation state", () => {
  const msg = JSON.parse(
    JSON.stringify(movePayload({ x: 0, z: 50 }, 1, true, true, false)),
  );
  assert.deepEqual(msg.position, { x: 0, z: 50 });
  assert.equal(msg.movementMode, "swimming");
  assert.equal(msg.moving, true);
  assert.equal(msg.running, false);
  assert.equal(msg.jump, false);
});
test("bounds and nonfinite coordinates cannot escape the playable world", () => {
  const p = constrainPosition({ x: 999, z: -999 }, { x: 0, z: 18 });
  assert.ok(p.x <= 73 && p.z >= -73);
  assert.deepEqual(
    constrainPosition({ x: NaN, z: Infinity }, { x: 0, z: 18 }),
    { x: 0, z: 18 },
  );
});

test("camera-relative strafe uses the visible screen right direction", async () => {
  const { cameraInput } = await import("../src/world/freetower/model.ts");
  assert.ok(cameraInput(1, 0, Math.PI).x < 0);
  assert.ok(cameraInput(0, 1, Math.PI).z > 0);
});

test("jump survives a move sent less than 40ms earlier and is sent once", async () => {
  const { MovementSender } = await import("../src/world/freetower/model.ts");
  const sender = new MovementSender();
  const idle = movePayload({ x: 0, z: 18 }, 0, false, false, false);
  assert.ok(sender.next(idle, 1));
  assert.equal(sender.next({ ...idle, jump: true }, 1.02), null);
  assert.equal(sender.next(idle, 1.11)?.jump, true);
  assert.equal(sender.next(idle, 1.22)?.jump, false);
  assert.equal(sender.next(idle, 1.4), null);
});

test("camera stops before a canopy instead of clipping through it", async () => {
  const { cameraStop } = await import("../src/world/freetower/model.ts");
  const from = { x: 0, y: 1, z: 0 },
    to = { x: 0, y: 5, z: -10 };
  const stopped = cameraStop(from, to, [{ x: 0, y: 3, z: -5, radius: 2 }]);
  assert.ok(Math.hypot(stopped.x, stopped.y - 1, stopped.z) < 4);
  assert.deepEqual(cameraStop(from, to, []), to);
});
