import test from 'node:test';
import assert from 'node:assert/strict';
import { LifeController } from '../src/world/freetower/life/controller.ts';
import { castProblem, fishBehavior } from '../src/world/freetower/fishing/model.ts';
import { cropPhase } from '../src/world/freetower/farming/model.ts';
import { riverCenter } from '../src/world/freetower/model.ts';
const actor={x:0,z:2}, target={x:0,z:riverCenter(0)};
const context=()=>({actor,facing:Math.PI,mode:'land' as const,tool:'basic-fishing-rod',bait:2,buildings:[]});
test('cast requires rod, bait and dry shore',()=>{
 assert.equal(castProblem(target,context()),null);
 assert.match(castProblem(target,{...context(),tool:'basic-axe'})!,/Rod/);
 assert.match(castProblem(target,{...context(),bait:0})!,/Bait/);
 assert.match(castProblem(target,{...context(),mode:'swimming'})!,/land/);
});
test('cast rejects dry ground, range, facing and building obstruction',()=>{
 assert.ok(castProblem({x:0,z:18},context()));
 assert.ok(castProblem({x:30,z:riverCenter(30)},context()));
 assert.ok(castProblem(target,{...context(),facing:0}));
 assert.ok(castProblem(target,{...context(),buildings:[{id:'b',definitionId:'small-shelter',position:{x:0,y:2,z:0},rotationY:0,status:'completed',constructionStartedAt:0,constructionEndsAt:0,ownerId:'u'}]}));
});
test('crop phase is derived from authoritative deadline and watering',()=>{
 const p={id:'p',buildingId:'p',state:'planted' as const,cropId:'wheat',plantedAt:1000,wateredAt:0,readyAt:0,growthModifier:1};
 assert.equal(cropPhase(p,999999),'seed');
 assert.equal(cropPhase({...p,state:'growing',wateredAt:1000,readyAt:46000},10000),'sprout');
 assert.equal(cropPhase({...p,state:'growing',wateredAt:1000,readyAt:46000},35000),'growing');
 assert.equal(cropPhase({...p,state:'ready',wateredAt:1000,readyAt:46000},47000),'ready');
});
test('life retries preserve identity, never mutate personal inventory locally',()=>{
 const packets:any[]=[];const c=new LifeController(m=>packets.push(m));c.ready=true;
 const personal={owner:{kind:'player' as const,roomId:'r',userId:'a'},inventory:{carrot:3},survival:{health:100,hunger:80,thirst:90,energy:100,updatedAt:1}};
 c.personal=structuredClone(personal);c.command('eat',{itemId:'carrot'},1000);c.tick(4100);
 assert.equal(packets.length,2);assert.equal(packets[0].requestId,packets[1].requestId);
 assert.deepEqual(c.personal,personal);c.handle({type:'life-result',requestId:packets[0].requestId,message:'Eaten'});assert.equal(c.pending,null);
});
test('unrelated replies cannot consume pending command and duplicate click is suppressed',()=>{
 const sent:any[]=[];const c=new LifeController(m=>sent.push(m));c.ready=true;
 c.command('harvest',{targetId:'p'},1);c.command('harvest',{targetId:'p'},2);
 c.handle({type:'life-result',requestId:'other',message:'old'});assert.ok(c.pending);assert.equal(sent.length,1);
});
test('fishing server stages map to low-cost visible AI cues',()=>{
 assert.equal(fishBehavior('fish-noticed'),'approach');assert.equal(fishBehavior('nibbling'),'nibble');assert.equal(fishBehavior('escaped'),'flee');
});
test('disconnection clears aiming and reel intent but retains safe request identity',()=>{
 const c=new LifeController(()=>{});c.ready=true;c.aiming=true;c.command('cast',{position:target},1);const id=c.pending?.request.requestId;
 c.disconnect();assert.equal(c.aiming,false);assert.equal(c.pending?.request.requestId,id);
});
test('cast wire carries canonical WaterZone spot and persistence retry retains identity',()=>{
 const sent:any[]=[];const c=new LifeController(m=>sent.push(m));c.ready=true;c.tool='basic-fishing-rod';c.position=actor;c.facing=Math.PI;
 c.personal={owner:{kind:'player',roomId:'r',userId:'a'},inventory:{'basic-bait':2},survival:{health:100,hunger:90,thirst:90,energy:100,updatedAt:0}};
 assert.equal(c.cast(target),true);assert.equal(sent[0].targetId,'silver-river');
 const id=sent[0].requestId;c.handle({type:'life-error',requestId:id,code:'persistence',message:'retry'});assert.equal(c.pending?.request.requestId,id);
});

test('a new cast does not inherit reel intent from the previous catch',()=>{
 const c=new LifeController(()=>{});c.ready=true;c.tool='basic-fishing-rod';c.reelHeld=true;c.startAim();
 assert.equal(c.aiming,true);assert.equal(c.reelHeld,false);
});

test('fishing cue interface emits each authoritative transition once',()=>{
 const c=new LifeController(()=>{});c.selfId='a';const cues:string[]=[];c.onFishingCue=cue=>cues.push(cue);
 const state=(version:number,status:string)=>({version,life:{plots:{},animals:{},fishing:{a:{id:'s',state:status}}}} as any);
 c.applyState(state(1,'waiting'));c.applyState(state(2,'hooked'));c.applyState(state(3,'hooked'));c.applyState(state(4,'caught'));
 assert.deepEqual(cues,['bite','catch']);
});
