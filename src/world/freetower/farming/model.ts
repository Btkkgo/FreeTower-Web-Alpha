export type FarmPlotStatus='empty'|'tilled'|'planted'|'watered'|'growing'|'ready';
export interface CropDefinition {id:string;displayName:string;seedId:string;itemId:string;growSeconds:number;yield:number;assetId:string}
export interface FarmPlotState {id:string;buildingId:string;state:FarmPlotStatus;cropId:string;plantedAt:number;readyAt:number;wateredAt:number;growthModifier:number}
export type CropState='seed'|'sprout'|'growing'|'ready';
export function cropPhase(p:FarmPlotState,now:number):CropState {
 if(p.state==='ready')return 'ready';
 if(!p.wateredAt||!p.readyAt)return 'seed';
 const progress=(now-p.wateredAt)/Math.max(1,p.readyAt-p.wateredAt);
 return progress<.12?'seed':progress<.5?'sprout':'growing';
}
