import {useEffect,useReducer,useRef} from 'react';
import {LifeController} from './controller';
import type {LifeMessage} from './types';
import type {GatheringController} from '../gathering/model';
import type {BuildingController} from '../building/controller';
import {itemName,activeItem,distanceTo} from '../gathering/model';
import {survivalLabels} from '../survival/model';
import './life.css';
const focus=()=>document.querySelector<HTMLCanvasElement>('.ft-world canvas')?.focus({preventScroll:true});
export function useLife(send:(m:LifeMessage)=>void,ready:boolean,roomId?:string){const ref=useRef<LifeController|null>(null),sender=useRef(send);sender.current=send;if(!ref.current)ref.current=new LifeController(m=>sender.current(m));const c=ref.current;useEffect(()=>{c.ready=ready;if(!ready)c.disconnect()},[ready,c]);useEffect(()=>{c.reset();return()=>c.reset()},[roomId,c]);return c;}
export default function LifeHUD({controller:c,gathering:g,building:b,debug}:{controller:LifeController;gathering:GatheringController;building:BuildingController;debug:boolean}){
 const [,refresh]=useReducer(n=>n+1,0);
 const plot=c.state.plots[c.selectedPlot??b.selectedId??''];
 const animal=c.state.animals[c.selectedAnimal??''];
 const plotBuilding=plot?c.world?.buildings?.[plot.buildingId]:null;
 const nearPlot=!!plotBuilding&&plotBuilding.status==='completed'&&distanceTo(c.position,plotBuilding.position)<=3.5;
 const nearAnimal=!!animal&&distanceTo(c.position,animal.position)<=3.5;
 const nearEgg=!!animal&&distanceTo(c.position,animal.eggPosition??animal.position)<=3.5;
 const act=(action:Parameters<LifeController['command']>[0],args:Parameters<LifeController['command']>[1])=>{g.cancelInput();c.command(action,args);focus();refresh()};
 useEffect(()=>{const t=setInterval(()=>{c.tick();refresh()},100);return()=>clearInterval(t)},[c]);
 useEffect(()=>{
  g.useOverride=()=>{
   if(b.active)return false;
   if(activeItem(g.hands)==='basic-fishing-rod'){if(c.fishing)c.reel(!c.reelHeld);else c.startAim();return true;}
   const p=c.state.plots[c.selectedPlot??b.selectedId??''];
   if(p){if(p.state==='empty')c.command('till',{targetId:p.id});else if(p.state==='tilled')c.command('plant',{targetId:p.id,itemId:c.seed});else if(p.state==='planted')c.command('water',{targetId:p.id});else if(p.state==='ready')c.command('harvest',{targetId:p.id});return true;}
   if(c.selectedAnimal){const a=c.state.animals[c.selectedAnimal];if(a)c.command(a.eggReady?'collect-egg':'feed',{targetId:a.id,itemId:'chicken-feed'});return true;}
   return false;
  };
  const down=(e:KeyboardEvent)=>{if((e.target as HTMLElement).closest('input,textarea,[contenteditable]'))return;if(e.code==='KeyI'&&!e.repeat){c.inventoryOpen=!c.inventoryOpen;refresh();}if(e.code==='Escape'){c.cancel();c.inventoryOpen=false;c.selectedAnimal=null;c.selectedPlot=null;b.selectedId=null;focus();}};
  const blur=()=>c.reel(false);
  window.addEventListener('keydown',down);window.addEventListener('blur',blur);
  return()=>{g.useOverride=null;window.removeEventListener('keydown',down);window.removeEventListener('blur',blur)};
 },[c,g,b]);
 const nearbyEgg=Object.values(c.state.animals).find(a=>a.eggReady&&distanceTo(c.position,a.eggPosition??a.position)<=3.5);
 const nearbyChicken=Object.values(c.state.animals).filter(a=>distanceTo(c.position,a.position)<=3.5).sort((a,b)=>distanceTo(c.position,a.position)-distanceTo(c.position,b.position))[0];
 const inv=c.personal?.inventory??{},s=c.personal?.survival,now=Date.now()+c.serverOffset;
 return <>
 <div className="ft-survival" aria-label="Personal survival">{s&&(['health','hunger','thirst','energy'] as const).map((key,i)=><div key={key} title={survivalLabels[key]}><span>{['♥','◐','◒','ϟ'][i]} {key}</span><b>{s[key].toFixed(1)}</b><progress max={100} value={s[key]}/></div>)}</div>
 <button className="ft-pack-button" onClick={()=>{c.inventoryOpen=!c.inventoryOpen;refresh()}}>I · Pack</button>
 {c.inventoryOpen&&<section className="ft-life-panel ft-pack" aria-label="Personal inventory"><header><strong>YOUR PACK · 个人物品</strong><button onClick={()=>{c.inventoryOpen=false;focus()}}>×</button></header><div className="ft-pack-grid">{Object.entries(inv).filter(([,q])=>q>0).map(([id,q])=><div key={id}><span>{itemName(id)}</span><b>{q}</b>{id==='carrot'&&<button disabled={!!c.pending||!c.ready} onClick={()=>act('eat',{itemId:id})}>Eat Carrot</button>}{id==='water-flask'&&<button disabled={!!c.pending||!c.ready} onClick={()=>act('drink',{itemId:id})}>Drink Water</button>}</div>)}</div><small>物品和生存状态属于当前世界中的你的角色。</small></section>}
 {!b.active&&!c.active&&!plot&&!animal&&nearbyEgg&&<button className="ft-life-interact" onClick={()=>{c.selectedAnimal=nearbyEgg.id;focus();refresh()}}>Egg · Interact</button>}
 {!b.active&&!c.active&&!plot&&!animal&&!nearbyEgg&&c.tool!=='basic-fishing-rod'&&nearbyChicken&&<button className="ft-life-interact" onClick={()=>{c.selectedAnimal=nearbyChicken.id;focus();refresh()}}>Chicken · Interact</button>}
 {!b.active&&c.tool==='basic-fishing-rod'&&!c.active&&<button className="ft-life-interact" onClick={()=>{g.cancelInput();c.startAim();focus()}}>E · Fish / 钓鱼</button>}
 {!b.active&&c.aiming&&<section className="ft-life-panel ft-life-action" aria-label="Fishing aim"><strong>FISHING · 选择水面</strong><label>Bait <select aria-label="Fishing bait" value={c.bait} onChange={e=>{c.bait=e.target.value}}><option value="basic-bait">Basic Bait ({inv['basic-bait']??0})</option></select></label><small>面向河流或海岸，点击 2–12 m 内水面抛竿。</small><button onClick={()=>{c.cancel();focus()}}>Cancel fishing</button></section>}
 {c.fishing&&c.session&&<section className="ft-life-panel ft-life-action" aria-label="Fishing session"><strong>{c.session.state.toUpperCase()} · {c.session.spotId==='silver-river'?'River':'Coast'}</strong><span>Distance {c.session.distance.toFixed(1)} m · Tension {Math.round(c.session.tension)}%</span><progress max={100} value={c.session.tension}/><small>{['hooked','reeling'].includes(c.session.state)?'Reel to pull closer; release before tension fills.':'Watch the float · 等待咬钩'}</small><div><button disabled={!['hooked','reeling'].includes(c.session.state)} onClick={()=>{c.reel(!c.reelHeld);focus();refresh()}}>{c.reelHeld?'Release reel':'Reel'}</button><button onClick={()=>{c.cancel();focus()}}>Cancel fishing</button></div></section>}
 {!b.active&&!c.active&&plot&&<section className="ft-life-panel ft-life-action" aria-label="Farm plot actions"><header><strong>FARM PLOT · {plot.state}</strong><button onClick={()=>{c.selectedPlot=null;b.selectedId=null;focus()}}>×</button></header><small>{plot.cropId||'Empty soil'} {plot.readyAt>now?`· ${Math.ceil((plot.readyAt-now)/1000)}s`:''} · {plotBuilding?distanceTo(c.position,plotBuilding.position).toFixed(1):'—'} m</small>{plot.state==='empty'&&<button disabled={!nearPlot||c.tool!=='basic-hoe'||!!c.pending} onClick={()=>act('till',{targetId:plot.id})}>Till · Hoe</button>}{plot.state==='tilled'&&<><select aria-label="Choose seed" value={c.seed} onChange={e=>{c.seed=e.target.value;refresh()}}><option value="wheat-seed">Wheat Seed ({inv['wheat-seed']??0})</option><option value="carrot-seed">Carrot Seed ({inv['carrot-seed']??0})</option></select><button disabled={!nearPlot||!!c.pending||!inv[c.seed]} onClick={()=>act('plant',{targetId:plot.id,itemId:c.seed})}>Plant</button></>}{plot.state==='planted'&&<button disabled={!nearPlot||c.tool!=='basic-watering-can'||!!c.pending} onClick={()=>act('water',{targetId:plot.id})}>Water · Watering Can</button>}{plot.state==='ready'&&<button disabled={!nearPlot||!!c.pending} onClick={()=>act('harvest',{targetId:plot.id})}>Harvest</button>}<small>{!nearPlot?'请靠近至 3.5 m 内':plot.state==='empty'?'Equip Hoe · 装备锄头':plot.state==='planted'?'Equip Watering Can · 装备水壶':'E / 选择操作'}</small><button onClick={()=>{c.selectedPlot=null;b.selectedId=plot.buildingId;b.beginMove();focus()}}>Move Plot</button></section>}
 {!b.active&&!c.active&&animal&&<section className="ft-life-panel ft-life-action" aria-label="Chicken actions"><header><strong>CHICKEN · {animal.state}</strong><button onClick={()=>{c.selectedAnimal=null;focus()}}>×</button></header><small>{animal.id} · {distanceTo(c.position,animal.position).toFixed(1)} m</small>{animal.eggReady?<button disabled={!nearEgg||!!c.pending} onClick={()=>act('collect-egg',{targetId:animal.id})}>Collect Egg</button>:animal.nextEggAt>0?<span>Egg in {Math.max(0,Math.ceil((animal.nextEggAt-now)/1000))}s</span>:<button disabled={!nearAnimal||!!c.pending||!inv['chicken-feed']} onClick={()=>act('feed',{targetId:animal.id,itemId:'chicken-feed'})}>Feed Chicken ({inv['chicken-feed']??0})</button>}<small>Approach within 3.5 m · 靠近互动</small></section>}
 {c.message&&Date.now()<c.messageUntil&&<div className="ft-life-feedback" role="status">{c.message}</div>}
 {debug&&<output className="ft-life-debug">Fishing {c.aiming?'aiming':c.session?.state??'idle'} · {c.session?.spotId??'none'} · {c.session?.fishId??'none'} · Tension {c.session?.tension??0}<br/>Plot {plot?.state??'none'} · Crop {plot?.cropId??'none'} · ReadyAt {plot?.readyAt??0}<br/>Animal {animal?.state??'none'} · Target {animal?`${animal.target.x.toFixed(1)},${animal.target.z.toFixed(1)}`:'none'} · Egg {animal?.nextEggAt??0}<br/>WS in {(c.networkIn/Math.max(1,(Date.now()-c.networkStart)/1000)).toFixed(1)}/s · out {(c.networkOut/Math.max(1,(Date.now()-c.networkStart)/1000)).toFixed(1)}/s</output>}
 </>;
}
