import { useEffect, useReducer, useRef } from "react";
import {
  GatheringController,
  activeItem,
  config,
  definitions,
  distanceTo,
  itemName,
  toolFor,
  wheelIndex,
  wheelSlots,
  type GatheringMessage,
} from "./model";
import "./gathering.css";
const focusWorld = () =>
  document
    .querySelector<HTMLCanvasElement>(".ft-world canvas")
    ?.focus({ preventScroll: true });
export function useGathering(
  send: (m: GatheringMessage) => void,
  ready: boolean,
  roomId?: string,
) {
  const sendRef = useRef(send);
  sendRef.current = send;
  const ref = useRef<GatheringController | null>(null);
  if (!ref.current)
    ref.current = new GatheringController((m) => sendRef.current(m));
  const c = ref.current;
  useEffect(() => {
    c.ready = ready;
    if (!ready) c.cancelInput();
  }, [c, ready]);
  useEffect(() => {
    c.reset();
    return () => c.reset();
  }, [c, roomId]);
  return c;
}
export default function GatheringHUD({
  controller: c,
  debug,
}: {
  controller: GatheringController;
  debug: boolean;
}) {
  const [, refresh] = useReducer((n) => n + 1, 0);
  const center = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const t = setInterval(() => {
      c.tick();
      refresh();
    }, 50);
    return () => clearInterval(t);
  }, [c]);
  useEffect(() => {
    const typing = (target: EventTarget | null) =>
      target instanceof HTMLElement &&
      Boolean(target.closest("input,textarea,[contenteditable=true]"));
    const down = (e: KeyboardEvent) => {
      if (typing(e.target) || !c.ready) return;
      if (e.code === "KeyQ") {
        e.preventDefault();
        if (!e.repeat) c.qDown(Date.now());
      }
      if (e.code === "KeyE" && !e.repeat && !c.wheel) {
        e.preventDefault();
        c.use();
      }
      if (c.wheel) {
        if (e.code === "Escape") {
          c.cancelInput();
          focusWorld();
          e.preventDefault();
        }
        if (e.code === "Tab") {
          e.preventDefault();
          c.wheel = c.wheel === "tools" ? "items" : "tools";
          c.highlight = null;
        }
        const digit = /^Digit([1-8])$/.exec(e.code);
        if (digit && !e.repeat) {
          e.preventDefault();
          const id = wheelSlots(c.wheel ?? "tools")[Number(digit[1]) - 1];
          if (id) {
            c.choose(id);
            if (!c.wheel) focusWorld();
          }
        }
      }
      refresh();
    };
    const up = (e: KeyboardEvent) => {
      if (e.code === "KeyQ") {
        const held = c.qStarted !== null;
        c.qUp(Date.now());
        if (held && !c.wheel) focusWorld();
        refresh();
      }
    };
    const clear = () => {
      c.cancelInput();
      refresh();
    };
    const focus = (e: FocusEvent) => {
      if (typing(e.target)) clear();
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", clear);
    window.addEventListener("focusin", focus);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", clear);
      window.removeEventListener("focusin", focus);
    };
  }, [c]);
  const node = c.state?.nodes[c.targetId ?? ""];
  const tool = toolFor(c.hands);
  const now = Date.now();
  const cooldown = Math.max(0, c.cooldownUntil - now - c.serverOffset);
  return (
    <>
      <div className="ft-resources" aria-label="Shared resources">
        <span>
          ◈ Wood <b>{c.state?.inventory.wood ?? 0}</b>
        </span>
        <span>
          ◆ Stone <b>{c.state?.inventory.stone ?? 0}</b>
        </span>
        <small>WORLD SHARED</small>
      </div>
      <div className="ft-hands" aria-label="Hand equipment">
        <button
          aria-label={`Left hand: ${itemName(c.hands.left)}`}
          className={c.hands.active === "left" ? "active" : ""}
          onClick={() => {
            if (c.hands.active !== "left") c.switchHand();
            focusWorld();
            refresh();
          }}
        >
          <small>L</small>
          {itemName(c.hands.left)}
        </button>
        <button
          aria-label={`Right hand: ${itemName(c.hands.right)}`}
          className={c.hands.active === "right" ? "active" : ""}
          onClick={() => {
            if (c.hands.active !== "right") c.switchHand();
            focusWorld();
            refresh();
          }}
        >
          <small>R</small>
          {itemName(c.hands.right)}
        </button>
        <button
          aria-label="Open Tool Wheel"
          onClick={() => {
            c.openWheel("tools");
            refresh();
          }}
        >
          Tools
        </button>
        <button
          aria-label="Open Item Wheel"
          onClick={() => {
            c.openWheel("items");
            refresh();
          }}
        >
          Items
        </button>
        <small>Q 换手 · Hold Q 轮盘 · E 采集</small>
      </div>
      {node && !node.depleted && (
        <div className="ft-resource-target" aria-label="Selected resource">
          <span>
            {definitions[node.definitionId]?.type === "tree"
              ? "TREE · 林木"
              : "ROCK · 岩石"}
          </span>
          <b>
            {node.durability} / {node.maxDurability}
          </b>
          <small>
            {distanceTo(c.position, node.position).toFixed(1)} m ·{" "}
            {distanceTo(c.position, node.position) <= config.interactionRange
              ? "E / 再次点击采集"
              : "靠近后 E"}
          </small>
        </div>
      )}
      {c.feedbackUntil > now && (
        <div role="status" className="ft-gather-feedback">
          {c.feedback}
        </div>
      )}
      {debug && (
        <div className="ft-gather-debug">
          Tool {tool?.id ?? activeItem(c.hands)} · Active {c.hands.active}
          <br />
          Target {node?.id ?? "none"} ·{" "}
          {node ? definitions[node.definitionId]?.type : "none"} ·{" "}
          {node ? distanceTo(c.position, node.position).toFixed(2) : "—"} m
          <br />
          Durability {node?.durability ?? "—"} · Cooldown {Math.ceil(cooldown)}{" "}
          ms
        </div>
      )}
      {c.wheel && (
        <div
          className="ft-wheel-scrim"
          onPointerDown={(e) => e.stopPropagation()}
        >
          <div
            className="ft-wheel"
            ref={center}
            role="dialog"
            aria-modal="true"
            aria-label={c.wheel === "tools" ? "Tool Wheel" : "Item Wheel"}
            onPointerMove={(e) => {
              const r = center.current!.getBoundingClientRect();
              c.highlight = wheelIndex(
                e.clientX - r.left - r.width / 2,
                e.clientY - r.top - r.height / 2,
              );
              refresh();
            }}
          >
            <div className="ft-wheel-core">
              <strong>
                {c.wheel === "tools" ? "TOOL WHEEL" : "ITEM WHEEL"}
              </strong>
              <span>
                {c.hands.active === "left" ? "LEFT HAND" : "RIGHT HAND"}
              </span>
              <small>1–8 选择 · Tab 分类</small>
              <button
                aria-label="Close wheel"
                onClick={() => {
                  c.cancelInput();
                  focusWorld();
                  refresh();
                }}
              >
                ✕
              </button>
            </div>
            {wheelSlots(c.wheel).map((id, i) => {
              const a = (i * Math.PI) / 4;
              const available =
                id &&
                (id === "empty" ||
                  Boolean(config.tools.find((t) => t.id === id)) ||
                  (c.state?.inventory[id] ?? 0) > 0);
              return (
                <button
                  key={i}
                  className={`ft-wheel-slot ${c.highlight === i ? "selected" : ""}`}
                  style={{
                    left: `calc(50% + ${Math.sin(a) * 145}px)`,
                    top: `calc(50% - ${Math.cos(a) * 145}px)`,
                  }}
                  disabled={!available}
                  aria-label={`Slot ${i + 1}: ${id ? itemName(id) : "Unavailable"}`}
                  onPointerEnter={() => {
                    c.highlight = i;
                    refresh();
                  }}
                  onClick={() => {
                    if (id) c.choose(id);
                    if (!c.wheel) focusWorld();
                    refresh();
                  }}
                >
                  <small>{i + 1}</small>
                  <span>{id ? itemName(id) : "—"}</span>
                  {id && c.wheel === "items" && id !== "empty" && (
                    <b>{c.state?.inventory[id] ?? 0}</b>
                  )}
                </button>
              );
            })}
          </div>
          <div className="ft-wheel-tabs">
            <button
              onClick={() => {
                c.wheel = "tools";
                c.highlight = null;
                refresh();
              }}
            >
              Tools
            </button>
            <button
              onClick={() => {
                c.wheel = "items";
                c.highlight = null;
                refresh();
              }}
            >
              Items
            </button>
            <span>选择装备到当前手 · 松开 Q 确认方向 · Esc 关闭</span>
          </div>
        </div>
      )}
    </>
  );
}
