import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import FreeTowerApp from '../world/freetower/FreeTowerApp';
import { identityAPI, IdentityAPIError } from './api';
import { IdentityAttempt, verifyWallet } from './model';
import { IdentityLifecycle, checkedWalletConnection } from './lifecycle';
import { recoverUnauthorized } from './recovery';
import { FRACTAL_CHAIN, UniSatWalletAdapter } from './wallet/UniSatWalletAdapter';
import type { WalletAdapter, WalletConnection } from './wallet/types';
import type { IdentityConfig, IdentityOptions, IdentitySession } from './types';
import './identity.css';
const LegacyApp = lazy(() => import('../App'));
const query = new URLSearchParams(location.search);
const requestedDev = import.meta.env.DEV && query.get('identityDev') === '1';
const requestedLegacy = import.meta.env.DEV && query.get('legacyAuth') === '1';
const short = (value: string) => value.length > 18 ? `${value.slice(0,8)}…${value.slice(-6)}` : value;
export default function IdentityGate() {
  const [config,setConfig] = useState<IdentityConfig | null>(null);
  const [connection,setConnection] = useState<WalletConnection | null>(null);
  const [session,setSession] = useState<IdentitySession | null>(null);
  const [options,setOptions] = useState<IdentityOptions | null>(null);
  const [asset,setAsset] = useState('');
  const [name,setName] = useState('');
  const [busy,setBusy] = useState(true);
  const [error,setError] = useState('');
  const [reload,setReload] = useState(0);
  const [persona,setPersona] = useState<'alice'|'bob'>('alice');
  const adapter = useRef<WalletAdapter>(new UniSatWalletAdapter());
  const attempt = useRef(new IdentityAttempt());
  const lifecycle = useRef(new IdentityLifecycle(identityAPI.logout));
  const mounted = useRef(true);
  const watched = useRef<string | null>(null);
  const sessionRef = useRef<IdentitySession | null>(null);
  sessionRef.current = session;
  const devAllowed = !!(requestedDev && config?.development && config.devEnabled);
  const current = (token: number) => mounted.current && lifecycle.current.current(token);
  const reset = (reason='') => {
    lifecycle.current.cancel(); attempt.current.cancel(); watched.current=null;
    sessionRef.current=null;
    setSession(null);setConnection(null);setOptions(null);setAsset('');setName('');setError(reason);
  };
  const revoke = (reason: string) => {
    reset(reason);setBusy(true);
    const cleanup=lifecycle.current.revoke();
    const token=lifecycle.current.snapshot();
    void cleanup.catch(()=>{if(current(token))setError('Disconnected locally. Server sign-out unavailable; reconnect will retry sign-out first.');})
      .finally(()=>{if(current(token))setBusy(false);});
  };
  const loadOptions = async (token: number) => {
    const data=await identityAPI.options();
    if(!current(token))return;
    setOptions(data);setAsset(data.assets[0]?.assetId||data.assets[0]?.inscriptionId||'');setName(data.names[0]?.name||'');
  };
  const run = async (work:(token:number)=>Promise<void>) => {
    const token=lifecycle.current.begin();setBusy(true);setError('');
    try { await lifecycle.current.run(token,()=>work(token)); }
    catch(e){if(current(token)&&!recoverUnauthorized(e,()=>revoke('Session expired or revoked. Authenticate again.')))setError(e instanceof Error?e.message:'Request failed. Retry.');}
    finally{if(current(token))setBusy(false);}
  };
  useEffect(()=>{
    mounted.current=true;const token=lifecycle.current.begin();setBusy(true);setError('');
    void lifecycle.current.run(token,async()=>{
      const cfg=await identityAPI.config();if(!current(token))return;setConfig(cfg);
      let existing:IdentitySession;
      try{existing=await identityAPI.me();}catch(e){if(e instanceof IdentityAPIError&&e.status===401)return;throw e;}
      if(!current(token))return;
      if(existing.dev){
        if(!(requestedDev&&cfg.development&&cfg.devEnabled)){revoke('Connect your Fractal wallet to continue.');return;}
      }else{
        try{
          const connection=await checkedWalletConnection(adapter.current,()=>lifecycle.current.assert(token),existing.wallet.address);
          if(!current(token))return;
          watched.current=connection.address;setConnection(connection);
        }catch{if(current(token))revoke('Connect your Fractal wallet again to continue.');return;}
      }
      if(!current(token))return;setSession(existing);
      if(!existing.user||existing.identity?.status!=='active')await loadOptions(token);
    }).catch(e=>{if(current(token)&&!recoverUnauthorized(e,()=>revoke('Session expired or revoked. Authenticate again.')))setError(e instanceof Error?e.message:'Identity service unavailable.');})
    .finally(()=>{if(current(token))setBusy(false);});
    return()=>{mounted.current=false;lifecycle.current.cancel();attempt.current.cancel();};
  },[reload]);
  useEffect(()=>{
    // Events invalidate pending restore/authentication even before watched exists.
    const stop=adapter.current.onChange(()=>revoke('Wallet account or network changed. Connect again to check the current wallet.'));
    let polling=false;
    const timer=setInterval(()=>{
      const address=watched.current,token=lifecycle.current.snapshot();if(!address||polling||sessionRef.current?.dev)return;polling=true;
      void Promise.all([adapter.current.getAddress(),adapter.current.getNetwork()])
        .then(([addressNow,network])=>{if(current(token)&&watched.current===address&&(addressNow!==address||network!==FRACTAL_CHAIN))revoke('Wallet disconnected or changed. Authenticate again.');})
        .catch(()=>{if(current(token)&&watched.current===address)revoke('Wallet unavailable. Reconnect to continue.');}).finally(()=>{polling=false;});
    },3000);
    return()=>{stop();clearInterval(timer);};
  },[reload]);
  const disconnect = async () => {
    const wasDev=sessionRef.current?.dev;
    reset();setBusy(true);
    const cleanup=lifecycle.current.revoke(),token=lifecycle.current.snapshot();
    try{await cleanup;if(current(token)&&!wasDev&&adapter.current.isAvailable())await adapter.current.disconnect();}
    catch{if(current(token))setError('Sign-out could not finish. Reconnect will retry sign-out first.');}
    finally{if(current(token))setBusy(false);}
  };
  const connect = () => run(async token=>{
    await adapter.current.connect();
    const connection=await checkedWalletConnection(adapter.current,()=>lifecycle.current.assert(token));
    if(current(token))setConnection(connection);
  });
  const switchNetwork = () => run(async token=>{
    await adapter.current.ensureFractalNetwork();
    const connection=await checkedWalletConnection(adapter.current,()=>lifecycle.current.assert(token));
    if(current(token))setConnection(connection);
  });
  const authenticate = () => run(async token=>{
    if(!connection)return;
    try { await checkedWalletConnection(adapter.current,()=>lifecycle.current.assert(token),connection.address); }
    catch { if(current(token))revoke('Wallet changed. Authenticate again.');return; }
    watched.current=connection.address;
    const data=await verifyWallet(adapter.current,identityAPI,attempt.current);
    if(!current(token))return;
    try { await checkedWalletConnection(adapter.current,()=>lifecycle.current.assert(token),connection.address); }
    catch { if(current(token))revoke('Wallet changed. Authenticate again.');return; }
    if(!current(token))return;
    setSession(data);await loadOptions(token);
  });
  const devConnect = () => run(async token=>{
    if(!devAllowed)throw new Error('DEV identity is disabled.');
    const {MockWalletAdapter}=await import('./wallet/MockWalletAdapter');
    const mock=new MockWalletAdapter(persona);await mock.connect();
    if(!current(token))return;
    const data=await identityAPI.dev(persona);if(!current(token))return;
    setSession(data);await loadOptions(token);
  });
  const select = () => run(async token=>{
    if(!session||!asset||!name)return;
    if(!session.dev){
      try{await checkedWalletConnection(adapter.current,()=>lifecycle.current.assert(token),session.wallet.address);}
      catch{if(current(token))revoke('Wallet changed. Authenticate again.');return;}
    }
    if(!current(token))return;
    const selected=await identityAPI.select({characterAssetId:asset,btcName:name});
    if(!current(token))return;
    if(!session.dev){
      try{await checkedWalletConnection(adapter.current,()=>lifecycle.current.assert(token),session.wallet.address);}
      catch{if(current(token))revoke('Wallet changed. Authenticate again.');return;}
    }
    if(current(token))setSession({...session,...selected});
  });
  if(requestedLegacy&&config?.development&&config.legacyEnabled){
    return <><div className="ft-identity-mode">DEV · LEGACY MIGRATION</div>{query.get('experience')==='little-flock'?<Suspense fallback={<p>Loading…</p>}><LegacyApp/></Suspense>:<FreeTowerApp/>}</>;
  }
  if(session?.user&&session.identity?.status==='active'){
    return <><div className="ft-identity-mode">{session.dev?'DEV ACCEPTANCE · ':''}{session.identity.btcName}<button onClick={()=>void disconnect()}>Disconnect</button></div><FreeTowerApp key={session.user.id} identityUser={session.user} onIdentityDisconnect={disconnect} onIdentityRevoked={()=>revoke('Session expired or revoked. Authenticate again.')}/></>;
  }
  return <main className="ft-identity-entry"><section className="ft-identity-card">
    <p className="ft-identity-eyebrow">A WORLD BEFORE CIVILIZATION</p><h1>FREE<span>TOWER</span></h1>
    <p className="ft-identity-intro">你的角色，你的名字。<br/>A wallet. A character. Your .fb.</p>
    {devAllowed&&<p className="ft-dev-banner">DEV ACCEPTANCE · 模拟身份，不代表链上验证</p>}
    <ol className="ft-identity-steps"><li className={session?'done':''}>{session?'✓':'1'} Wallet verified</li><li className={asset?'done':''}>2 Character asset</li><li className={name?'done':''}>3 .fb name</li></ol>
    {!config?<button disabled={busy} onClick={()=>setReload(n=>n+1)}>{busy?'Connecting…':'Retry service connection'}</button>:!session?<>
      {!connection?<button className="ft-identity-primary" disabled={busy} onClick={()=>void connect()}>{busy?'Waiting for wallet…':'Connect Wallet · UniSat'}</button>:<>
        <p className="ft-wallet-summary">UniSat · {short(connection.address)}</p>
        {connection.network!==FRACTAL_CHAIN?<><p role="status">Fractal Bitcoin Mainnet required. Current chain: {connection.network}</p><button disabled={busy} onClick={()=>void switchNetwork()}>Switch to Fractal Mainnet</button><button disabled={busy} onClick={()=>void connect()}>Retry network check</button></>:<><p className="ft-verified">Fractal Bitcoin Mainnet ✓</p><button className="ft-identity-primary" disabled={busy} onClick={()=>void authenticate()}>{busy?'Verify in UniSat…':'Sign in with wallet'}</button><small>Sign a one-time FreeTower message. No transaction or spending approval.</small></>}
        <button className="ft-identity-link" disabled={busy} onClick={()=>void disconnect()}>Disconnect</button>
      </>}
      {devAllowed&&<div className="ft-dev-controls"><label>DEV persona<select value={persona} onChange={e=>setPersona(e.target.value as 'alice'|'bob')}><option value="alice">alice.fb</option><option value="bob">bob.fb</option></select></label><button disabled={busy} onClick={()=>void devConnect()}>Use DEV identity</button></div>}
    </>:<>
      <p className="ft-verified">Wallet verified ✓ {session.dev?'· DEV':''}</p>
      {!options?<p>Loading eligible assets and names…</p>:<>
        <h2>Choose your character</h2>
        {options.assets.length===0?<p role="status">No eligible Character asset found</p>:<div className="ft-character-options">{options.assets.map(a=>{const id=a.assetId||a.inscriptionId;return <button key={id} className={asset===id?'selected':''} aria-pressed={asset===id} onClick={()=>setAsset(id)} disabled={busy}><span className="ft-character-preview" aria-hidden="true">♙</span><strong>Character</strong><small>{short(a.inscriptionId)}</small><small>{a.status||'eligible'} · Placeholder preview</small></button>})}</div>}
        <h2>Your public name</h2>
        {options.names.length===0?<p role="status">No eligible .fb name found</p>:<label className="ft-name-select">Owned .fb<select value={name} onChange={e=>setName(e.target.value)} disabled={busy}>{options.names.map(n=><option key={n.name} value={n.name}>{n.name}</option>)}</select></label>}
        <button className="ft-identity-primary" disabled={busy||!asset||!name} onClick={()=>void select()}>Enter FreeTower</button>
      </>}
      <div className="ft-identity-actions"><button disabled={busy} onClick={()=>void run(loadOptions)}>Retry verification</button><button disabled={busy} onClick={()=>void disconnect()}>Disconnect</button></div>
    </>}
    {error&&<div className="ft-identity-error" role="alert">{error}<button disabled={busy} onClick={()=>{setError('');if(!config)setReload(n=>n+1);}}>Dismiss</button></div>}
    <footer>Fractal character ownership + verified .fb identity.<br/>角色与名称由服务端验证。</footer>
  </section></main>;
}
