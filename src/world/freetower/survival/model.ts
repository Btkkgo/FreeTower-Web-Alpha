export interface SurvivalState {health:number;hunger:number;thirst:number;energy:number;updatedAt:number}
export type InventoryOwner={kind:'player';roomId:string;userId:string}|{kind:'world';roomId:string};
export interface PersonalState {owner:Extract<InventoryOwner,{kind:'player'}>;inventory:Record<string,number>;survival:SurvivalState}
export const survivalLabels={health:'Health · 生命',hunger:'Hunger · 饱食',thirst:'Thirst · 饮水',energy:'Energy · 精力'};
