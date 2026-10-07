import test from 'node:test';
import assert from 'node:assert/strict';
import { UniSatWalletAdapter, FRACTAL_CHAIN } from '../src/identity/wallet/UniSatWalletAdapter.ts';
import { IdentityAttempt, verifyWallet, publicCharacterName } from '../src/identity/model.ts';
const address='bc1qexample';
function fixture(){
 const calls:any[]=[];let chain='BITCOIN_MAINNET';let accounts=[address];const listeners=new Map<string,Function>();
 const provider={requestAccounts:async()=>accounts,getAccounts:async()=>accounts,getPublicKey:async()=>'02pub',getChain:async()=>({enum:chain}),switchChain:async(c:string)=>{calls.push(['switch',c]);chain=c;return {enum:c}},signMessage:async(m:string,t:string)=>{calls.push(['sign',m,t]);return 'signature'},disconnect:async()=>{accounts=[]},on:(n:string,f:Function)=>listeners.set(n,f),removeListener:(n:string)=>listeners.delete(n)};
 return {adapter:new UniSatWalletAdapter(()=>provider),provider,calls,listeners,setChain:(c:string)=>chain=c,setAccounts:(a:string[])=>accounts=a};
}
test('wallet absent yields recoverable detection error',async()=>{const a=new UniSatWalletAdapter(()=>undefined);assert.equal(a.isAvailable(),false);await assert.rejects(a.connect(),/UniSat/)});
test('connection rejection is recoverable and never switches or signs automatically',async()=>{const f=fixture();f.provider.requestAccounts=async()=>{throw {code:4001}};await assert.rejects(f.adapter.connect(),/reject|cancel/i);assert.deepEqual(f.calls,[])});
test('exact chain enum detects Fractal rather than compatible Bitcoin address format',async()=>{const f=fixture();assert.equal((await f.adapter.connect()).network,'BITCOIN_MAINNET');await f.adapter.ensureFractalNetwork();assert.equal(await f.adapter.getNetwork(),FRACTAL_CHAIN);assert.deepEqual(f.calls,[['switch',FRACTAL_CHAIN]])});
test('successful signature uses official BIP322-simple and exact server message',async()=>{const f=fixture();await f.adapter.signMessage('FreeTower nonce');assert.deepEqual(f.calls,[['sign','FreeTower nonce','bip322-simple']])});
test('empty accounts means locked/disconnected, not a cached address',async()=>{const f=fixture();f.setAccounts([]);await assert.rejects(f.adapter.getAddress(),/locked|disconnect|connect/i)});
test('wallet events subscribe and clean up both account and chain listeners',()=>{const f=fixture();let changes=0;const stop=f.adapter.onChange(()=>changes++);f.listeners.get('accountsChanged')?.([]);f.listeners.get('chainChanged')?.('BITCOIN_MAINNET');assert.equal(changes,2);stop();assert.equal(f.listeners.size,0)});
test('network switching failure cannot masquerade as Fractal',async()=>{const f=fixture();f.provider.switchChain=async()=>({enum:'BITCOIN_MAINNET'});await assert.rejects(f.adapter.ensureFractalNetwork(),/Fractal/)});
test('stale async attempts cannot reenter after disconnect',()=>{const a=new IdentityAttempt();const token=a.begin();a.cancel();assert.throws(()=>a.assert(token),/changed|cancel/i)});
test('authenticate verifies exact chain before requesting any challenge',async()=>{const f=fixture();let requests=0;const api:any={challenge:async()=>{requests++;}};await assert.rejects(verifyWallet(f.adapter,api,new IdentityAttempt()),/Fractal/);assert.equal(requests,0)});
test('account switch during signing cannot establish server session',async()=>{const f=fixture();f.setChain(FRACTAL_CHAIN);let verified=0;f.provider.signMessage=async()=>{f.setAccounts(['different']);return 'sig'};const api:any={challenge:async()=>({nonce:'n',message:'m'}),verify:async()=>{verified++}};await assert.rejects(verifyWallet(f.adapter,api,new IdentityAttempt()),/changed/i);assert.equal(verified,0)});
test('valid wallet signs stored challenge and sends normalized chain identity',async()=>{const f=fixture();f.setChain(FRACTAL_CHAIN);let sent:any;const api:any={challenge:async(q:any)=>{assert.equal(q.network,'fractal-mainnet');return {nonce:'n',message:'server-message'}},verify:async(q:any)=>{sent=q;return {wallet:{address}}}};await verifyWallet(f.adapter,api,new IdentityAttempt());assert.equal(sent.nonce,'n');assert.equal(sent.signature,'signature');assert.equal(sent.address,address);assert.equal(sent.network,'fractal-mainnet')});
test('official identity display cannot be overwritten by username',()=>{assert.equal(publicCharacterName({btcName:'alice.fb',username:'spoof'}),'alice.fb');assert.equal(publicCharacterName({characterIdentityId:'id',username:'spoof'}),'Explorer');assert.equal(publicCharacterName({username:'legacy'}),'legacy')});
test('wallet event while server verification is pending cannot publish the returned session',async()=>{
 const f=fixture();f.setChain(FRACTAL_CHAIN);const attempt=new IdentityAttempt();let resolve!: (value:unknown)=>void;let started!:()=>void;const pending=new Promise(r=>resolve=r);const verifying=new Promise<void>(r=>started=r);
 const stop=f.adapter.onChange(()=>attempt.cancel());const operation=verifyWallet(f.adapter,{challenge:async()=>({nonce:'n',message:'m'}),verify:async()=>{started();return pending;}},attempt);
 await verifying;f.listeners.get('accountsChanged')?.(['B']);resolve({wallet:{address}});await assert.rejects(operation,/changed|cancel/i);stop();
});
