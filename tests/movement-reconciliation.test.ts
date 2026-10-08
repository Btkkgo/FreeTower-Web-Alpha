import test from "node:test";
import assert from "node:assert/strict";
import { MovementReconciler } from "../src/world/freetower/movement-reconciliation.ts";
import { movePayload } from "../src/world/freetower/model.ts";

const move = (x: number) => movePayload({ x, z: 18 }, 0, true, true, false);

test("400ms accepted movement echoes do not rewind newer prediction at sprint speed", () => {
  const reconciler = new MovementReconciler();
  const queued: { at: number; sequence: number; x: number }[] = [];
  let predicted = { x: 0, z: 18 };
  for (let tick = 0; tick < 30; tick++) {
    predicted = { x: predicted.x + 0.74, z: 18 };
    const packet = reconciler.packet(move(predicted.x));
    queued.push({ at: tick + 4, sequence: packet.moveSequence, x: packet.position.x });
    const echo = queued.find((entry) => entry.at === tick);
    if (echo) {
      assert.ok(predicted.x - echo.x > 2, "fixture must trigger the old hard rewind");
      const before = predicted.x;
      predicted = reconciler.reconcile({ x: echo.x, z: 18 }, echo.sequence, predicted);
      assert.equal(predicted.x, before);
    }
  }
  assert.ok(Math.abs(predicted.x - 22.2) < 1e-10);
});

test("genuine collision correction replays newer displacement and repeated acknowledgements cannot reapply it", () => {
  const reconciler = new MovementReconciler();
  const first = reconciler.packet(move(5));
  const second = reconciler.packet(move(6));
  let predicted = reconciler.reconcile({ x: 3, z: 18 }, first.moveSequence, { x: 7, z: 18 });
  assert.deepEqual(predicted, { x: 5, z: 18 });
  predicted = reconciler.reconcile({ x: 3, z: 18 }, first.moveSequence, predicted);
  assert.deepEqual(predicted, { x: 5, z: 18 });
  predicted = reconciler.reconcile({ x: 3, z: 18 }, second.moveSequence, predicted);
  assert.deepEqual(predicted, { x: 4, z: 18 }, "pending prediction is rebased after the first correction");
  assert.deepEqual(reconciler.reconcile({ x: 3, z: 18 }, first.moveSequence, predicted), predicted);
});

test("initial presence cannot rewind unacknowledged moves and sent history is a snapshot", () => {
  const reconciler = new MovementReconciler();
  const payload = move(1);
  const packet = reconciler.packet(payload);
  payload.position.x = 50;
  assert.equal(packet.position.x, 1);
  assert.deepEqual(reconciler.reconcile({ x: 0, z: 18 }, 0, { x: 5, z: 18 }), { x: 5, z: 18 });
  assert.deepEqual(reconciler.reconcile({ x: 1, z: 18 }, packet.moveSequence, { x: 5, z: 18 }), { x: 5, z: 18 });
});

test("ignored packets can be skipped by a newer acknowledgement; history remains bounded", () => {
  const reconciler = new MovementReconciler();
  for (let i = 1; i <= 300; i++) reconciler.packet(move(i));
  assert.deepEqual(reconciler.reconcile({ x: 1, z: 18 }, 1, { x: 301, z: 18 }), { x: 301, z: 18 });
  assert.deepEqual(reconciler.reconcile({ x: 299, z: 18 }, 299, { x: 301, z: 18 }), { x: 301, z: 18 });
  assert.deepEqual(reconciler.reconcile({ x: 300, z: 18 }, 300, { x: 301, z: 18 }), { x: 301, z: 18 });
});

test("reconnect starts a new acknowledgement epoch and older servers keep correction behavior", () => {
  const previous = new MovementReconciler();
  previous.packet(move(1));
  previous.packet(move(2));
  const reconnected = new MovementReconciler();
  assert.equal(reconnected.packet(move(0)).moveSequence, 1);
  assert.deepEqual(reconnected.reconcile({ x: 0, z: 18 }, undefined, { x: 3, z: 18 }), { x: 0, z: 18 });
});
