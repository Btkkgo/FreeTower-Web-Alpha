import test from "node:test";
import assert from "node:assert/strict";
import {
  GatheringController,
  defaultHands,
  config,
  definitions,
  nodes,
  toolFor,
  wheelSlots,
  selectHandItem,
  gatherProblem,
  cancelApproach,
  wheelIndex,
} from "../src/world/freetower/gathering/model.ts";
const fullState = () => ({
  version: 1,
  inventory: { wood: 4, stone: 3 },
  nodes: Object.fromEntries(
    nodes.map((n) => [
      n.id,
      {
        ...n,
        durability: definitions[n.definitionId].maxDurability,
        maxDurability: definitions[n.definitionId].maxDurability,
        depleted: false,
        respawnAt: 0,
      },
    ]),
  ),
});
const rig = () => {
  const packets: any[] = [];
  const c = new GatheringController((p) => packets.push(p));
  c.ready = true;
  c.applyState(fullState());
  c.position = { ...nodes[0].position };
  return { c, packets };
};
test("tap Q switches once; repeated keydown does not reset hold or toggle", () => {
  const { c, packets } = rig();
  c.qDown(100);
  c.qDown(150);
  c.qUp(200);
  assert.equal(c.hands.active, "left");
  assert.equal(packets.length, 1);
});
test("long Q opens wheel without quick switch; release cancels when no selection", () => {
  const { c, packets } = rig();
  c.qDown(100);
  c.tick(451);
  assert.equal(c.wheel, "tools");
  c.qUp(500);
  assert.equal(c.wheel, null);
  assert.equal(c.hands.active, "right");
  assert.equal(packets.length, 0);
});
test("focus cancellation closes wheel without switching", () => {
  const { c, packets } = rig();
  c.qDown(100);
  c.cancelInput();
  c.qUp(150);
  assert.equal(c.wheel, null);
  assert.equal(packets.length, 0);
});
test("two eight-slot categories and tool selection retain inactive hand", () => {
  assert.equal(wheelSlots("tools").length, 8);
  assert.equal(wheelSlots("items").length, 8);
  const h = selectHandItem(
    defaultHands(),
    "basic-pickaxe",
    fullState().inventory,
  );
  assert.equal(h.right, "basic-pickaxe");
  assert.equal(h.left, "basic-pickaxe");
  assert.equal(toolFor(h)?.type, "pickaxe");
});
test("item wheel needs inventory, empty always usable and resources are not spent", () => {
  assert.equal(selectHandItem(defaultHands(), "wood", {}), null);
  const inv = { wood: 4 };
  assert.equal(selectHandItem(defaultHands(), "wood", inv)?.right, "wood");
  assert.equal(inv.wood, 4);
  assert.equal(selectHandItem(defaultHands(), "empty", {})?.right, "empty");
});
test("wrong tool, range and swimming feedback never mutate resources", () => {
  const s = fullState(),
    n = s.nodes["tree-1"];
  assert.equal(
    gatherProblem(n, { ...n.position }, "land", {
      left: "empty",
      right: "basic-pickaxe",
      active: "right",
    }),
    "Wrong tool · 需要 Basic Axe",
  );
  assert.match(
    gatherProblem(n, { x: 70, z: 70 }, "land", defaultHands())!,
    /靠近/,
  );
  assert.match(
    gatherProblem(n, n.position, "swimming", defaultHands())!,
    /游泳/,
  );
  assert.equal(n.durability, 3);
});
test("gather waits for authoritative action and hit frame; sends once", () => {
  const { c, packets } = rig();
  c.select("tree-1");
  c.use(1000);
  assert.equal(packets[0].type, "gather-start");
  c.tick(1400);
  assert.equal(packets.length, 1);
  const id = packets[0].actionId;
  c.applyPlayer(
    {
      hands: defaultHands(),
      gatherAction: {
        id,
        resourceId: "tree-1",
        toolId: "basic-axe",
        startedAt: 1100,
        hitAt: 1380,
        endsAt: 1750,
      },
    },
    0,
  );
  c.tick(1390);
  assert.equal(packets.length, 1);
  c.tick(1410);
  c.tick(1500);
  assert.equal(packets.filter((p) => p.type === "resource-hit").length, 1);
  assert.equal(c.state!.nodes["tree-1"].durability, 3);
});
test("swimming cancels pending gather before hit", () => {
  const { c, packets } = rig();
  c.select("tree-1");
  c.use(1000);
  c.mode = "swimming";
  c.tick(1200);
  assert.equal(c.action, null);
  assert.equal(packets.at(-1).type, "gather-cancel");
});
test("depletion clears target; stale snapshots cannot restore resource", () => {
  const { c } = rig();
  c.select("tree-1");
  const s = fullState();
  s.version = 2;
  s.nodes["tree-1"].depleted = true;
  s.nodes["tree-1"].durability = 0;
  c.applyState(s);
  assert.equal(c.targetId, null);
  c.applyState(fullState());
  assert.equal(c.state!.nodes["tree-1"].depleted, true);
});
test("manual input cancels gather approach and radial direction selects slots", () => {
  assert.equal(cancelApproach("tree-1", { x: 1, z: 0 }), null);
  assert.equal(cancelApproach("tree-1", { x: 0, z: 0 }), "tree-1");
  assert.equal(wheelIndex(0, -100), 0);
  assert.equal(wheelIndex(100, 0), 2);
  assert.equal(wheelIndex(0, 0), null);
});
test("hand payload carries both sides and active hand, wheel selection closes", () => {
  const { c, packets } = rig();
  c.openWheel("items");
  c.choose("wood");
  assert.deepEqual(packets.at(-1), {
    type: "hands-set",
    hands: { left: "basic-pickaxe", right: "wood", active: "right" },
  });
  assert.equal(c.wheel, null);
});
test("server acknowledgement reconciles cooldown to authoritative start time", () => {
  const { c, packets } = rig();
  c.select("tree-1");
  c.use(1000);
  c.applyPlayer(
    {
      gatherAction: {
        id: packets[0].actionId,
        resourceId: "tree-1",
        toolId: "basic-axe",
        startedAt: 1100,
        hitAt: 1380,
        endsAt: 1750,
      },
    },
    0,
  );
  assert.equal(c.cooldownUntil, 1900);
});
test("far target starts approach, wheel cancels it without moving", () => {
  const { c } = rig();
  c.position = { x: 0, z: 18 };
  c.select("tree-1");
  assert.equal(c.approachId, "tree-1");
  c.openWheel("tools");
  assert.equal(c.approachId, null);
});
