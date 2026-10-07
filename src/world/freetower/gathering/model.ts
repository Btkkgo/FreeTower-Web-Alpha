import type { BuildingInstance } from "../building/types.ts";
import raw from "../../../../server/freetower_resources.json" with { type: "json" };
import type { Point, MovementMode } from "../model.ts";
export type ResourceType = "tree" | "rock";
export type ToolType =
  | "axe"
  | "pickaxe"
  | "shovel"
  | "hoe"
  | "watering-can"
  | "fishing-rod"
  | "hammer"
  | "weapon";
export type Hand = "left" | "right";
export type WheelCategory = "tools" | "items";
export type AssetId = string;
export interface ToolDefinition {
  id: string;
  type: ToolType;
  displayName: string;
  assetId: AssetId;
  gatherPower: number;
  cooldown: number;
  validResourceTypes: ResourceType[];
  handUsage: "one" | "two";
}
export interface ItemDefinition {
  id: string;
  displayName: string;
  assetId: AssetId;
  category: string;
}
export interface ResourceDrop {
  itemId: string;
  quantity: number;
}
export interface ResourceNodeDefinition {
  id: string;
  type: ResourceType;
  assetId: AssetId;
  maxDurability: number;
  requiredTool: ToolType;
  respawnSeconds: number;
  drops: ResourceDrop[];
}
export interface ResourcePlacement {
  id: string;
  definitionId: string;
  position: Point;
  scale: number;
  treeIndex?: number;
}
export interface ResourceNodeState {
  id: string;
  definitionId: string;
  position: Point;
  durability: number;
  maxDurability: number;
  depleted: boolean;
  respawnAt: number;
}
export interface ResourceState {
  life?: import("../life/types.ts").LifeState;
  buildings?: Record<string, BuildingInstance>;
  version: number;
  nodes: Record<string, ResourceNodeState>;
  inventory: Record<string, number>;
}
export interface HandState {
  left: string;
  right: string;
  active: Hand;
}
export interface GatherAction {
  id: string;
  resourceId: string;
  toolId: string;
  startedAt: number;
  hitAt: number;
  endsAt: number;
}
export interface ResourceHit {
  resourceId: string;
  userId: string;
  actionId: string;
  at: number;
  depleted: boolean;
}
export type GatheringMessage =
  | { type: "hands-set"; hands: HandState }
  | {
      type: "gather-start" | "resource-hit";
      resourceId: string;
      toolId: string;
      actionId: string;
    }
  | { type: "gather-cancel" };
export const config = raw;
export const tools = Object.fromEntries(
  raw.tools.map((t) => [t.id, t as ToolDefinition]),
);
export const items = Object.fromEntries(
  raw.items.map((t) => [t.id, t as ItemDefinition]),
);
export const definitions = Object.fromEntries(
  raw.definitions.map((d) => [d.id, d as ResourceNodeDefinition]),
);
export const nodes: ResourcePlacement[] = raw.nodes;
export const defaultHands = (): HandState => ({
  left: "basic-pickaxe",
  right: "basic-axe",
  active: "right",
});
export const activeItem = (h: HandState) => h[h.active];
export const toolFor = (h: HandState) => tools[activeItem(h)];
export const itemName = (id: string) =>
  tools[id]?.displayName ?? items[id]?.displayName ?? "Empty";
export const itemAsset = (id: string) =>
  tools[id]?.assetId ?? items[id]?.assetId;
export function wheelSlots(category: WheelCategory): (string | null)[] {
  const slots = category === "tools" ? ["basic-axe", "basic-pickaxe", "empty", "basic-fishing-rod", "basic-hoe", "basic-watering-can"] : ["empty", "wood", "stone", "basic-bait", "wheat-seed", "carrot-seed", "chicken-feed", "water-flask"];
  return Array.from({length:8},(_,i)=>slots[i]??null);
}
export function selectHandItem(
  h: HandState,
  id: string,
  inventory: Record<string, number>,
): HandState | null {
  if (id !== "empty" && !tools[id] && !(items[id] && inventory[id] > 0))
    return null;
  return { ...h, [h.active]: id };
}
export const distanceTo = (p: Point, q: Point) =>
  Math.hypot(p.x - q.x, p.z - q.z);
export function gatherProblem(
  node: ResourceNodeState | undefined,
  p: Point,
  mode: MovementMode,
  hands: HandState,
): string | null {
  if (mode === "swimming") return "游泳时不能采集 · Swimming";
  if (!node || node.depleted) return "选择一棵标记树或岩石 · Select resource";
  if (distanceTo(p, node.position) > raw.interactionRange)
    return "请靠近目标 · Out of range";
  const def = definitions[node.definitionId],
    tool = toolFor(hands);
  if (!def || !tool || !tool.validResourceTypes.includes(def.type))
    return `Wrong tool · 需要 ${def?.type === "tree" ? "Basic Axe" : "Basic Pickaxe"}`;
  return null;
}
export const cancelApproach = (id: string | null, input: Point) =>
  Math.hypot(input.x, input.z) > 0.01 ? null : id;
export function wheelIndex(x: number, y: number): number | null {
  if (Math.hypot(x, y) < 38) return null;
  return Math.round((Math.atan2(x, -y) + Math.PI * 2) / (Math.PI / 4)) % 8;
}
/** Local intent and animation only. Durability and inventory are exclusively server snapshots. */
export class GatheringController {
  send: (m: GatheringMessage) => void;
  state: ResourceState | null = null;
  hands = defaultHands();
  targetId: string | null = null;
  hoverId: string | null = null;
  approachId: string | null = null;
  position: Point = { x: 0, z: 18 };
  mode: MovementMode = "land";
  ready = false;
  suspended = false;
  personalInventory: Record<string,number> = {};
  useOverride: (()=>boolean)|null = null;
  get inventory(){return {...(this.state?.inventory??{}),...this.personalInventory};}
  wheel: WheelCategory | null = null;
  highlight: number | null = null;
  action: (GatherAction & { confirmed: boolean; sent: boolean }) | null = null;
  cooldownUntil = 0;
  serverOffset = 0;
  feedback = "";
  feedbackUntil = 0;
  hits: Record<string, ResourceHit> = {};
  qStarted: number | null = null;
  qConsumed = false;
  private serial = 0;
  constructor(send: (m: GatheringMessage) => void) {
    this.send = send;
  }
  applyState(s: ResourceState) {
    if (this.state && s.version < this.state.version) return;
    this.state = s;
    if (
      this.targetId &&
      (!s.nodes[this.targetId] || s.nodes[this.targetId].depleted)
    ) {
      this.targetId = null;
      this.approachId = null;
    }
  }
  applyPlayer(
    p: { hands?: HandState; gatherAction?: GatherAction | null },
    offset: number,
  ) {
    this.serverOffset = offset;
    if (p.hands) this.hands = { ...p.hands };
    if (p.gatherAction && this.action?.id === p.gatherAction.id) {
      this.action = {
        ...p.gatherAction,
        confirmed: true,
        sent: this.action.sent,
      };
      this.cooldownUntil =
        p.gatherAction.startedAt +
        (tools[p.gatherAction.toolId]?.cooldown ?? 800);
    }
  }
  note(text: string, now = Date.now()) {
    this.feedback = text;
    this.feedbackUntil = now + 2300;
  }
  select(id: string) {
    if (this.suspended) return;
    if (!this.state?.nodes[id] || this.state.nodes[id].depleted) return;
    this.targetId = id;
    this.approachId =
      distanceTo(this.position, this.state.nodes[id].position) >
      config.interactionRange
        ? id
        : null;
  }
  openWheel(category: WheelCategory) {
    if (!this.ready || this.suspended) return;
    this.cancelAction();
    this.wheel = category;
    this.highlight = null;
    this.approachId = null;
  }
  choose(id: string) {
    if (this.suspended) return;
    const h = selectHandItem(this.hands, id, this.inventory);
    if (!h) {
      this.note("库存中暂无此物品");
      return;
    }
    this.cancelAction();
    this.hands = h;
    this.send({ type: "hands-set", hands: h });
    this.wheel = null;
    this.highlight = null;
    this.qConsumed = true;
  }
  switchHand() {
    if (!this.ready || this.suspended) return;
    this.cancelAction();
    this.hands = {
      ...this.hands,
      active: this.hands.active === "left" ? "right" : "left",
    };
    this.send({ type: "hands-set", hands: this.hands });
  }
  qDown(now: number) {
    if (this.qStarted !== null || !this.ready || this.suspended) return;
    this.qStarted = now;
    this.qConsumed = false;
  }
  qUp(now: number) {
    if (this.qStarted === null) return;
    const long = now - this.qStarted >= raw.holdMs;
    if (long && !this.qConsumed) {
      if (!this.wheel) this.openWheel("tools");
      const id = wheelSlots(this.wheel ?? "tools")[this.highlight ?? -1];
      if (id) this.choose(id);
      this.wheel = null;
    } else if (!long && !this.qConsumed) this.switchHand();
    this.qStarted = null;
    this.qConsumed = false;
  }
  cancelInput() {
    this.qStarted = null;
    this.qConsumed = false;
    this.wheel = null;
    this.highlight = null;
    this.approachId = null;
    this.cancelAction();
  }
  cancelAction() {
    if (this.action) this.send({ type: "gather-cancel" });
    this.action = null;
  }
  reset() {
    this.cancelInput();
    this.state = null;
    this.targetId = null;
    this.hands = defaultHands();
    this.hits = {};
    this.cooldownUntil = 0;
    this.feedback = "";
  }
  use(now = Date.now()) {
    if (!this.suspended && this.useOverride?.()) return;
    if (this.suspended) return;
    if (!this.ready || this.wheel) return;
    if (this.action || now + this.serverOffset < this.cooldownUntil) {
      this.note("工具冷却中 · Recovering", now);
      return;
    }
    if (!this.targetId) {
      const near = Object.values(this.state?.nodes ?? {})
        .filter(
          (n) =>
            !n.depleted &&
            distanceTo(this.position, n.position) <= raw.interactionRange,
        )
        .sort(
          (a, b) =>
            distanceTo(this.position, a.position) -
            distanceTo(this.position, b.position),
        )[0];
      if (near) this.targetId = near.id;
    }
    const node = this.state?.nodes[this.targetId ?? ""];
    const problem = gatherProblem(node, this.position, this.mode, this.hands);
    if (problem) {
      this.note(problem, now);
      return;
    }
    const tool = toolFor(this.hands)!;
    const at = now + this.serverOffset,
      id = `${now.toString(36)}-${++this.serial}`;
    this.action = {
      id,
      resourceId: node!.id,
      toolId: tool.id,
      startedAt: at,
      hitAt: at + raw.windupMs,
      endsAt: at + raw.windupMs + raw.recoveryMs,
      confirmed: false,
      sent: false,
    };
    this.cooldownUntil = at + tool.cooldown;
    this.approachId = null;
    this.send({
      type: "gather-start",
      resourceId: node!.id,
      toolId: tool.id,
      actionId: id,
    });
  }
  tick(now = Date.now()) {
    if (!this.ready) {
      if (this.action || this.wheel || this.qStarted !== null)
        this.cancelInput();
      return;
    }
    if (
      this.qStarted !== null &&
      !this.qConsumed &&
      now - this.qStarted >= raw.holdMs &&
      !this.wheel
    )
      this.openWheel("tools");
    if (this.mode === "swimming" && this.action) {
      this.cancelAction();
      this.note("游泳时已收起工具", now);
    }
    const a = this.action,
      at = now + this.serverOffset;
    if (a) {
      if (a.confirmed && !a.sent && at >= a.hitAt + 20 && at <= a.endsAt) {
        a.sent = true;
        this.send({
          type: "resource-hit",
          resourceId: a.resourceId,
          toolId: a.toolId,
          actionId: a.id,
        });
      }
      if (at > a.endsAt + 100) {
        this.action = null;
      }
    }
  }
  locked(now = Date.now()) {
    return Boolean(
      this.wheel ||
      (this.action && now + this.serverOffset < this.action.endsAt),
    );
  }
}
