import { useEffect, useMemo, useReducer, useRef } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import { Html } from "@react-three/drei";
import * as THREE from "three";
import type { BuildingController } from "./controller";
import { buildingDefinitions } from "./types";
import { BuildingAsset } from "./BuildingAssets";
function BuildingGhost({ controller: c }: { controller: BuildingController }) {
  const group = useRef<THREE.Group>(null),
    footprint = useRef<THREE.Mesh>(null);
  const { camera, gl, scene } = useThree();
  const pointer = useRef(new THREE.Vector2()),
    tracking = useRef(false),
    session = useRef(-1),
    lastPointer = useRef<{ x: number; y: number } | null>(null),
    ray = useMemo(() => new THREE.Raycaster(), []);
  const material = useMemo(
    () =>
      new THREE.MeshBasicMaterial({
        color: "#5cd8ad",
        transparent: true,
        opacity: 0.34,
        depthWrite: false,
      }),
    [],
  );
  const outline = useMemo(
    () =>
      new THREE.MeshBasicMaterial({
        color: "#5cd8ad",
        transparent: true,
        opacity: 0.25,
        side: THREE.DoubleSide,
        depthWrite: false,
      }),
    [],
  );
  useEffect(
    () => () => {
      material.dispose();
      outline.dispose();
    },
    [material, outline],
  );
  const project = () => {
    const ground = scene.getObjectByName("ft-placement-ground");
    ray.setFromCamera(pointer.current, camera);
    const hit = ground ? ray.intersectObject(ground, false)[0] : undefined;
    c.setGhost(hit ? { x: hit.point.x, z: hit.point.z } : null);
  };
  useEffect(() => {
    const canvas = gl.domElement;
    const record = (e: PointerEvent) => {
      const previous = lastPointer.current;
      lastPointer.current = { x: e.clientX, y: e.clientY };
      const b = canvas.getBoundingClientRect();
      pointer.current.set(((e.clientX - b.left) / b.width) * 2 - 1, (-(e.clientY - b.top) / b.height) * 2 + 1);
      return !previous || previous.x !== e.clientX || previous.y !== e.clientY;
    };
    const move = (e: PointerEvent) => {
      const changed = record(e);
      if (session.current !== c.previewSession) {
        session.current = c.previewSession;
        tracking.current = c.mode === "placing" && !!lastPointer.current;
      }
      if (c.editing && changed) tracking.current = true;
    };
    const down = (e: PointerEvent) => {
      if (!c.editing || e.button !== 0) return;
      record(e);
      tracking.current = true;
      project();
    };
    canvas.addEventListener("pointermove", move);
    canvas.addEventListener("pointerdown", down, true);
    return () => {
      canvas.removeEventListener("pointermove", move);
      canvas.removeEventListener("pointerdown", down, true);
    };
  }, [c, gl, camera, scene, ray]);
  useFrame(() => {
    if (!group.current) return;
    if (session.current !== c.previewSession) {
      session.current = c.previewSession;
      tracking.current = c.mode === "placing" && !!lastPointer.current;
    }
    group.current.visible = c.editing;
    if (!c.editing) return;
    // Follow the current camera every frame after an actual pointer movement.
    // A newly granted move keeps the building's original location until then.
    if (tracking.current) project();
    group.current.visible = !!c.ghost;
    if (!c.ghost) return;
    const v = c.validation();
    group.current.position.set(c.ghost.x, v.y + 0.04, c.ghost.z);
    group.current.rotation.y = c.rotationY;
    material.color.set(v.valid ? "#67e2b8" : "#f57473");
    outline.color.copy(material.color);
    const d = buildingDefinitions[c.definitionId];
    if (footprint.current && d)
      footprint.current.scale.set(d.footprint.width, d.footprint.depth, 1);
  });
  const d = buildingDefinitions[c.definitionId];
  return (
    <group ref={group} visible={c.editing}>
      <BuildingAsset id={d?.assetId ?? ""} material={material} />
      <mesh
        ref={footprint}
        rotation={[-Math.PI / 2, 0, 0]}
        position={[0, 0.03, 0]}
        material={outline}
      >
        <planeGeometry args={[1, 1]} />
      </mesh>
    </group>
  );
}
export default function BuildingScene({
  controller: c,
}: {
  controller: BuildingController;
}) {
  const [, refresh] = useReducer((n) => n + 1, 0);
  useEffect(() => {
    const t = setInterval(refresh, 100);
    return () => clearInterval(t);
  }, []);
  return (
    <group>
      {Object.values(c.state?.buildings ?? {}).map((b) => {
        const d = buildingDefinitions[b.definitionId];
        if (!d) return null;
        const remaining = Math.max(
          0,
          Math.ceil(
            (b.constructionEndsAt - Date.now() - c.serverOffset) / 1000,
          ),
        );
        return (
          <group
            key={b.id}
            position={[b.position.x, b.position.y, b.position.z]}
            rotation={[0, b.rotationY, 0]}
            onPointerDown={(e) => {
              if (e.button !== 0) return;
              e.stopPropagation();
              if (c.editing) c.confirm();
              else c.select(b.id);
            }}
          >
            <BuildingAsset
              id={d.assetId}
              scaffold={b.status === "constructing"}
            />
            {(b.status !== "completed" || c.selectedId === b.id) && (
              <Html
                center
                position={[0, b.status === "constructing" ? 1.8 : 3.9, 0]}
                style={{ pointerEvents: "none" }}
              >
                <span className="ft-building-label">
                  {d.displayName} ·{" "}
                  {b.status === "constructing"
                    ? `${remaining}s`
                    : b.status === "moving"
                      ? "Moving…"
                      : "Move available"}
                </span>
              </Html>
            )}
          </group>
        );
      })}
      <BuildingGhost controller={c} />
    </group>
  );
}
