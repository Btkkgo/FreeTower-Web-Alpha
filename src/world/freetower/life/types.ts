import type {Point} from '../model.ts';
import type {FishingSession} from '../fishing/model.ts';
import type {FarmPlotState} from '../farming/model.ts';
import type {PersonalState} from '../survival/model.ts';
import type {AnimalState} from '../animals/model.ts';
export interface LifeState {plots:Record<string,FarmPlotState>;animals:Record<string,AnimalState>;fishing:Record<string,FishingSession>}
export type LifeAction='cast'|'reel'|'cancel-fishing'|'till'|'plant'|'water'|'harvest'|'feed'|'collect-egg'|'eat'|'drink';
export interface LifeMessage {type:'life-action';requestId:string;action:LifeAction;targetId?:string;itemId?:string;position?:Point;reeling?:boolean}
export type LifeReply={type:'life-result';requestId:string;message:string}|{type:'life-error';requestId:string;code:string;message:string}|{type:'life-self';personal:PersonalState;serverTime:number};
