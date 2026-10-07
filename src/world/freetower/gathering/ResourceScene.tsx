import { useRef } from "react";
import { useFrame, type ThreeEvent } from "@react-three/fiber";
import { Html } from "@react-three/drei";
import * as THREE from "three";
import { terrainHeight } from "../model";
import { Asset } from "./Assets";
import { definitions, nodes, type GatheringController } from "./model";
const chipGeometry = new THREE.IcosahedronGeometry(0.075, 0),
  chipMaterial = new THREE.MeshStandardMaterial({ color: "#d2bd8a" });
const dummy = new THREE.Object3D();
export default function ResourceScene({
  controller: c,
}: {
  controller: GatheringController;
}) {
  const groups = useRef<(THREE.Group | null)[]>([]),
    rings = useRef<(THREE.Mesh | null)[]>([]),
    flashes = useRef<(THREE.Mesh | null)[]>([]),
    chips = useRef<THREE.InstancedMesh>(null);
  useFrame((_, dt) => {
    const now = Date.now() + c.serverOffset;
    nodes.forEach((n, i) => {
      const state = c.state?.nodes[n.id],
        hit = c.hits[n.id],
        age = hit ? (now - hit.at) / 1000 : 99;
      const group = groups.current[i];
      if (!group) return;
      const target = state && !state.depleted ? n.scale : 0;
      const scale = THREE.MathUtils.damp(group.scale.x, target, 8, dt);
      group.scale.setScalar(scale);
      group.visible = scale > 0.015;
      group.rotation.z =
        age >= 0 && age < 0.4
          ? Math.sin(age * 45) * 0.055 * (1 - age / 0.4)
          : 0;
      if (rings.current[i]) {
        rings.current[i]!.visible = Boolean(state && !state.depleted);
        (rings.current[i]!.material as THREE.MeshBasicMaterial).opacity =
          c.targetId === n.id ? 0.95 : c.hoverId === n.id ? 0.7 : 0.25;
      }
      if (flashes.current[i]) {
        flashes.current[i]!.visible = age >= 0 && age < 0.22;
        (flashes.current[i]!.material as THREE.MeshBasicMaterial).opacity =
          Math.max(0, 0.38 - age * 1.7);
      }
      for (let k = 0; k < 6; k++) {
        const live = age >= 0 && age < 0.65;
        const a = (k * Math.PI) / 3;
        dummy.position.set(
          n.position.x + Math.cos(a) * age * 2,
          terrainHeight(n.position) + 1 + age * 2.4 - age * age * 5,
          n.position.z + Math.sin(a) * age * 2,
        );
        dummy.scale.setScalar(live ? 1 - age / 0.65 : 0);
        dummy.rotation.set(age * 4 + k, age * 3, age * 6);
        dummy.updateMatrix();
        chips.current?.setMatrixAt(i * 6 + k, dummy.matrix);
      }
    });
    if (chips.current) chips.current.instanceMatrix.needsUpdate = true;
  });
  const selected = c.state?.nodes[c.targetId ?? ""];
  const pointer = (e: ThreeEvent<PointerEvent>, id: string) => {
    if (e.button !== 0 || c.wheel) return;
    e.stopPropagation();
    if (c.targetId === id) c.use();
    else c.select(id);
  };
  return (
    <group>
      {nodes.map((n, i) => (
        <group
          key={n.id}
          position={[n.position.x, terrainHeight(n.position), n.position.z]}
        >
          <group
            ref={(r) => {
              groups.current[i] = r;
            }}
            scale={n.scale}
            onPointerDown={(e) => pointer(e, n.id)}
            onPointerOver={(e) => {
              e.stopPropagation();
              if (!c.state?.nodes[n.id]?.depleted) c.hoverId = n.id;
            }}
            onPointerOut={() => {
              if (c.hoverId === n.id) c.hoverId = null;
            }}
          >
            <Asset id={definitions[n.definitionId].assetId} />
            <mesh
              ref={(r) => {
                flashes.current[i] = r;
              }}
              position={[0, 1, 0]}
              scale={[0.58, 1.2, 0.58]}
              raycast={() => {}}
            >
              <icosahedronGeometry args={[1, 0]} />
              <meshBasicMaterial
                color="#fff4b5"
                transparent
                opacity={0}
                depthWrite={false}
              />
            </mesh>
          </group>
          <mesh
            ref={(r) => {
              rings.current[i] = r;
            }}
            rotation={[-Math.PI / 2, 0, 0]}
            position={[0, 0.06, 0]}
            raycast={() => {}}
          >
            <ringGeometry args={[0.9, 1.02, 24]} />
            <meshBasicMaterial
              color="#edcd88"
              transparent
              opacity={0.25}
              depthWrite={false}
            />
          </mesh>
        </group>
      ))}
      {selected && !selected.depleted && (
        <Html
          position={[
            selected.position.x,
            terrainHeight(selected.position) + 2.5,
            selected.position.z,
          ]}
          center
          distanceFactor={18}
          style={{ pointerEvents: "none" }}
        >
          <span className="ft-resource-label">
            {selected.durability}/{selected.maxDurability}
          </span>
        </Html>
      )}
      <instancedMesh
        ref={chips}
        args={[chipGeometry, chipMaterial, nodes.length * 6]}
        dispose={null}
        frustumCulled={false}
        raycast={() => {}}
      />
    </group>
  );
}
