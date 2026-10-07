import { useEffect, useReducer, useRef } from "react";
import { BuildingController } from "./controller";
import { buildingDefinitions, type BuildingMessage } from "./types";
import type { GatheringController } from "../gathering/model";
import "./building.css";
const focusWorld = () =>
  document
    .querySelector<HTMLCanvasElement>(".ft-world canvas")
    ?.focus({ preventScroll: true });
export function useBuilding(
  send: (m: BuildingMessage) => void,
  ready: boolean,
  roomId?: string,
) {
  const sendRef = useRef(send);
  sendRef.current = send;
  const ref = useRef<BuildingController | null>(null);
  if (!ref.current)
    ref.current = new BuildingController((m) => sendRef.current(m));
  const c = ref.current;
  useEffect(() => {
    c.connection(ready);
  }, [c, ready]);
  useEffect(() => {
    c.reset();
    return () => c.reset();
  }, [c, roomId]);
  return c;
}
export default function BuildingHUD({
  controller: c,
  gathering,
  debug,
}: {
  controller: BuildingController;
  gathering: GatheringController;
  debug: boolean;
}) {
  const [, refresh] = useReducer((n) => n + 1, 0);
  const open = () => {
    gathering.cancelInput();
    gathering.approachId = null;
    c.openMenu();
    focusWorld();
    refresh();
  };
  useEffect(() => {
    const t = setInterval(() => {
      c.tick();
      refresh();
    }, 100);
    return () => clearInterval(t);
  }, [c]);
  useEffect(() => {
    const typing = (t: EventTarget | null) =>
      t instanceof HTMLElement &&
      !!t.closest("input,textarea,[contenteditable=true]");
    const key = (e: KeyboardEvent) => {
      if (typing(e.target) || !c.ready || e.repeat) return;
      if (e.code === "KeyB") {
        e.preventDefault();
        if (c.active) c.cancel();
        else {
          gathering.cancelInput();
          gathering.approachId = null;
          c.openMenu();
        }
        focusWorld();
      }
      if (c.editing && e.code === "KeyR") {
        e.preventDefault();
        c.rotate();
      }
      if (c.active && e.code === "Escape") {
        e.preventDefault();
        c.cancel();
        focusWorld();
      }
      refresh();
    };
    const right = (e: MouseEvent) => {
      if (c.active) {
        e.preventDefault();
        c.cancel();
        refresh();
      }
    };
    const blur = () => {
      if (c.active) c.cancel();
    };
    window.addEventListener("keydown", key);
    window.addEventListener("blur", blur);
    const canvas = document.querySelector("canvas");
    canvas?.addEventListener("contextmenu", right);
    return () => {
      window.removeEventListener("keydown", key);
      window.removeEventListener("blur", blur);
      canvas?.removeEventListener("contextmenu", right);
    };
  }, [c, gathering]);
  const v = c.validation(),
    selected = c.state?.buildings?.[c.selectedId ?? ""],
    def = selected ? buildingDefinitions[selected.definitionId] : null;
  return (
    <>
      <button
        className="ft-build-toggle"
        onClick={open}
        disabled={!c.ready || !!c.pending}
      >
        B · Build
      </button>
      {c.mode === "menu" && (
        <section className="ft-build-menu" aria-label="Build Menu">
          <header>
            <span>BUILD · 营地建造</span>
            <button
              aria-label="Close Build Menu"
              onClick={() => {
                c.cancel();
                focusWorld();
              }}
            >
              ×
            </button>
          </header>
          {Object.values(buildingDefinitions).map((d, i) => {
            const enough = d.cost.every(
              (x) => (c.state?.inventory[x.itemId] ?? 0) >= x.quantity,
            );
            return (
              <button
                key={d.id}
                className="ft-build-choice"
                disabled={!enough}
                onClick={() => {
                  c.choose(d.id);
                  focusWorld();
                  refresh();
                }}
              >
                <span className="ft-building-icon">{["♨", "⊞", "⌂", "▤"][i]}</span>
                <span>
                  <strong>{d.displayName}</strong>
                  <small>
                    {d.cost.map((x) => `${x.itemId} ${x.quantity}`).join(" · ")}
                    <br />
                    {d.constructionSeconds}s{" "}
                    {enough ? "construction" : "· 资源不足"}
                  </small>
                </span>
              </button>
            );
          })}
          <small>自由放置 · R 旋转 · Esc 取消</small>
        </section>
      )}
      {c.editing && (
        <section className="ft-placement-bar" aria-label="Building placement">
          <strong>
            {c.mode === "moving" ? "Move · " : ""}
            {buildingDefinitions[c.definitionId]?.displayName}
          </strong>
          <span className={v.valid ? "valid" : "invalid"}>{v.reason}</span>
          <div>
            <button
              disabled={!v.valid || !!c.pending}
              onClick={() => c.confirm()}
            >
              {c.pending ? "Saving…" : "Place"}
            </button>
            <button
              disabled={!!c.pending}
              onClick={() => {
                c.rotate();
                focusWorld();
              }}
            >
              Rotate R
            </button>
            <button
              disabled={!!c.pending}
              onClick={() => {
                c.cancel();
                focusWorld();
              }}
            >
              Cancel Esc
            </button>
          </div>
        </section>
      )}
      {!c.active && selected && def && selected.definitionId !== "farm-plot" && (
        <section
          className="ft-building-selection"
          aria-label="Selected building"
        >
          <strong>{def.displayName}</strong>
          <small>
            {selected.status === "constructing"
              ? `Construction · ${Math.max(0, Math.ceil((selected.constructionEndsAt - Date.now() - c.serverOffset) / 1000))}s`
              : selected.status === "moving"
                ? "Building currently being moved"
                : "Completed"}
          </small>
          <button
            disabled={selected.status === "constructing" || !!c.pending}
            onClick={() => {
              gathering.cancelInput();
              c.beginMove();
              focusWorld();
            }}
          >
            Move
          </button>
          <button
            aria-label="Close building selection"
            onClick={() => {
              c.selectedId = null;
              focusWorld();
            }}
          >
            ×
          </button>
        </section>
      )}
      {c.message && Date.now() < c.messageUntil && (
        <div className="ft-building-message" role="status">
          {c.message}
        </div>
      )}
      {debug && (
        <aside className="ft-building-debug">
          Building {c.mode} · {c.definitionId || "none"}
          <br />
          Ghost{" "}
          {c.ghost
            ? `${c.ghost.x.toFixed(2)}, ${c.ghost.z.toFixed(2)}`
            : "—"} · {((c.rotationY * 180) / Math.PI).toFixed(0)}°<br />
          Valid {String(v.valid)} · {v.code} · slope {v.slope.toFixed(3)}
          <br />
          Overlap {v.overlapIds.join(",") || "none"}
          <br />
          Buildings {Object.keys(c.state?.buildings ?? {}).length} · Lock{" "}
          {c.moveId ?? "none"}
        </aside>
      )}
    </>
  );
}
