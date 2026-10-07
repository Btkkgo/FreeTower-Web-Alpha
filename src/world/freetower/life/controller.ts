import type {LifeAction,LifeMessage,LifeReply,LifeState} from './types.ts';
import type {PersonalState} from '../survival/model.ts';
import type {ResourceState} from '../gathering/model.ts';
import {activeFishing,castProblem} from '../fishing/model.ts';
import {waterAt,type Point,type MovementMode} from '../model.ts';
export class LifeController {
 send:(m:LifeMessage)=>void;ready=false;selfId='';personal:PersonalState|null=null;world:ResourceState|null=null;
 position:Point={x:0,z:18};facing=0;mode:MovementMode='land';tool='empty';serverOffset=0;
 aiming=false;selectedPlot:string|null=null;selectedAnimal:string|null=null;seed='wheat-seed';bait='basic-bait';inventoryOpen=false;
 pending:{request:LifeMessage;at:number}|null=null;message='';messageUntil=0;reelHeld=false;private serial=0;
 onFishingCue?: (cue:'bite'|'catch'|'escape')=>void;
 networkIn=0;networkOut=0;networkStart=Date.now();
 constructor(send:(m:LifeMessage)=>void){this.send=send;}
 get state():LifeState {return this.world?.life??{plots:{},animals:{},fishing:{}};}
 get session(){return this.state.fishing[this.selfId];}
 get fishing(){return activeFishing(this.session);}
 get active(){return this.aiming||this.fishing;}
 note(s:string){this.message=s;this.messageUntil=Date.now()+4500;}
 applyState(s:ResourceState){
  if(this.world&&s.version<this.world.version)return;
  const previous=this.session;this.world=s;const current=this.session;
  if(current&&(previous?.id!==current.id||previous?.state!==current.state)){
   const cue=current.state==='hooked'?'bite':current.state==='caught'?'catch':current.state==='escaped'?'escape':null;
   if(cue)this.onFishingCue?.(cue);
  }
 }
 command(action:LifeAction,args:Partial<LifeMessage>={},now=Date.now()){
  if(!this.ready||this.pending)return false;
  const request:LifeMessage={...args,type:'life-action',action,requestId:`life-${now.toString(36)}-${++this.serial}-${Math.random().toString(36).slice(2)}`};
  this.pending={request,at:now};this.send(request);return true;
 }
 handle(m:LifeReply){
  if(m.type==='life-self'){this.personal=m.personal;this.serverOffset=m.serverTime-Date.now();return;}
  if(m.requestId!==this.pending?.request.requestId)return;
  if(m.type==='life-error' && ['persistence','rate-limit'].includes(m.code)){this.note(m.message);return;}
  const action=this.pending.request.action;this.pending=null;this.note(m.message);
  if(m.type==='life-result'&&action==='cast')this.aiming=false;
 }
 startAim(){
  if(!this.ready||this.pending||this.fishing)return;
  if(this.tool!=='basic-fishing-rod'){this.note('Equip Fishing Rod');return;}
  if(this.mode!=='land'){this.note('Return to dry land');return;}
  this.reelHeld=false;this.aiming=true;this.selectedPlot=null;this.selectedAnimal=null;this.note('Choose Basic Bait, then click water · 点击水面抛竿');
 }
 cast(p:Point){
  const issue=castProblem(p,{actor:this.position,facing:this.facing,mode:this.mode,tool:this.tool,bait:this.personal?.inventory[this.bait]??0,buildings:Object.values(this.world?.buildings??{})});
  if(issue){this.note(issue);return false;}
  return this.command('cast',{targetId:waterAt(p).id,position:p,itemId:this.bait});
 }
 reel(on:boolean){this.reelHeld=on;}
 cancel(){this.aiming=false;this.reelHeld=false;if(this.fishing)this.command('cancel-fishing');}
 disconnect(){this.ready=false;this.aiming=false;this.reelHeld=false;}
 reset(){this.disconnect();this.personal=null;this.world=null;this.pending=null;this.selectedAnimal=null;this.selectedPlot=null;}
 tick(now=Date.now()){
  if(!this.ready)return;
  if(this.pending&&now-this.pending.at>=3000){this.pending.at=now;this.send(this.pending.request);}
  if(this.mode!=='land'||this.tool!=='basic-fishing-rod') {if(this.active)this.cancel();return;}
  if(this.fishing&&!this.pending&&this.session&&['hooked','reeling'].includes(this.session.state)&&this.session.reeling!==this.reelHeld)this.command('reel',{reeling:this.reelHeld},now);
 }
}
