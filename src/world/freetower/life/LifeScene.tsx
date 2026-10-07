import {useEffect,useReducer,useRef} from 'react';
import {useFrame} from '@react-three/fiber';
import {Html,Line} from '@react-three/drei';
import * as THREE from 'three';
import type {LifeController} from './controller';
import type {BuildingController} from '../building/controller';
import {terrainHeight} from '../model';
import {LifeAsset} from './LifeAssets';
import {cropPhase} from '../farming/model';
import {activeFishing,fishBehavior} from '../fishing/model';
export default function LifeScene({controller:c,building}:{controller:LifeController;building:BuildingController}){
 const [,refresh]=useReducer(n=>n+1,0),animals=useRef(new Map<string,THREE.Group>()),fish=useRef(new Map<string,THREE.Group>()),floats=useRef(new Map<string,THREE.Group>());
 useEffect(()=>{const t=setInterval(refresh,200);return()=>clearInterval(t)},[]);
 useFrame(({clock},dt)=>{
  for(const [id,g] of animals.current){const a=c.state.animals[id];if(!a)continue;g.position.lerp(new THREE.Vector3(a.position.x,terrainHeight(a.position),a.position.z),1-Math.exp(-dt*5));if(a.state==='wander'||a.state==='approach-food')g.rotation.y=Math.atan2(a.target.x-a.position.x,a.target.z-a.position.z);g.rotation.x=a.state==='peck'||a.state==='eat'?Math.sin(clock.elapsedTime*5)*.15:0;}
  for(const [id,g] of fish.current){const s=c.state.fishing[id];if(!s)continue;const t=clock.elapsedTime,d=s.state==='waiting'?2:s.state==='fish-noticed'?1:.35;g.position.set(s.position.x+Math.sin(t*2)*d,-.08,s.position.z+Math.cos(t*2)*d);g.rotation.y=t*2;}
  for(const [id,g] of floats.current){const s=c.state.fishing[id];if(s)g.position.y=['hooked','reeling','nibbling'].includes(s.state)?Math.sin(clock.elapsedTime*14)*.12:.06+Math.sin(clock.elapsedTime*3)*.025;}
 });
 const now=Date.now()+c.serverOffset;
 return <group>
 {Object.values(c.state.plots).map(p=>{const b=c.world?.buildings?.[p.buildingId];if(!b||b.status==='constructing')return null;return <group key={p.id} position={[b.position.x,b.position.y+.05,b.position.z]} rotation={[0,b.rotationY,0]} onPointerDown={e=>{if(e.button!==0||building.active)return;e.stopPropagation();c.selectedPlot=p.id;c.selectedAnimal=null;building.selectedId=null;}}>
 <mesh rotation={[-Math.PI/2,0,0]} position={[0,.08,0]}><planeGeometry args={[2.75,2.75]}/><meshStandardMaterial color={p.wateredAt?'#40372b':p.state==='empty'?'#806549':'#554031'}/></mesh>
 {p.cropId&&<LifeAsset id={`crop.${p.cropId}.stage.${cropPhase(p,now)}`}/>}
 {(c.selectedPlot===p.id||p.state==='ready')&&<Html center position={[0,1.4,0]} style={{pointerEvents:'none'}}><span className="ft-life-label">{p.cropId||'Farm Plot'} · {p.state}{p.readyAt>now?` · ${Math.ceil((p.readyAt-now)/1000)}s`:''}</span></Html>}
 </group>})}
 {Object.values(c.state.animals).map(a=><group key={a.id}>
 <group ref={g=>{if(g)animals.current.set(a.id,g);else animals.current.delete(a.id)}} position={[a.position.x,terrainHeight(a.position),a.position.z]} onPointerDown={e=>{if(e.button!==0||building.active)return;e.stopPropagation();c.selectedAnimal=a.id;c.selectedPlot=null;building.selectedId=null;}}><LifeAsset id="animal.chicken.basic"/>{(c.selectedAnimal===a.id||a.nextEggAt>now)&&<Html center position={[0,1.4,0]} style={{pointerEvents:'none'}}><span className="ft-life-label">Chicken · {a.state}{a.nextEggAt>now?` · Egg ${Math.ceil((a.nextEggAt-now)/1000)}s`:''}</span></Html>}</group>
 {a.eggReady&&<group position={[(a.eggPosition??a.position).x,terrainHeight(a.eggPosition??a.position)+.06,(a.eggPosition??a.position).z]} onPointerDown={e=>{if(e.button!==0||building.active)return;e.stopPropagation();c.selectedAnimal=a.id;c.selectedPlot=null;}}><LifeAsset id="item.egg.basic"/><Html center position={[0,.65,0]} style={{pointerEvents:'none'}}><span className="ft-life-label">Egg · Collect</span></Html></group>}
 </group>)}
 {Object.entries(c.state.fishing).map(([id,s])=>{if(!activeFishing(s))return s.state==='caught'?<Html key={s.id} position={[s.origin.x,terrainHeight(s.origin)+2.5,s.origin.z]} center style={{pointerEvents:'none'}}><span className="ft-life-label">Caught! · {s.fishId}</span></Html>:null;return <group key={s.id}>
 <Line points={[[s.origin.x,terrainHeight(s.origin)+2.1,s.origin.z],[s.position.x,.13,s.position.z]]} color="#f0e7bb" lineWidth={1.5}/>
 <group ref={g=>{if(g)floats.current.set(id,g);else floats.current.delete(id)}} position={[s.position.x,.1,s.position.z]}><mesh><sphereGeometry args={[.11,8,6]}/><meshStandardMaterial color="#e3c272"/></mesh><mesh position={[0,.13,0]}><cylinderGeometry args={[.03,.03,.25,5]}/><meshStandardMaterial color="#d7604f"/></mesh></group>
 <group ref={g=>{if(g)fish.current.set(id,g);else fish.current.delete(id)}}><LifeAsset id={`fish.${s.fishId}`}/></group>
 <Html position={[s.position.x,.6,s.position.z]} center style={{pointerEvents:'none'}}><span className="ft-life-label">{s.state} · {fishBehavior(s.state)}</span></Html>
 </group>})}
 </group>;
}
