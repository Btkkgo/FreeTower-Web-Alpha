import * as THREE from "three";
import type { AssetId } from "./model";
const wood = new THREE.MeshStandardMaterial({ color: "#826043", roughness: 1 }),
  metal = new THREE.MeshStandardMaterial({
    color: "#b8cace",
    metalness: 0.25,
    roughness: 0.65,
  }),
  leaf = new THREE.MeshStandardMaterial({ color: "#537e51", roughness: 1 }),
  rock = new THREE.MeshStandardMaterial({ color: "#8fa3a7", roughness: 1 });
const shaft = new THREE.CylinderGeometry(0.045, 0.055, 0.75, 6),
  axeHead = new THREE.BoxGeometry(0.34, 0.24, 0.095),
  pickHead = new THREE.ConeGeometry(0.085, 0.55, 4),
  trunk = new THREE.CylinderGeometry(0.25, 0.48, 4, 6),
  crown = new THREE.IcosahedronGeometry(1, 0),
  rockGeo = new THREE.DodecahedronGeometry(1, 0),
  woodItem = new THREE.CylinderGeometry(0.1, 0.1, 0.45, 6);
function Axe() {
  return (
    <group rotation={[0.3, 0, -0.2]}>
      <mesh geometry={shaft} material={wood} position={[0, 0.15, 0]} />
      <mesh geometry={axeHead} material={metal} position={[0.12, 0.48, 0]} />
    </group>
  );
}
function Pickaxe() {
  return (
    <group rotation={[0.3, 0, -0.2]}>
      <mesh geometry={shaft} material={wood} position={[0, 0.15, 0]} />
      <mesh
        geometry={pickHead}
        material={metal}
        rotation={[0, 0, -Math.PI / 2]}
        position={[0.2, 0.48, 0]}
      />
      <mesh
        geometry={pickHead}
        material={metal}
        rotation={[0, 0, Math.PI / 2]}
        position={[-0.2, 0.48, 0]}
      />
    </group>
  );
}
function Tree() {
  return (
    <>
      <mesh geometry={trunk} material={wood} position={[0, 2, 0]} castShadow />
      <mesh
        geometry={crown}
        material={leaf}
        position={[0, 5, 0]}
        scale={[2.8, 3.1, 2.6]}
        castShadow
      />
      <mesh
        geometry={crown}
        material={leaf}
        position={[0, 7, 0]}
        scale={[2.1, 2.7, 2.1]}
        castShadow
      />
    </>
  );
}
function Rock() {
  return (
    <>
      <mesh
        geometry={rockGeo}
        material={rock}
        position={[0, 0.8, 0]}
        scale={[1.1, 0.85, 0.9]}
        castShadow
      />
      <mesh
        geometry={rockGeo}
        material={metal}
        position={[0.55, 0.4, 0.2]}
        scale={0.48}
      />
    </>
  );
}
function Wood() {
  return (
    <mesh geometry={woodItem} material={wood} rotation={[Math.PI / 2, 0, 0]} />
  );
}
function Stone() {
  return <mesh geometry={rockGeo} material={rock} scale={0.18} />;
}
/** Only this registry knows which mesh implements a gameplay AssetId. */
const assetRegistry: Record<AssetId, () => React.ReactNode> = {
  "tool.axe.basic": Axe,
  "tool.pickaxe.basic": Pickaxe,
  "environment.tree.basic": Tree,
  "environment.rock.basic": Rock,
  "item.wood.basic": Wood,
  "item.stone.basic": Stone,
};
export function Asset({ id }: { id?: AssetId }) {
  const Component = id ? assetRegistry[id] : undefined;
  return Component ? (
    <group dispose={null}>
      <Component />
    </group>
  ) : null;
}
