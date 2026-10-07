import { useRef, type MutableRefObject } from "react";
import { useFrame } from "@react-three/fiber";
import { Html } from "@react-three/drei";
import * as THREE from "three";
import type { Player } from "../../game/types";
import type { Motion } from "./World";
import { locomotionAt } from "./model";
import { Asset } from "./gathering/Assets";
import {
  itemAsset,
  itemName,
  type GatheringController,
} from "./gathering/model";
const gold = "#d4b779";
export default function Explorer({
  motion,
  remote,
  name,
  color,
  gathering,
}: {
  motion?: MutableRefObject<Motion>;
  remote?: Player;
  name: string;
  color: string;
  gathering?: GatheringController;
}) {
  const root = useRef<THREE.Group>(null),
    body = useRef<THREE.Group>(null),
    left = useRef<THREE.Group>(null),
    right = useRef<THREE.Group>(null),
    legL = useRef<THREE.Group>(null),
    legR = useRef<THREE.Group>(null);
  const initialized = useRef(false);
  const heldL = useRef<THREE.Group>(null),
    heldR = useRef<THREE.Group>(null);
  const hands = remote?.hands ?? gathering?.hands;
  useFrame(({ clock }, dt) => {
    const m = motion?.current;
    const p = m?.position ?? remote!.position,
      mode = m?.mode ?? remote?.movementMode ?? "land",
      moving = m?.moving ?? remote?.moving ?? false,
      running = m?.running ?? remote?.running ?? false,
      y = m?.elevation ?? remote?.elevation ?? locomotionAt(p).elevation;
    const swim = mode === "swimming",
      t = clock.elapsedTime * (swim ? 3 : running ? 11 : 7),
      jumpAt = m?.jumpAt ?? remote?.jumpAt ?? 0,
      jt = (Date.now() - jumpAt) / 650,
      jump = !swim && jt >= 0 && jt < 1 ? Math.sin(jt * Math.PI) * 1.2 : 0;
    const target = new THREE.Vector3(p.x, y + jump, p.z);
    if (!initialized.current) {
      root.current!.position.copy(target);
      initialized.current = true;
    } else
      root.current!.position.lerp(target, 1 - Math.exp(-dt * (m ? 24 : 14)));
    const facing = m?.facing ?? remote?.facing ?? 0;
    root.current!.rotation.y +=
      Math.atan2(
        Math.sin(facing - root.current!.rotation.y),
        Math.cos(facing - root.current!.rotation.y),
      ) *
      (1 - Math.exp(-dt * 16));
    body.current!.rotation.x = THREE.MathUtils.damp(
      body.current!.rotation.x,
      swim ? Math.PI * 0.38 : 0,
      10,
      dt,
    );
    body.current!.position.y = swim ? 0.18 : 0;
    left.current!.rotation.x = swim
      ? -1.4 + Math.sin(t) * 0.8
      : moving
        ? Math.sin(t) * 0.65
        : 0;
    right.current!.rotation.x = swim
      ? -1.4 - Math.sin(t) * 0.8
      : moving
        ? -Math.sin(t) * 0.65
        : 0;
    const action = remote ? remote.gatherAction : gathering?.action;
    const actionTime = Date.now() + (gathering?.serverOffset ?? 0);
    if (
      action &&
      !swim &&
      actionTime >= action.startedAt &&
      actionTime < action.endsAt
    ) {
      const progress =
        (actionTime - action.startedAt) / (action.endsAt - action.startedAt);
      const arm = hands?.active === "left" ? left : right;
      arm.current!.rotation.x =
        progress < 0.43
          ? -2.2 * (progress / 0.43)
          : -2.2 + 3.2 * Math.sin(((progress - 0.43) / 0.57) * Math.PI);
    }
    if (heldL.current) heldL.current.visible = !swim;
    if (heldR.current) heldR.current.visible = !swim;
    legL.current!.rotation.x = moving ? Math.sin(t) * (swim ? 0.22 : 0.65) : 0;
    legR.current!.rotation.x = -legL.current!.rotation.x;
  });
  const limb = (ref: typeof left, x: number, arm: boolean) => (
    <group ref={ref} position={[x, arm ? 1.15 : 0.68, 0]}>
      <mesh position={[0, arm ? -0.26 : -0.29, 0]} castShadow>
        <capsuleGeometry args={[arm ? 0.105 : 0.12, arm ? 0.33 : 0.4, 3, 6]} />
        <meshStandardMaterial color={arm ? color : "#384b58"} />
      </mesh>
      {arm && (
        <group ref={x < 0 ? heldL : heldR} position={[0, -0.5, 0.04]}>
          <Asset
            id={itemAsset((x < 0 ? hands?.left : hands?.right) ?? "empty")}
          />
        </group>
      )}
      {arm && (
        <mesh position={[0, -0.5, 0]}>
          <sphereGeometry args={[0.11, 6, 5]} />
          <meshStandardMaterial color="#cba887" />
        </mesh>
      )}
    </group>
  );
  return (
    <group ref={root}>
      <group ref={body}>
        <mesh position={[0, 0.97, 0]} castShadow>
          <capsuleGeometry args={[0.27, 0.42, 4, 8]} />
          <meshStandardMaterial color={color} />
        </mesh>
        <mesh position={[0, 1.52, 0]} castShadow>
          <sphereGeometry args={[0.23, 8, 6]} />
          <meshStandardMaterial color="#cba887" />
        </mesh>
        <mesh position={[0, 1.67, -0.035]}>
          <sphereGeometry args={[0.23, 8, 4, 0, Math.PI * 2, 0, Math.PI / 2]} />
          <meshStandardMaterial color="#383632" />
        </mesh>
        <mesh position={[0, 1.2, -0.26]}>
          <boxGeometry args={[0.38, 0.42, 0.18]} />
          <meshStandardMaterial color="#79694e" />
        </mesh>
        <mesh position={[0, 1.22, 0.21]}>
          <boxGeometry args={[0.4, 0.085, 0.12]} />
          <meshStandardMaterial color={gold} />
        </mesh>
        {limb(left, -0.36, true)}
        {limb(right, 0.36, true)}
        {limb(legL, -0.15, false)}
        {limb(legR, 0.15, false)}
      </group>
      {remote && (
        <Html position={[0, 2.25, 0]} center distanceFactor={20}>
          <span className="ft-name">
            {name}
            {remote.hands
              ? ` · ${itemName(remote.hands[remote.hands.active])}`
              : ""}
            {remote.gatherAction && Date.now() < remote.gatherAction.endsAt
              ? " · Gathering"
              : ""}
            {remote.movementMode === "swimming" ? " · Swimming" : ""}
          </span>
        </Html>
      )}
    </group>
  );
}
