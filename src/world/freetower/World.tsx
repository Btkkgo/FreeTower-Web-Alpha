import { publicCharacterName } from "../../identity/model";
import LifeScene from "./life/LifeScene";
import type {LifeController} from "./life/controller";
import {activeItem} from "./gathering/model";
import BuildingScene from "./building/BuildingScene";
import type { BuildingController } from "./building/controller";
import { blocksMovement } from "./building/validation";
import {
  Canvas,
  useFrame,
  useThree,
  type ThreeEvent,
} from "@react-three/fiber";
import Explorer from "./Explorer";
import ResourceScene from "./gathering/ResourceScene";
import {
  nodes as resourceNodes,
  cancelApproach,
  distanceTo,
  config as gatheringConfig,
  type GatheringController,
} from "./gathering/model";
import { useEffect, useMemo, useRef, type MutableRefObject } from "react";
import * as THREE from "three";
import type { Player } from "../../game/types";
import {
  advanceMovement,
  constrainPosition,
  MovementSender,
  cameraInput,
  cameraStop,
  cameraObstacles,
  cancelNavigation,
  locomotionAt,
  movePayload,
  regionAt,
  riverCenter,
  terrainHeight,
  trees,
  worldConfig,
  type Point,
  type MovementMode,
} from "./model";

export interface Motion {
  position: Point;
  facing: number;
  moving: boolean;
  running: boolean;
  jumpAt: number;
  mode: MovementMode;
  elevation: number;
}
export interface Readout {
  fps: number;
  calls: number;
  triangles: number;
  position: Point;
  mode: MovementMode;
  water: string;
  region: string;
}
interface Props {
  life: LifeController;
  building: BuildingController;
  gathering: GatheringController;
  players: Player[];
  selfId?: string;
  motion: MutableRefObject<Motion>;
  ready: boolean;
  onMove: (m: ReturnType<typeof movePayload>) => void;
  onReadout: (r: Readout) => void;
  debug: boolean;
}
const stone = "#a9b4b7",
  gold = "#d4b779";
const dummy = new THREE.Object3D();
function Landscape({ onTarget }: { onTarget: (p: Point) => void }) {
  const mesh = useMemo(() => {
    const g = new THREE.PlaneGeometry(190, 190, 190, 190);
    g.rotateX(-Math.PI / 2);
    const a = g.attributes.position;
    const colors = [];
    const color = new THREE.Color();
    for (let i = 0; i < a.count; i++) {
      const x = a.getX(i),
        z = a.getZ(i),
        h = terrainHeight({ x, z });
      a.setY(i, h);
      const path =
        Math.abs(x - Math.sin(z * 0.06) * 4) < 1.6 ||
        (z < -9 && Math.abs(z + 19 + Math.sin(x * 0.08) * 5) < 1.5);
      color.set(
        h < 0.6
          ? "#b6b28a"
          : z > 33
            ? "#c6bf90"
            : h > 6
              ? "#859693"
              : path
                ? "#a9a277"
                : x < -16 && z < 7
                  ? "#597b4e"
                  : "#8aab64",
      );
      color.multiplyScalar(
        0.97 + 0.025 * Math.sin(x * 0.45) * Math.cos(z * 0.41),
      );
      colors.push(color.r, color.g, color.b);
    }
    g.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));
    g.computeVertexNormals();
    return g;
  }, []);
  useEffect(() => () => mesh.dispose(), [mesh]);
  return (
    <mesh
      geometry={mesh}
      name="ft-placement-ground"
      receiveShadow
      onPointerDown={(e: ThreeEvent<PointerEvent>) => {
        if (e.button === 0) {
          e.stopPropagation();
          onTarget({ x: e.point.x, z: e.point.z });
        }
      }}
    >
      <meshStandardMaterial vertexColors roughness={1} />
    </mesh>
  );
}
function Forest() {
  const trunk = useRef<THREE.InstancedMesh>(null),
    crown = useRef<THREE.InstancedMesh>(null),
    top = useRef<THREE.InstancedMesh>(null);
  useEffect(() => {
    trees.forEach((p, i) => {
      const y = terrainHeight(p),
        s = resourceNodes.some((n) => n.treeIndex === i) ? 0 : p.scale;
      dummy.position.set(p.x, y + 2 * s, p.z);
      dummy.scale.set(s, 4 * s, s);
      dummy.rotation.set(0, i, 0);
      dummy.updateMatrix();
      trunk.current!.setMatrixAt(i, dummy.matrix);
      dummy.position.y = y + 5 * s;
      dummy.scale.set(2.5 * s, 3 * s, 2.5 * s);
      dummy.updateMatrix();
      crown.current!.setMatrixAt(i, dummy.matrix);
      dummy.position.y = y + 7 * s;
      dummy.scale.set(1.8 * s, 2.4 * s, 1.8 * s);
      dummy.updateMatrix();
      top.current!.setMatrixAt(i, dummy.matrix);
    });
    [trunk, crown, top].forEach((r) => {
      r.current!.instanceMatrix.needsUpdate = true;
      r.current!.computeBoundingSphere();
    });
  }, []);
  return (
    <group>
      <instancedMesh
        ref={trunk}
        args={[undefined, undefined, trees.length]}
        castShadow
      >
        <cylinderGeometry args={[0.25, 0.42, 1, 5]} />
        <meshStandardMaterial color="#665746" />
      </instancedMesh>
      <instancedMesh
        ref={crown}
        args={[undefined, undefined, trees.length]}
        castShadow
      >
        <icosahedronGeometry args={[1, 0]} />
        <meshStandardMaterial color="#3f7654" roughness={1} />
      </instancedMesh>
      <instancedMesh
        ref={top}
        args={[undefined, undefined, trees.length]}
        castShadow
      >
        <icosahedronGeometry args={[1, 0]} />
        <meshStandardMaterial color="#548656" roughness={1} />
      </instancedMesh>
    </group>
  );
}
function Rocks() {
  const ref = useRef<THREE.InstancedMesh>(null);
  useEffect(() => {
    for (let i = 0; i < 70; i++) {
      const angle = i * 2.4,
        r = 7 + Math.sqrt(i / 70) * 25;
      const x = 34 + Math.cos(angle) * r,
        z = -22 + Math.sin(angle) * r;
      if (Math.abs(z - riverCenter(x)) < 6) dummy.scale.setScalar(0);
      else
        dummy.scale.set(
          1 + (i % 4) * 0.4,
          0.6 + (i % 5) * 0.25,
          1 + (i % 3) * 0.6,
        );
      dummy.position.set(x, terrainHeight({ x, z }) - 0.2, z);
      dummy.rotation.set(0.1 * i, 0.8 * i, 0.2 * i);
      dummy.updateMatrix();
      ref.current!.setMatrixAt(i, dummy.matrix);
    }
    ref.current!.instanceMatrix.needsUpdate = true;
    ref.current!.computeBoundingSphere();
  }, []);
  return (
    <instancedMesh ref={ref} args={[undefined, undefined, 70]} castShadow>
      <dodecahedronGeometry args={[1, 0]} />
      <meshStandardMaterial color={stone} roughness={0.95} />
    </instancedMesh>
  );
}
function GroundDetails() {
  const ref = useRef<THREE.InstancedMesh>(null);
  const items = useMemo(
    () =>
      Array.from({ length: 450 }, (_, i) => {
        const x = Math.sin(i * 12.9898) * 70,
          z = Math.cos(i * 7.233) * 60;
        return { x, z };
      }).filter(
        (p) =>
          terrainHeight(p) > 0.8 &&
          Math.abs(p.x - Math.sin(p.z * 0.06) * 4) > 2.5,
      ),
    [],
  );
  useEffect(() => {
    items.forEach((p, i) => {
      dummy.position.set(p.x, terrainHeight(p) + 0.2, p.z);
      dummy.rotation.set(0, i * 2.1, 0.1);
      dummy.scale.set(
        0.25 + (i % 3) * 0.12,
        0.22 + (i % 5) * 0.08,
        0.25 + (i % 3) * 0.12,
      );
      dummy.updateMatrix();
      ref.current!.setMatrixAt(i, dummy.matrix);
      ref.current!.setColorAt(
        i,
        new THREE.Color(
          i % 8 === 0 ? "#d6d3af" : i % 5 === 0 ? "#94a386" : "#6f9456",
        ),
      );
    });
    ref.current!.instanceMatrix.needsUpdate = true;
    ref.current!.computeBoundingSphere();
  }, [items]);
  return (
    <instancedMesh ref={ref} args={[undefined, undefined, items.length]}>
      <icosahedronGeometry args={[1, 0]} />
      <meshStandardMaterial roughness={1} />
    </instancedMesh>
  );
}
function Water({ onTarget }: { onTarget: (p: Point) => void }) {
  const river = useMemo(() => {
    const v = [],
      idx = [];
    for (let i = 0; i <= 190; i++) {
      const x = i - 95;
      for (const side of [-1, 1]) v.push(x, 0.015, riverCenter(x) + side * 3.2);
      if (i < 190) {
        const n = i * 2;
        idx.push(n, n + 2, n + 1, n + 1, n + 2, n + 3);
      }
    }
    const g = new THREE.BufferGeometry();
    g.setAttribute("position", new THREE.Float32BufferAttribute(v, 3));
    g.setIndex(idx);
    g.computeVertexNormals();
    return g;
  }, []);
  useEffect(() => () => river.dispose(), [river]);
  const mat = useRef<THREE.MeshStandardMaterial>(null);
  useFrame(({ clock }) => {
    if (mat.current)
      mat.current.opacity = 0.68 + Math.sin(clock.elapsedTime * 0.4) * 0.025;
  });
  const click = (e: ThreeEvent<PointerEvent>) => {
    if (e.button === 0) {
      e.stopPropagation();
      onTarget({ x: e.point.x, z: e.point.z });
    }
  };
  return (
    <group>
      <mesh geometry={river} onPointerDown={click}>
        <meshStandardMaterial
          ref={mat}
          color="#5dc1ca"
          transparent
          opacity={0.7}
          roughness={0.25}
          metalness={0.2}
          side={THREE.DoubleSide}
        />
      </mesh>
      <mesh
        position={[0, 0.01, 230]}
        rotation={[-Math.PI / 2, 0, 0]}
        onPointerDown={click}
      >
        <planeGeometry args={[1000, 400]} />
        <meshStandardMaterial
          color="#3aa7b9"
          transparent
          opacity={0.8}
          roughness={0.24}
          metalness={0.22}
        />
      </mesh>
      {Array.from({ length: 18 }, (_, i) => (
        <mesh
          key={i}
          position={[Math.sin(i * 6.7) * 64, 0.027, 43 + i * 4.8]}
          rotation={[-Math.PI / 2, 0, 0.05 * Math.sin(i)]}
        >
          <planeGeometry args={[18 + (i % 4) * 9, 0.045]} />
          <meshBasicMaterial color="#b5eeeb" transparent opacity={0.28} />
        </mesh>
      ))}
    </group>
  );
}
function Tower() {
  const { x, z, height } = worldConfig.tower;
  return (
    <group position={[x, -2, z]}>
      <mesh position={[0, 2, 0]}>
        <cylinderGeometry args={[14, 22, 6, 8]} />
        <meshStandardMaterial color="#909e9e" />
      </mesh>
      {[0, 1, 2, 3, 4].map((i) => (
        <group
          key={i}
          position={[0, 6 + i * 11, 0]}
          rotation={[0, i * 0.12, 0]}
        >
          <mesh castShadow position={[0, 5, 0]}>
            <cylinderGeometry args={[5.7 - i * 0.95, 7 - i * 1.04, 11, 8]} />
            <meshStandardMaterial color="#e4e4d0" roughness={0.7} />
          </mesh>
          <mesh position={[0, 10, 0]}>
            <cylinderGeometry
              args={[6.1 - i * 0.95, 6.1 - i * 0.95, 0.75, 8]}
            />
            <meshStandardMaterial color={gold} />
          </mesh>
          {[0, 1, 2, 3].map((j) => (
            <mesh
              key={j}
              position={[
                Math.sin((j * Math.PI) / 2) * (7 - i),
                4,
                Math.cos((j * Math.PI) / 2) * (7 - i),
              ]}
            >
              <boxGeometry args={[0.8, 11, 0.8]} />
              <meshStandardMaterial color="#d8d7be" />
            </mesh>
          ))}
        </group>
      ))}
      {[18, 37, 55].map((y, i) => (
        <mesh
          key={y}
          position={[0, y, 0]}
          rotation={[Math.PI / 2 + 0.12 * i, 0, 0.1 * i]}
        >
          <torusGeometry args={[12 - i * 2, 0.45, 6, 48]} />
          <meshStandardMaterial color={gold} metalness={0.4} roughness={0.5} />
        </mesh>
      ))}
      <mesh position={[0, height, 0]}>
        <octahedronGeometry args={[3, 0]} />
        <meshStandardMaterial
          color="#cbffff"
          emissive="#7bdddf"
          emissiveIntensity={0.5}
        />
      </mesh>
      <mesh position={[0, height - 3, 0]}>
        <coneGeometry args={[2, 6, 8]} />
        <meshStandardMaterial color={gold} />
      </mesh>
    </group>
  );
}
function Ruins() {
  return (
    <group>
      {[-1, 1].flatMap((side) =>
        [0, 1, 2].map((i) => {
          const x = side * 7,
            z = -53 - i * 5,
            y = terrainHeight({ x, z });
          return (
            <group
              key={`${side}-${i}`}
              position={[x, y, z]}
              rotation={[0, 0.1 * i, side * 0.04]}
            >
              <mesh position={[0, 2.5, 0]} castShadow>
                <cylinderGeometry args={[0.7, 0.95, 5, 6]} />
                <meshStandardMaterial color="#cac9b4" />
              </mesh>
              <mesh position={[0, 5, 0]}>
                <boxGeometry args={[2, 0.55, 2]} />
                <meshStandardMaterial color={gold} />
              </mesh>
            </group>
          );
        }),
      )}
      <mesh position={[0, terrainHeight({ x: 0, z: -58 }) + 0.1, -58]}>
        <cylinderGeometry args={[9, 10, 0.5, 12]} />
        <meshStandardMaterial color="#aaa99b" />
      </mesh>
      <mesh position={[0, terrainHeight({ x: 0, z: -64 }) + 4, -64]}>
        <torusGeometry args={[3.5, 0.65, 5, 10, Math.PI * 1.65]} />
        <meshStandardMaterial color="#b7baae" />
      </mesh>
    </group>
  );
}
function Boundaries() {
  return (
    <group>
      {Array.from({ length: 29 }, (_, i) => {
        const a = Math.PI + (i * Math.PI) / 28;
        const x = Math.cos(a) * 100,
          z = Math.sin(a) * 95;
        return (
          <mesh key={i} position={[x, 3, z]} rotation={[0, i * 0.7, 0]}>
            <coneGeometry args={[12 + (i % 5), 17 + (i % 7) * 2, 5]} />
            <meshStandardMaterial color="#829b98" />
          </mesh>
        );
      })}
    </group>
  );
}
function Runtime({
  players,
  selfId,
  motion,
  ready,
  onMove,
  onReadout,
  debug,
  gathering,
  building,
  life,
}: Props) {
  const { camera, gl } = useThree();
  const keys = useRef(new Set<string>()),
    route = useRef<Point[]>([]),
    orbit = useRef({
      yaw: Math.PI,
      pitch: 0.62,
      distance: 17,
      drag: false,
      x: 0,
      y: 0,
    }),
    pendingJump = useRef(false),
    sender = useRef(new MovementSender()),
    stuckTime = useRef(0),
    frames = useRef(0),
    lastRead = useRef(0),
    target = useRef(new THREE.Vector3()),
    cameraInit = useRef(false),
    marker = useRef<THREE.Mesh>(null);
  useEffect(() => {
    if (!ready) {
      route.current = [];
      keys.current.clear();
      pendingJump.current = false;
      sender.current = new MovementSender();
      stuckTime.current = 0;
    }
  }, [ready]);
  useEffect(() => {
    const canvas = gl.domElement;
    canvas.tabIndex = 0;
    const key = (e: KeyboardEvent, down: boolean) => {
      if (!down) keys.current.delete(e.code);
      if (
        (e.target as HTMLElement).matches(
          "input,textarea,button,[contenteditable]",
        )
      )
        return;
      if (down && e.code === "BracketRight") orbit.current.yaw += Math.PI / 4;
      if (down && e.code === "BracketLeft") orbit.current.yaw -= Math.PI / 4;
      if (down && e.code === "Home") {
        orbit.current.yaw = Math.PI;
        orbit.current.pitch = 0.62;
        orbit.current.distance = 17;
      }
      if (
        [
          "KeyW",
          "KeyA",
          "KeyS",
          "KeyD",
          "ArrowUp",
          "ArrowDown",
          "ArrowLeft",
          "ArrowRight",
          "Space",
          "ShiftLeft",
          "ShiftRight",
        ].includes(e.code)
      ) {
        e.preventDefault();
        if (down) {
          keys.current.add(e.code);
          if (e.code === "Space" && !e.repeat) pendingJump.current = true;
        } else keys.current.delete(e.code);
      }
    };
    const kd = (e: KeyboardEvent) => key(e, true),
      ku = (e: KeyboardEvent) => key(e, false),
      clear = () => {
        keys.current.clear();
        orbit.current.drag = false;
      };
    const down = (e: PointerEvent) => {
      canvas.focus({ preventScroll: true });
      if (e.button === 2 && !building.active) {
        orbit.current.drag = true;
        orbit.current.x = e.clientX;
        orbit.current.y = e.clientY;
        canvas.setPointerCapture(e.pointerId);
      }
    };
    const move = (e: PointerEvent) => {
      if (orbit.current.drag) {
        orbit.current.yaw -= (e.clientX - orbit.current.x) * 0.006;
        orbit.current.pitch = THREE.MathUtils.clamp(
          orbit.current.pitch + (e.clientY - orbit.current.y) * 0.004,
          0.42,
          1.02,
        );
        orbit.current.x = e.clientX;
        orbit.current.y = e.clientY;
      }
    };
    const up = () => {
      orbit.current.drag = false;
    };
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      orbit.current.distance = THREE.MathUtils.clamp(
        orbit.current.distance + e.deltaY * 0.012,
        9,
        30,
      );
    };
    const context = (e: Event) => e.preventDefault();
    window.addEventListener("keydown", kd);
    window.addEventListener("keyup", ku);
    window.addEventListener("blur", clear);
    window.addEventListener("focusin", clear);
    canvas.addEventListener("pointerdown", down);
    canvas.addEventListener("pointermove", move);
    canvas.addEventListener("pointerup", up);
    canvas.addEventListener("lostpointercapture", up);
    canvas.addEventListener("wheel", wheel, { passive: false });
    canvas.addEventListener("contextmenu", context);
    return () => {
      window.removeEventListener("keydown", kd);
      window.removeEventListener("keyup", ku);
      window.removeEventListener("blur", clear);
      window.removeEventListener("focusin", clear);
      canvas.removeEventListener("pointerdown", down);
      canvas.removeEventListener("pointermove", move);
      canvas.removeEventListener("pointerup", up);
      canvas.removeEventListener("lostpointercapture", up);
      canvas.removeEventListener("wheel", wheel);
      canvas.removeEventListener("contextmenu", context);
    };
  }, [gl, building]);
  useFrame(({ clock }, dt) => {
    building.actor = motion.current.position;
    building.movementMode = motion.current.mode;
    building.tick();
    life.position=motion.current.position;
    life.facing=motion.current.facing;
    life.mode=motion.current.mode;
    life.tool=activeItem(gathering.hands);
    if(building.active && life.active)life.cancel();
    gathering.suspended = building.active || life.fishing;
    if(life.active){gathering.approachId=null;route.current=[];}

    if (building.active) {
      gathering.cancelInput();
      route.current = [];
    }
    gathering.position = motion.current.position;
    gathering.mode = motion.current.mode;
    gathering.tick();
    const m = motion.current,
      k = keys.current,
      o = orbit.current;
    const sx =
        (k.has("KeyD") || k.has("ArrowRight") ? 1 : 0) -
        (k.has("KeyA") || k.has("ArrowLeft") ? 1 : 0),
      sz =
        (k.has("KeyW") || k.has("ArrowUp") ? 1 : 0) -
        (k.has("KeyS") || k.has("ArrowDown") ? 1 : 0);
    let input = cameraInput(sx, sz, o.yaw);
    route.current = cancelNavigation(route.current, input);
    gathering.approachId = cancelApproach(gathering.approachId, input);
    if (gathering.wheel || gathering.action) route.current = [];

    if (!sx && !sz && gathering.approachId) {
      const resource = gathering.state?.nodes[gathering.approachId];
      route.current = [];
      if (
        !resource ||
        resource.depleted ||
        distanceTo(m.position, resource.position) <=
          gatheringConfig.interactionRange - 0.3
      )
        gathering.approachId = null;
      else
        input = {
          x: resource.position.x - m.position.x,
          z: resource.position.z - m.position.z,
        };
    }
    if (!sx && !sz && route.current.length) {
      const p = route.current[0],
        dx = p.x - m.position.x,
        dz = p.z - m.position.z;
      if (Math.hypot(dx, dz) < 0.25) route.current.shift();
      else input = { x: dx, z: dz };
    }
    m.moving =
      ready && !gathering.locked() && Math.hypot(input.x, input.z) > 0.01;
    m.running = m.moving && (k.has("ShiftLeft") || k.has("ShiftRight"));
    if (m.moving) {
      const before = m.position;
      m.position = advanceMovement(
        m.position,
        input,
        m.running,
        dt,
        resourceNodes
          .filter((n) => gathering.state?.nodes[n.id]?.depleted)
          .map((n) => n.treeIndex)
          .filter((n): n is number => n !== undefined),
      );
      if (
        blocksMovement(
          m.position,
          Object.values(building.state?.buildings ?? {}),
          before,
        )
      )
        m.position = before;
      stuckTime.current =
        Math.hypot(m.position.x - before.x, m.position.z - before.z) < 0.002
          ? stuckTime.current + dt
          : 0;
      if (stuckTime.current > 0.5) {
        route.current = [];
        gathering.approachId = null;
        m.moving = false;
      }
      m.facing = Math.atan2(input.x, input.z);
    }
    if (gathering.action) {
      const resource = gathering.state?.nodes[gathering.action.resourceId];
      if (resource)
        m.facing = Math.atan2(
          resource.position.x - m.position.x,
          resource.position.z - m.position.z,
        );
    }
    const state = locomotionAt(m.position);
    m.mode = state.mode;
    m.elevation = state.elevation;
    if (m.mode !== "land") m.running = false;
    const jump =
      ready &&
      !gathering.locked() &&
      pendingJump.current &&
      m.mode !== "swimming" &&
      Date.now() - m.jumpAt > 650;
    if (jump) m.jumpAt = Date.now();
    pendingJump.current = false;
    if (ready) {
      const packet = sender.current.next(
        movePayload(m.position, m.facing, m.moving, m.running, jump),
        clock.elapsedTime,
      );
      if (packet) onMove(packet);
    }
    target.current.set(m.position.x, m.elevation + 1, m.position.z);
    const offset = new THREE.Vector3(
      Math.sin(o.yaw) * Math.cos(o.pitch) * o.distance,
      Math.sin(o.pitch) * o.distance,
      Math.cos(o.yaw) * Math.cos(o.pitch) * o.distance,
    );
    const destination = target.current.clone().add(offset);
    const stop = cameraStop(
      target.current,
      { x: destination.x, y: destination.y, z: destination.z },
      cameraObstacles,
    );
    const desired = new THREE.Vector3(stop.x, stop.y, stop.z);
    desired.y = Math.max(
      desired.y,
      terrainHeight({ x: desired.x, z: desired.z }) + 1.5,
    );
    if (!cameraInit.current) {
      camera.position.copy(desired);
      cameraInit.current = true;
    } else camera.position.lerp(desired, 1 - Math.exp(-dt * 8));
    camera.lookAt(target.current);
    if (marker.current) {
      marker.current.visible = route.current.length > 0;
      if (route.current.length) {
        const p = route.current[0];
        marker.current.position.set(
          p.x,
          Math.max(0.06, terrainHeight(p) + 0.06),
          p.z,
        );
      }
    }
    frames.current++;
    if (clock.elapsedTime - lastRead.current >= 0.5) {
      const elapsed = clock.elapsedTime - lastRead.current;
      onReadout({
        fps: Math.round(frames.current / elapsed),
        calls: gl.info.render.calls,
        triangles: gl.info.render.triangles,
        position: { ...m.position },
        mode: m.mode,
        water: state.water.id,
        region: regionAt(m.position),
      });
      frames.current = 0;
      lastRead.current = clock.elapsedTime;
    }
  });
  const onTarget = (p: Point) => {
    if (building.editing) {
      building.setGhost(p);
      building.confirm();
      return;
    }
    if (building.active) return;
    if (life.aiming) { life.cast(p); return; }
    if (life.fishing) return;
    life.selectedPlot=null;life.selectedAnimal=null;
    building.selectedId = null;
    if (ready && !gathering.locked()) {
      gathering.approachId = null;
      route.current = [constrainPosition(p, motion.current.position)];
      stuckTime.current = 0;
    }
  };
  return (
    <>
      <Landscape onTarget={onTarget} />
      <Forest />
      <ResourceScene controller={gathering} />
      <BuildingScene controller={building} />
      <LifeScene controller={life} building={building}/>
      <Rocks />
      <GroundDetails />
      <Water onTarget={onTarget} />
      <Ruins />
      <Tower />
      <Boundaries />
      <Explorer
        motion={motion}
        name="You"
        color="#496b79"
        gathering={gathering}
        life={life}
      />
      {players
        .filter((p) => p.id !== selfId && p.world === "freetower")
        .map((p) => (
          <Explorer
            key={p.id}
            remote={p}
            name={publicCharacterName(p)}
            color={p.color}
            gathering={gathering}
        life={life}
          />
        ))}
      <mesh ref={marker} rotation={[-Math.PI / 2, 0, 0]}>
        <ringGeometry args={[0.25, 0.4, 24]} />
        <meshBasicMaterial color="#f3dc99" side={THREE.DoubleSide} />
      </mesh>
    </>
  );
}
function CameraProjection() {
  const { camera, size } = useThree();
  useEffect(() => {
    if (camera instanceof THREE.PerspectiveCamera) {
      camera.setViewOffset(
        size.width,
        size.height,
        0,
        -size.height * 0.24,
        size.width,
        size.height,
      );
      camera.updateProjectionMatrix();
    }
  }, [camera, size.width, size.height]);
  return null;
}
export default function FreeTowerWorld(props: Props) {
  return (
    <Canvas
      shadows
      dpr={[1, 1.5]}
      camera={{ position: [0, 12, 4], fov: 75, near: 0.1, far: 800 }}
      gl={{ antialias: true, powerPreference: "high-performance" }}
    >
      <CameraProjection />
      <color attach="background" args={["#8fc9dc"]} />
      <fog attach="fog" args={["#a8cfd9", 100, 480]} />
      <hemisphereLight args={["#d8f1ff", "#75834e", 2.1]} />
      <directionalLight
        position={[-30, 70, 35]}
        intensity={2.5}
        castShadow
        shadow-mapSize={[1024, 1024]}
        shadow-camera-left={-65}
        shadow-camera-right={65}
        shadow-camera-top={65}
        shadow-camera-bottom={-65}
        shadow-camera-far={180}
        shadow-bias={-0.001}
      />
      <Runtime {...props} />
    </Canvas>
  );
}
