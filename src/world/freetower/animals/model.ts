import type {Point} from '../model.ts';
export type AnimalAIState='idle'|'wander'|'peck'|'approach-food'|'eat';
export interface AnimalDefinition {id:string;displayName:string;assetId:string;habitatRadius:number;eggSeconds:number}
export interface AnimalState {id:string;definitionId:string;position:Point;home:Point;target:Point;state:AnimalAIState;fedAt:number;nextEggAt:number;eggReady:boolean;eggPosition?:Point;updatedAt:number}
