import { FarmPlotAsset } from "../life/LifeAssets";
import * as THREE from "three";
const wood = new THREE.MeshStandardMaterial({ color: "#8d7150", roughness: 1 }),
  stone = new THREE.MeshStandardMaterial({ color: "#9aa7a8", roughness: 1 }),
  roof = new THREE.MeshStandardMaterial({ color: "#496c63", roughness: 1 }),
  fire = new THREE.MeshStandardMaterial({
    color: "#ffc879",
    emissive: "#dc8131",
    emissiveIntensity: 0.5,
  });
const box = new THREE.BoxGeometry(1, 1, 1),
  rock = new THREE.DodecahedronGeometry(1, 0),
  cone = new THREE.ConeGeometry(1, 1, 4);
export interface BuildingAssetProps {
  material?: THREE.Material;
  scaffold?: boolean;
}
function Part({
  position,
  scale,
  material,
  rotation = 0,
}: {
  position: [number, number, number];
  scale: [number, number, number];
  material: THREE.Material;
  rotation?: number;
}) {
  return (
    <mesh
      geometry={box}
      material={material}
      position={position}
      scale={scale}
      rotation={[0, rotation, 0]}
      castShadow
      receiveShadow
    />
  );
}
function Campfire({ material, scaffold }: BuildingAssetProps) {
  return (
    <group dispose={null}>
      {Array.from({ length: 7 }, (_, i) => (
        <mesh
          key={i}
          geometry={rock}
          material={material ?? stone}
          position={[
            Math.sin((i * Math.PI * 2) / 7) * 0.66,
            0.18,
            Math.cos((i * Math.PI * 2) / 7) * 0.66,
          ]}
          scale={[0.25, 0.18, 0.23]}
          castShadow
        />
      ))}
      <Part
        position={[0, 0.2, 0]}
        scale={[1.15, 0.16, 0.17]}
        material={material ?? wood}
        rotation={0.6}
      />
      <Part
        position={[0, 0.28, 0]}
        scale={[1.15, 0.16, 0.17]}
        material={material ?? wood}
        rotation={-0.6}
      />
      {!scaffold && (
        <mesh
          geometry={cone}
          material={material ?? fire}
          position={[0, 0.65, 0]}
          scale={[0.3, 0.65, 0.3]}
        />
      )}
    </group>
  );
}
function Workbench({ material, scaffold }: BuildingAssetProps) {
  return (
    <group dispose={null}>
      {[-1, 1].flatMap((x) =>
        [-1, 1].map((z) => (
          <Part
            key={`${x}:${z}`}
            position={[x * 1.05, 0.6, z * 0.6]}
            scale={[0.15, 1.2, 0.15]}
            material={material ?? wood}
          />
        )),
      )}
      <Part
        position={[0, scaffold ? 0.25 : 1.22, 0]}
        scale={[2.65, 0.2, 1.6]}
        material={material ?? wood}
      />
      {!scaffold && (
        <Part
          position={[0.6, 1.5, 0.1]}
          scale={[0.5, 0.4, 0.45]}
          material={material ?? stone}
        />
      )}
    </group>
  );
}
function Shelter({ material, scaffold }: BuildingAssetProps) {
  return (
    <group dispose={null}>
      <Part
        position={[0, 0.12, 0]}
        scale={[4.7, 0.24, 3.7]}
        material={material ?? wood}
      />
      {[-1, 1].flatMap((x) =>
        [-1, 1].map((z) => (
          <Part
            key={`${x}:${z}`}
            position={[x * 2.2, scaffold ? 0.7 : 1.4, z * 1.7]}
            scale={[0.22, scaffold ? 1.4 : 2.8, 0.22]}
            material={material ?? wood}
          />
        )),
      )}
      {!scaffold && (
        <>
          <Part
            position={[0, 1.3, -1.7]}
            scale={[4.5, 2.5, 0.18]}
            material={material ?? wood}
          />
          {[-1, 1].map((x) => (
            <Part
              key={x}
              position={[x * 2.2, 1.3, 0]}
              scale={[0.18, 2.5, 3.4]}
              material={material ?? wood}
            />
          ))}
          <mesh
            geometry={cone}
            material={material ?? roof}
            position={[0, 3.1, 0]}
            rotation={[0, Math.PI / 4, 0]}
            scale={[3.65, 1.8, 3.1]}
            castShadow
          />
        </>
      )}
    </group>
  );
}
const registry: Record<string, (props: BuildingAssetProps) => React.ReactNode> =
  {
    "building.farm-plot.basic": FarmPlotAsset,
    "building.campfire.basic": Campfire,
    "building.workbench.basic": Workbench,
    "building.shelter.basic": Shelter,
  };
export function BuildingAsset({
  id,
  ...props
}: BuildingAssetProps & { id: string }) {
  const Component = registry[id];
  return Component ? <Component {...props} /> : null;
}
