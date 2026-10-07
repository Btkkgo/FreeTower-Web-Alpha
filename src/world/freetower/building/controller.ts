import type { MovementMode, Point } from "../model.ts";
import type { ResourceState } from "../gathering/model.ts";
import {
  buildingDefinitions,
  buildingConfig,
  type BuildingMessage,
  type BuildingReply,
} from "./types.ts";
import { normalizeRotation } from "./buildingMath.ts";
import { validatePlacement } from "./validation.ts";
export class BuildingController {
  send: (m: BuildingMessage) => void;
  now: () => number;
  id: () => string;
  ready = false;
  mode: "gameplay" | "menu" | "placing" | "moving" = "gameplay";
  definitionId = "";
  selectedId: string | null = null;
  moveId: string | null = null;
  moveToken = "";
  ghost: Point | null = null;
  rotationY = 0;
  actor: Point = { x: 0, z: 18 };
  movementMode: MovementMode = "land";
  players: { id: string; position: Point }[] = [];
  selfId = "";
  state: ResourceState | null = null;
  serverOffset = 0;
  message = "";
  messageUntil = 0;
  pending: BuildingMessage | null = null;
  pendingAt = 0;
  lastRenew = 0;
  cancelledBegin = false;
  cancelling = false;
  cancelAt = 0;
  previewSession = 0;
  constructor(
    send: (m: BuildingMessage) => void,
    now = () => Date.now(),
    id = () => crypto.randomUUID(),
  ) {
    this.send = send;
    this.now = now;
    this.id = id;
  }
  get active() {
    return this.mode !== "gameplay";
  }
  get editing() {
    return this.mode === "placing" || this.mode === "moving";
  }
  note(message: string) {
    this.message = message;
    this.messageUntil = this.now() + 4500;
  }
  applyState(state: ResourceState) {
    if (this.state && state.version < this.state.version) return;
    this.state = state;
    if (this.selectedId && !state.buildings?.[this.selectedId])
      this.selectedId = null;
    if (this.moveId && this.moveToken) {
      const b = state.buildings?.[this.moveId];
      if (!b || b.status !== "moving" || b.moveOwnerId !== this.selfId) {
        this.resetPreview();
        this.note("Move ended · 移动会话已结束");
      }
    }
  }
  validation() {
    return this.ghost
      ? validatePlacement(this.definitionId, this.ghost, this.rotationY, {
          actor: this.actor,
          players: this.players,
          inventory: this.state?.inventory ?? {},
          buildings: this.state?.buildings ?? {},
          mode: this.movementMode,
          ignoreId: this.moveId ?? undefined,
        })
      : {
          valid: false,
          code: "pointer",
          reason: "Point at ground · 指向地面",
          y: 0,
          slope: 0,
          overlapIds: [],
        };
  }
  openMenu() {
    if (!this.ready || this.pending || this.moveToken) return;
    if (this.movementMode !== "land") {
      this.note("Return to dry land · 请先上岸");
      return;
    }
    this.cancel();
    this.mode = "menu";
  }
  choose(id: string) {
    if (
      !this.ready ||
      this.pending ||
      this.moveToken ||
      this.movementMode !== "land" ||
      !buildingDefinitions[id]
    )
      return;
    const d = buildingDefinitions[id];
    if (
      d.cost.some((x) => (this.state?.inventory[x.itemId] ?? 0) < x.quantity)
    ) {
      this.note("Not enough resources · 资源不足");
      return;
    }
    this.definitionId = id;
    this.mode = "placing";
    this.previewSession++;
    this.ghost = { x: this.actor.x + 4, z: this.actor.z };
    this.rotationY = 0;
    this.selectedId = null;
  }
  setGhost(p: Point | null) {
    if (this.editing && !this.pending) this.ghost = p ? { ...p } : null;
  }
  rotate() {
    if (this.editing && !this.pending)
      this.rotationY = normalizeRotation(
        this.rotationY +
          (buildingDefinitions[this.definitionId].rotationStep * Math.PI) / 180,
      );
  }
  submit(m: BuildingMessage) {
    if (this.pending) return;
    this.pending = m;
    this.pendingAt = this.now();
    this.send(m);
  }
  confirm() {
    if (!this.ready || this.pending || !this.editing || !this.ghost) return;
    const v = this.validation();
    if (!v.valid) {
      this.note(v.reason);
      return;
    }
    const common = {
      requestId: this.id(),
      position: { ...this.ghost },
      rotationY: this.rotationY,
    };
    if (this.moveId)
      this.submit({
        type: "move-building",
        ...common,
        buildingId: this.moveId,
        moveToken: this.moveToken,
      });
    else
      this.submit({
        type: "place-building",
        ...common,
        definitionId: this.definitionId,
      });
  }
  select(id: string) {
    if (this.active || this.pending) return;
    this.selectedId = id;
  }
  beginMove() {
    const b = this.state?.buildings?.[this.selectedId ?? ""];
    if (!this.ready || this.pending || this.moveToken || !b) return;
    if (b.status !== "completed") {
      this.note(
        b.status === "moving"
          ? "Building currently being moved"
          : "Construction in progress",
      );
      return;
    }
    if (this.movementMode !== "land") {
      this.note("Return to dry land");
      return;
    }
    this.cancelledBegin = false;
    this.submit({
      type: "begin-building-move",
      requestId: this.id(),
      buildingId: b.id,
    });
  }
  handle(reply: BuildingReply) {
    if (!reply.requestId) {
      if (reply.type === "building-error") this.note(reply.message);
      return;
    }
    if (
      reply.requestId &&
      (!this.pending ||
        !("requestId" in this.pending) ||
        reply.requestId !== this.pending.requestId)
    )
      return;
    const pending = this.pending;
    this.pending = null;
    if (reply.type === "building-error") {
      this.note(reply.message);
      if (pending?.type === "move-building" && this.cancelling) this.cancel();
      return;
    }
    if (pending?.type === "begin-building-move" && reply.moveToken) {
      const b = this.state?.buildings?.[reply.buildingId];
      this.moveId = reply.buildingId;
      this.moveToken = reply.moveToken;
      if (this.cancelledBegin || !this.ready || !b) {
        this.cancel();
        return;
      }
      this.mode = "moving";
      this.previewSession++;
      this.moveId = b.id;
      this.moveToken = reply.moveToken;
      this.definitionId = b.definitionId;
      this.ghost = { x: b.position.x, z: b.position.z };
      this.rotationY = b.rotationY;
      this.lastRenew = this.now();
      this.note("Move building · 选择新位置");
    } else {
      this.resetPreview();
      this.selectedId = reply.buildingId;
      this.note("Saved · 已保存");
    }
  }
  resetPreview() {
    this.mode = "gameplay";
    this.definitionId = "";
    this.ghost = null;
    this.moveId = null;
    this.moveToken = "";
    this.cancelling = false;
  }
  cancel() {
    if (this.pending?.type === "begin-building-move") this.cancelledBegin = true;
    if (this.moveId && this.moveToken) {
      this.cancelling = true;
      this.mode = "gameplay";
      this.ghost = null;
      if (!this.pending && this.ready) {
        this.cancelAt = this.now();
        this.send({ type: "cancel-building-move", buildingId: this.moveId, moveToken: this.moveToken });
      }
    } else this.resetPreview();
  }
  connection(ready: boolean) {
    this.ready = ready;
    if (!ready) {
      if (this.pending?.type === "begin-building-move")
        this.cancelledBegin = true;
      this.cancel();
    } else if (this.pending) {
      this.pendingAt = this.now();
      this.send(this.pending);
    }
  }
  tick() {
    if (this.movementMode !== "land" && this.active) this.cancel();
    if (this.ready && this.pending && this.now() - this.pendingAt > 3000) {
      this.pendingAt = this.now();
      this.send(this.pending);
    }
    if (this.ready && this.cancelling && !this.pending && this.now() - this.cancelAt > 3000) this.cancel();
    if (
      this.ready &&
      !this.cancelling &&
      this.moveId &&
      this.moveToken &&
      this.now() - this.lastRenew > buildingConfig.moveLockMs / 3
    ) {
      this.lastRenew = this.now();
      this.send({
        type: "renew-building-move",
        buildingId: this.moveId,
        moveToken: this.moveToken,
      });
    }
  }
  reset() {
    this.cancel();
    this.pending = null;
    this.state = null;
    this.selectedId = null;
    this.message = "";
  }
}
