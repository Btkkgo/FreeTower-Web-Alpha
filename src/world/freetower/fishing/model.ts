import config from "../../../../server/freetower_life.json" with {type:"json"};
import { waterAt, type Point, type MovementMode } from '../model.ts';
import type { BuildingInstance } from '../building/types.ts';
import { buildingDefinitions } from '../building/types.ts';
import { circleOverlap } from '../building/buildingMath.ts';
export type FishingState='casting'|'waiting'|'fish-noticed'|'nibbling'|'hooked'|'reeling'|'caught'|'escaped'|'cancelled';
export interface FishDefinition { id:string;displayName:string;habitat:string;rarity:number;biteDelayRange:number[];fightStrength:number;baseValue:number;assetId:string }
export interface FishingSpotDefinition {id:string;habitat:string}
export interface BaitDefinition {id:string;displayName:string;assetId:string}
export interface CatchResult {sessionId:string;fishId:string;quantity:number;userId:string}
export interface FishingSession {id:string;userId:string;spotId:string;fishId:string;state:FishingState;position:Point;origin:Point;startedAt:number;biteAt:number;expiresAt:number;distance:number;tension:number;reeling:boolean;updatedAt:number}
export const activeFishing=(s?:FishingSession)=>!!s&&!['caught','escaped','cancelled'].includes(s.state);
export const fishBehavior=(state:FishingState)=>({casting:'roam',waiting:'notice','fish-noticed':'approach',nibbling:'nibble',hooked:'bite',reeling:'bite',caught:'bite',escaped:'flee',cancelled:'flee'}[state]);
export function castProblem(p:Point, c:{actor:Point;facing:number;mode:MovementMode;tool:string;bait:number;buildings:BuildingInstance[]}):string|null {
 if(c.tool!=='basic-fishing-rod')return 'Equip Fishing Rod';
 if(c.mode!=='land')return 'Return to dry land';
 if(c.bait<1)return 'Basic Bait required';
 if(!Number.isFinite(p.x)||!Number.isFinite(p.z))return 'Choose a water surface';
 const water=waterAt(p);if(!['silver-river','ancient-sea'].includes(water.id))return 'Choose river or coast water';
 const dx=p.x-c.actor.x,dz=p.z-c.actor.z,d=Math.hypot(dx,dz);
 if(d<config.fishing.minRange||d>config.fishing.maxRange)return 'Cast between 2 and 12 m';
 if((Math.sin(c.facing)*dx+Math.cos(c.facing)*dz)/d<Math.cos(config.fishing.facingConeDegrees/2*Math.PI/180))return 'Face the water first';
 for(let i=1;i<=Math.ceil(d/.25);i++) {const t=i/Math.ceil(d/.25),q={x:c.actor.x+dx*t,z:c.actor.z+dz*t};
  for(const b of c.buildings){const def=buildingDefinitions[b.definitionId];if(def&&b.definitionId!=='farm-plot'&&circleOverlap(b.position,b.rotationY,def.footprint,q,.05))return 'Building blocks the cast';}
 }
 return null;
}
