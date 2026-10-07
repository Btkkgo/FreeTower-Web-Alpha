import test from 'node:test';
import assert from 'node:assert/strict';
import { IdentityLifecycle, checkedWalletConnection } from '../src/identity/lifecycle.ts';
import { recoverUnauthorized, reconnectSession } from '../src/identity/recovery.ts';
import { api } from '../src/game/api.ts';
import { FRACTAL_CHAIN } from '../src/identity/wallet/UniSatWalletAdapter.ts';
function deferred<T>() { let resolve!: (value:T)=>void; let reject!: (error:unknown)=>void; const promise=new Promise<T>((a,b)=>{resolve=a;reject=b}); return {promise,resolve,reject}; }
const tick=()=>new Promise<void>(resolve=>setImmediate(resolve));
test('restore cannot publish A after account event while chain read is pending',async()=>{
 const chain=deferred<string>(); const lifecycle=new IdentityLifecycle(async()=>{}); const token=lifecycle.begin(); let published=false;
 const restore=lifecycle.run(token,async()=>{await checkedWalletConnection({getAddress:async()=>'A',getNetwork:()=>chain.promise,getPublicKey:async()=>null},()=>lifecycle.assert(token),'A');published=true;});
 await tick(); const revoked=lifecycle.revoke(); chain.resolve(FRACTAL_CHAIN);
 await assert.rejects(restore,/cancel|changed/i); await revoked; assert.equal(published,false);
});
for(const fails of [false,true]) test(`old ${fails?'rejection':'success'} settles before logout and new B session`,async()=>{
 const pending=deferred<void>(); const logout=deferred<void>(); let cookie='A'; const calls:string[]=[];
 const lifecycle=new IdentityLifecycle(async()=>{calls.push('logout-start');await logout.promise;cookie='';calls.push('logout-end');});
 const a=lifecycle.begin(); const old=lifecycle.run(a,async()=>{calls.push('A');await pending.promise;cookie='stale-A';lifecycle.assert(a);}).catch(()=>{});
 await tick(); const revoke=lifecycle.revoke(); const b=lifecycle.begin(); const newer=lifecycle.run(b,async()=>{cookie='B';calls.push('B');});
 await tick(); assert.deepEqual(calls,['A']); if(fails)pending.reject(new Error('signature rejected'));else pending.resolve(); await old;await tick();
 assert.deepEqual(calls,['A','logout-start']);logout.resolve();await revoke;await newer;assert.equal(cookie,'B');assert.deepEqual(calls,['A','logout-start','logout-end','B']);
});
test('failed logout blocks new authentication until cleanup succeeds',async()=>{
 let available=false;let authenticated=false;const lifecycle=new IdentityLifecycle(async()=>{if(!available)throw new Error('offline');});
 await assert.rejects(lifecycle.revoke(),/offline/);const token=lifecycle.begin();await assert.rejects(lifecycle.run(token,async()=>{authenticated=true;}),/offline/);assert.equal(authenticated,false);
 available=true;await lifecycle.run(lifecycle.begin(),async()=>{authenticated=true;});assert.equal(authenticated,true);
});
test('lobby API preserves 401 and routes to gate recovery',async()=>{
 const original=globalThis.fetch;globalThis.fetch=async()=>new Response(JSON.stringify({error:'expired'}),{status:401});let revoked=0;
 try{await api.rooms().catch(error=>{assert.equal(recoverUnauthorized(error,()=>revoked++),true);});assert.equal(revoked,1);}finally{globalThis.fetch=original;}
});
test('failed websocket handshake probes session and recovers 401',async()=>{assert.equal(await reconnectSession(async()=>{throw {status:401};},'A'),'expired');});
test('transient websocket probe network and 5xx failures preserve retry',async()=>{for(const error of [new Error('offline'),{status:503}])assert.equal(await reconnectSession(async()=>{throw error;},'A'),'retry');});
test('hung reconnect probe is bounded and retains retry',async()=>{assert.equal(await reconnectSession(()=>new Promise(()=>{}),'A',5),'retry');});
test('options/select 401 use the same unauthorized recovery, other errors do not revoke',()=>{let revoked=0;assert.equal(recoverUnauthorized({status:401},()=>revoked++),true);assert.equal(recoverUnauthorized({status:503},()=>revoked++),false);assert.equal(revoked,1);});
test('selection response cannot publish after a chain event',async()=>{
 const selected=deferred<void>();const calls:string[]=[];const lifecycle=new IdentityLifecycle(async()=>{calls.push('logout');});const token=lifecycle.begin();let entered=false;
 const select=lifecycle.run(token,async()=>{await selected.promise;lifecycle.assert(token);entered=true;});await tick();const cleanup=lifecycle.revoke();selected.resolve();await assert.rejects(select,/changed|cancel/i);await cleanup;assert.equal(entered,false);assert.deepEqual(calls,['logout']);
});
test('restore rechecks address to catch a change with missing provider events',async()=>{
 let reads=0;await assert.rejects(checkedWalletConnection({getAddress:async()=>++reads===1?'A':'B',getNetwork:async()=>FRACTAL_CHAIN,getPublicKey:async()=>null},()=>{},'A'),/changed/);
});
test('restore rechecks chain to catch a change with missing provider events',async()=>{
 let reads=0;await assert.rejects(checkedWalletConnection({getAddress:async()=>'A',getNetwork:async()=>++reads===1?FRACTAL_CHAIN:'BITCOIN_MAINNET',getPublicKey:async()=>null},()=>{},'A'),/changed/);
});

for (const [label, session] of [
 ['pending identity', {identity:null,user:null}],
 ['missing identity', {identity:null,user:{id:'A'}}],
 ['missing user', {identity:{id:'A',status:'active'},user:null}],
 ['inactive identity', {identity:{id:'A',status:'ownership-invalid'},user:{id:'A'}}],
 ['different character', {identity:{id:'B',status:'active'},user:{id:'B'}}],
 ['mismatched user', {identity:{id:'A',status:'active'},user:{id:'B'}}],
 ['mismatched identity', {identity:{id:'B',status:'active'},user:{id:'A'}}],
] as const) test(`HTTP 200 reconnect probe with ${label} exits to gate`, async()=>{
 assert.equal(await reconnectSession(async()=>session,'A'),'expired');
});
test('HTTP 200 reconnect probe for the same active character preserves retry',async()=>{
 assert.equal(await reconnectSession(async()=>({identity:{id:'A',status:'active'},user:{id:'A'}}),'A'),'retry');
});
