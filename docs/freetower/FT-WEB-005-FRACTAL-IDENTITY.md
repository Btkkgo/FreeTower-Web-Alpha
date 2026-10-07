# FT-WEB-005 — Fractal Identity Implementation Plan

> For agentic workers: use subagent-driven-development with one server implementer and root-owned frontend, followed by independent security review. The user execution spec authorizes implementation and one final exact-scope commit/push, not deployment or transactions.

Goal: replace default password entry with server-verified Fractal wallet + eligible Ordinal + owned .btc identity.
Architecture: wallet adapter in frontend; Go challenge/signature/session authority; server-only Ordinals/BNRP adapters; separate wallet/character persistence mapped by character ID to existing room membership. Existing React/Go stack; BIP-322 verifier may add reviewed Go crypto dependencies, no frontend wallet SDK required.
Spec: /Users/btkkgo/.codex/attachments/0e342c81-e61e-4c52-afa3-8d9186c3448d/已粘贴的文本.txt
Baseline: a93ef27e3e94554f352972b534c7a83c7621543e
Branch: codex/ft-web-005-fractal-identity

## Global constraints
Only FreeTower-Web. Preserve legacy users/rooms/buildings/life inventories. No seed/private-key access, transfers, PSBT signing, minting, purchases, deployment or FT006. User identity signatures only. Fail closed on missing provider/config/ownership. No client/API-key exposure, no raw signatures/tokens in logs. One final commit `feat: add FreeTower Fractal identity gate`, push named branch, no main merge.
Default production mode; mocks require explicit development AND enable flag, rendered DEV. Legacy auth only explicit development/migration fallback. Existing tests may explicitly enable legacy fixture mode; do not weaken defaults to pass tests.

## Verified API audit (2026-10-07, before implementation)
Official UniSat repository wallet-api/api-docs: requestAccounts() returns string[]; getAccounts() returns current authorized address array; getPublicKey() returns original (untweaked for P2TR) key; getChain() returns {enum,name,network}; switchChain('FRACTAL_BITCOIN_MAINNET') switches exact chain. Deprecated getNetwork() only describes address format and cannot establish Fractal identity. Events accountsChanged and chainChanged must invalidate local authenticated identity. disconnect() documented.
Official sign-message.md: signMessage(message,'bip322-simple') returns Base64 witness stack. Prefer BIP-322 simple for native SegWit/Taproot; reject unsupported types explicitly. Verify actual Bitcoin script/witness using established Go Bitcoin primitives and official BIP-322 vectors; never invent a signature format.
Sources:
- https://github.com/unisat-wallet/unisat-dev-docs/blob/master/wallet-api/api-docs/manage-networks.md
- https://github.com/unisat-wallet/unisat-dev-docs/blob/master/wallet-api/api-docs/access-accounts.md
- https://github.com/unisat-wallet/unisat-dev-docs/blob/master/wallet-api/api-docs/connect-extension.md
- https://github.com/unisat-wallet/unisat-dev-docs/blob/master/wallet-api/api-docs/sign-message.md
- https://docs.unisat.io/misc/knowledge-base/fractal-links
- https://open-api-fractal.unisat.io/
- https://github.com/unisat-wallet/unisat-dev-docs/blob/master/open-api/auto-generated/docs/inscription-indexer.md
UniSat Fractal base https://open-api-fractal.unisat.io; GET /v1/indexer/address/{address}/inscription-data?cursor=0&size=100 returns code/data.inscription[],total,cursor; GET /v1/indexer/inscription/info/{id} returns code/data.address,inscriptionId,contentType,metadata,height. Check code/status, pagination, owner, confirmed height; server Bearer key env only. Eligibility is exact configured inscription allowlist (no collection-trust guess); empty production list denies.
BNRP current published API base https://api.bnrp.name: GET /v1/reverse/{address}, GET /v1/resolve/{name}. Official https://bnrp.name/ and https://github.com/nlgal/bnrp-spec/blob/main/BNRPs/BNRP-IP-06.md. Resolve has name,owner,inscription_id,addresses,via,resolved_at. Reverse can be primary-only; adapter returns a list interface, never claims discovery of every owned name if provider returns one. Verify reverse result plus forward owner == canonical wallet address, exact .btc suffix. BNRP names are Bitcoin-anchored, separate from Fractal character Ordinals; record provider/network provenance, do not claim Fractal-native domain ownership. Public live probe: curl GET /v1/resolve/satoshi.btc returned HTTP200 and the documented name/owner/inscription_id/addresses/via/resolved_at keys. This confirms the public endpoint/schema only, not ownership of the test wallet. UniSat Fractal blockchain-info without a configured API key returned HTTP403/code -2004. No real key has been provided and no mock fallback is allowed.

## Shared contracts
JSON network identity `fractal-mainnet`; wallet chain enum `FRACTAL_BITCOIN_MAINNET`.
GET /api/identity/config -> {network,signatureScheme,development,devEnabled,legacyEnabled}.
POST /api/auth/wallet/challenge {address,publicKey?,network} -> {nonce,message,issuedAt,expiresAt,signatureScheme}. Message exact stored server-side includes FreeTower, configured origin/domain,address,network,nonce,timestamps; TTL300s secure32byte nonce, origin cookie/session binding, bounded cache, consume atomically with session creation. Do not trust request Host for production origin; env FT_PUBLIC_ORIGIN required for production auth, explicit loopback origins allowed in development.
POST /api/auth/wallet/verify {address,publicKey?,network,nonce,signature} -> same shape as /api/identity/me. New wallet session is pending until fresh eligibility/ownership selection. GET /api/identity/me -> {wallet:{id,address,network,provider},identity:CharacterIdentity|null,user:User|null,dev:boolean}; unauth401.
GET /api/identity/options -> {assets:CharacterAsset[],names:BtcNameRecord[],dev:boolean}; errors {error,code?}, non2xx. No assets/names means denied with precise UI. POST /api/identity/select {characterAssetId,btcName} -> {identity,user,dev}; fresh provider checks, no client owner trust. POST /api/auth/wallet/logout {} -> {ok:true}, invalidate cookie and existing sockets. POST /api/auth/wallet/dev {persona:'alice'|'bob'} -> same as me; explicit dev/mock flags and loopback only, separate route, never accepted by real verify.
CharacterAsset {inscriptionId,owner,contentType,assetId,previewUrl?,metadata?,collection?,status:'eligible'}; preview fallback original procedural card, never execute inscription HTML/SVG. BtcNameRecord {name,owner,provider,network?}. CharacterIdentity {id,walletAccountId,characterAssetId,btcName,createdAt,lastVerifiedAt,status,provider?,dev?}; own API may expose walletAccountId, public peers must not.
User extends existing with optional {characterIdentityId,btcName,characterAssetId,identityMode:'wallet'|'dev'|'legacy'}; id=characterIdentityId, username=btcName only as compatibility alias for existing engine. WS player/chat carry characterIdentityId,btcName,characterAssetId and never wallet address/account. Display selects btcName, no name input. Room owner/membership and personal inventory keyed immutable character ID; no automatic legacy mapping.
WalletAdapter includes id/name/isAvailable/connect/getAddress/getPublicKey/getNetwork/ensureFractalNetwork/signMessage/disconnect plus onChange invalidation subscription. UI asks connection, explicit network switch, then signature as separate steps. Any account/chain change exits world and logs out; stale async results cannot revive old identity. Refresh must check wallet still connected with matching address/chain before accepting server session. Dev mock adapter only dynamic-imported behind import.meta.env.DEV + explicit query + server config.

## Review focus
1. Replay/concurrent verify/save failure must never duplicate session or reuse consumed challenge; origin binding cannot be spoofed by body/Host.
2. Production cannot enter through legacy cookies, mock routes/query params, stale verified identity or direct room/WS endpoints.
3. Asset/domain transfer on new login invalidates old identity and prevents stale active sockets; outages deny entry without misreporting valid ownership.
4. Account/chain changes while provider/signature requests are pending must not reenter; logout closes already-upgraded sockets.
5. Provider owner/normalization mismatch, incomplete pagination, unsafe content/URLs or untrusted resolver routed address never count as verified ownership.

### Task 1: Server identity, providers, persistence and authority
Ownership: all server files including tests/config/example env; no frontend. Read user spec and audit above through this brief. No subagents, commits, UI, real wallet access, secret logs or transactions.
- [x] RED tests for requested wallet auth1–12, Character13–17, names18–24, Identity25–30; add concurrency/replay/failure rollback/origin/gate/socket invalidation/privacy and PostgreSQL persistence cases.
- [x] Implement focused identity config/models/store/provider/signature/auth modules. Use separate wallet accounts/identities/sessions persistent structures/tables; keep legacy tables/data intact. Unique canonical (network,address), preserve base58 case; lowercase valid bech32 only; reject malformed/mixedcase. Session ID cryptographically random HttpOnly/Lax/Secure prod; network persisted. Production allowed origins configured explicitly.
- [x] Official BIP322 simple verification for P2WPKH/P2TR, established btcd script primitives if no suitable compact verified package; browse official BIP/test vectors first, verify license/version, record dependency impact. No bespoke crypto.
- [x] Provider interfaces and concrete UniSatFractalOrdinalsProvider/BNRP resolver, timeouts, limited response size, fixed configured HTTPS endpoints, bounded pagination, sanitized errors. Allowlist IDs env; no auto-mock on provider failure. BNRP reverse/forward validates actual owner, not routing target. Mock provider explicit development+flag, synthetic assets/names clearly DEV.
- [x] Integrate room/WS gates and existing session APIs; public payload identity fields; on new auth invalidate old wallet sessions, identity fresh verification mandatory; save invalid status on definitive transfer. Legacy test fixtures explicit development. Disconnect/logout closes sockets and invalidates sessions.
- [x] go test -race ./..., go vet, safe existing PG test DB disposable schemas; write report with changed files, exact contracts/config/API validation, tests including counts/skips, limitations and dependency/license results.

### Task 2: Wallet adapter, identity gate and world integration (root)
Ownership: src/identity/{wallet,types,api,IdentityGate,identity.css}, tests/identity*.test.ts; src/game/types.ts; FreeTowerApp/World labels/Chat UI and legacy route guard as required. No server changes while Task1 running.
- [x] RED adapter/state tests: detect unavailable, connect reject/retry, correct exact chain, switch/reject, sign bip322, account/network invalidation, stale async work guard, no custom names, mocks DEV only.
- [x] Implement adapter-only window.unisat access; fresh chain/account check around signature; no SDK dependency or keys. Recovery UI concise Connect→network→verify→asset→.btc→Enter; default password hidden, production URL cannot reveal legacy bypass.
- [x] Integrate gate with existing lobby/world and remember rooms per character; name labels/chat/online use btcName. Logout/reset socket and in-flight work; clear prior character state. DEV badge visible in gate and world. Original procedural asset preview fallback, no remote HTML.
- [x] Frontend tests/build; maintain all FT004 gameplay paths; no world feature expansion.

### Task 3: Independent review, real/DEV browser acceptance and delivery
- [x] Task1/Task2 independent security/spec review and whole branch review; resolve findings with scoped re-review.
- [ ] NEEDS_VALIDATION — Actual UniSat browser check if available; reject connection/signature recoverable, chain detection and signed session if user wallet accessible. Do not guess asset eligibility or fabricate PASS if missing key/allowlist/names. Clearly distinguish real wallet/real providers/DEV acceptance.
- [x] Two DEV identities alice.btc and bob.btc in same room, labels/chat/online; regression gathering/build/fishing/farming/survival/animals and persistence. Preserve old test data. Browser screenshot/console evidence external.
- [x] Final tests/build/race/vet/PG, credential-free source, exact-stage commit/push, verify remote SHA/clean. Strict55-field final report; stop FT006.


## Runtime configuration and identity boundary
The Go server reads environment variables directly; it does not automatically load `.env`. See `server/identity.env.example`. Production requires an HTTPS `FT_PUBLIC_ORIGIN`, a server-only `UNISAT_FRACTAL_API_KEY`, and `FT_CHARACTER_INSCRIPTION_ALLOWLIST` containing exact approved inscription IDs. No real Character collection/ID was supplied for this milestone: production eligibility remains empty and denies entry. Configure additional explicitly trusted origins through `FT_ALLOWED_ORIGINS` only when needed.

Local DEV acceptance uses `FT_ENV=development FT_ENABLE_DEV_IDENTITY=true FT_ENABLE_LEGACY_AUTH=false FT_PUBLIC_ORIGIN=http://localhost:4318 FT_ALLOWED_ORIGINS=http://127.0.0.1:4318 go run .` from `server`, plus Vite development pages at `?identityDev=1&debug=1`. Mocks require all three: development build, explicit query, and development server enablement. The real wallet route never accepts DEV signatures or auto-falls back to mocks. Existing legacy accounts remain stored; neither wallet address nor .btc is inferred from a legacy username. Legacy migration UI/server access requires a separate explicit development flag.

Selected CharacterIdentity.id is the immutable player/member/inventory key used by the existing world engine. Display names come from btcName; username is only an internal compatibility alias. Choosing another Character creates a separate identity and does not inherit the old character's room membership or personal inventory. A real wallet address is returned only to its own authenticated browser, never in peer/chat payloads. Account network is saved as fractal-mainnet; a signature proves address control of a network-bound message, while the injected wallet's exact chain enum is checked before signing and during the session. Compatible Bitcoin/Fractal address encoding alone cannot attest wallet chain selection.

## Browser acceptance (2026-10-07)
Evidence directory: `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-005`.

- PASS, DEV: wallet-missing prompt recovers without blank screen; two characters and two names selectable; no nickname input.
- PASS, DEV: alice.btc and bob.btc joined the newly created `FT005 Identity Acceptance` room. Both peer labels, online list and bidirectional chat use .btc; screenshot evidence explicitly displays DEV ACCEPTANCE.
- PASS, DEV: refresh reopens the same character's room; chicken feed decreased 5→4, carrot consumption 3→2, hunger recovered; refresh retained inventory. Bob disconnect returned to Gate and Alice saw online count 2→1. Re-login requires selection again. Selecting bob's second character/name opened a separate empty membership lobby.
- Real UniSat: extension toolbar detected in Chrome. Native control was interrupted twice before navigating a new task window; user was asked to open the localhost page and unlock personally if needed. No successful connection, reject/signature, chain switch or server wallet session has yet been observed. This is NEEDS_VALIDATION, not real-wallet PASS.
- Real Ordinals: BLOCKED by missing server API key and approved Character allowlist. Public unauthenticated API probe returned403; no chain-ownership acceptance claim.
- Real .btc: public BNRP sample schema responded200; test-wallet ownership is NOT RUN. The public sample does not establish any player entitlement.

## Known provider and operational limits
BNRP uses Bitcoin-anchored names; character inscriptions are separately queried on Fractal mainnet. Current reverse API discovers the primary name only, so real multi-name enumeration remains a provider limitation; the selection interface and DEV/test provider cover multiple returned names. Asset previews are safe procedural placeholders, with no untrusted inscription HTML/SVG execution. BIP-322 supports native P2WPKH and P2TR key-path only; unsupported address/script forms fail explicitly. Ownership is revalidated at each new authentication and selection; no background on-chain transfer watcher is included. Session/socket expiration is finite. This remains a single Go-instance baseline, not a multi-node deployment implementation. FT-WEB-006 is not started.


## Security implementation and dependencies
Wallet sessions use an independent HttpOnly, SameSite=Lax cookie; Secure is enabled in production. Tokens and nonces contain 32 cryptographically random bytes; only token SHA-256 hashes are persisted. Challenges expire after300 seconds and the cache is capped at2000 entries with request rate limits. A separate browser-binding cookie prevents cross-browser proof submission; a proof is consumed atomically and is burned if session persistence fails. Session lifetime is24 hours. The provider key is read only by the server; no frontend wallet SDK or credential variable is added.

The verifier reconstructs BIP-322 to_spend/to_sign and runs established btcd transaction script validation. Verification-only official BIP fixtures retain7 supported valid signatures,8 invalid cases and3 expected transaction hashes, removing the published fixture private keys. Fixture license is CC0-1.0. Bitcoin packages are ISC: btcd0.25.0, btcec/v2 2.3.5, btcutil1.1.5, chainhash1.1.0. Existing Go1.24 is retained. The pinned btcec Schnorr parser's scalar-overflow issue is blocked before script execution using ModNScalar.SetByteSlice, with a boundary regression. Upstream fix: https://github.com/btcsuite/btcd/pull/2561 ; release https://github.com/btcsuite/btcd/releases/tag/v0.26.2 requires the newer Go baseline. Frontend package dependencies are unchanged and the production bundle contains neither MockWalletAdapter nor DEV controls.

## Automated acceptance map
| Requested checks | Evidence |
| --- | --- |
|1–9 challenge/auth security|Randomness, expiry, one-use/replay, wrong address/network/signature, valid real proof, origin and browser-binding mismatch tests|
|10–12 wallet account|Canonical address uniqueness, same-account re-login, recorded provider/network; concurrent PostgreSQL pools|
|13–17 Character eligibility|Empty/allowlisted/multiple/wrong-owner cases and production DEV rejection|
|18–24 .btc|Missing/valid/suffix/owner/reverse-forward/multiple/outage cases; routed address alone rejected|
|25–30 identity|Immutable character ID, server-controlled .btc, username cannot override, fresh login ownership and both transfer invalidations|
|31–32 multiplayer/chat|Real WebSocket fixture plus actual two-browser DEV labels/online/chat|
|33–39 gameplay/persistence|Selected-character signed-proof integration: gather/build/till/plant/water/feed/eat/fish/save-reopen; baseline movement/build/life suites|

Independent review fixes cover logout during stalled provider lookup, stale restore/sign/select publication, older cleanup affecting newer login, failed ownership-status saves, HTTP401/WS1006 recovery, and full authority revocation. External provider calls never hold the global authentication lock. UI mutation ordering waits for old requests to settle before logout and new authentication; if a wallet popup remains pending, settle that request to continue.


Server restart policy: persisted wallet sessions carry a cryptographically random process boot epoch. A new process rejects old or missing epochs, requiring fresh wallet authentication and ownership selection. Wallet accounts, character IDs, room membership and all world/personal progress remain saved. This deliberately sacrifices session continuity across Go-server restarts so that a failed durable revocation cannot become an authorization bypass after a crash. No startup provider lookup or destructive data migration is required.


## Final validation
- Frontend `npm test`:87/87 PASS; identity subset33/33. `npm run build`:PASS TypeScript+Vite. No frontend dependency added.
- Server `go test -race -json ./...` with TEST_DATABASE_URL:186 test/subtest pass events,85 top-level tests,0 failures,0 skipped. Disposable PostgreSQL schemas include identity uniqueness/rollback/persistence plus baseline world tests. `go vet ./...`:PASS.
- Independent task reviews:PASS after two scoped correction rounds. Whole-branch independent security/spec review:PASS; no remaining Critical/Important findings.
- Final browser DEV double identity revalidated after final server restart: old cookie rejected, both users selected the same original characters and returned to original room. Labels/chat/online still .btc. Real-wallet acceptance remains NEEDS_VALIDATION.
- Client console inspected through browser tool: no error/warn entries during final normal gameplay. Intentional unauthenticated requests return401 as designed.
- Bundle main entry measured below; production search found no MockWalletAdapter, DEV persona, Use DEV identity, or VITE_UNISAT_API_KEY.
- Main bundle 105.90kB raw, 33.76kB gzip (baseline92.06kB/29.65kB).

## Exact changed-file manifest
Created:
- `docs/freetower/FT-WEB-005-FRACTAL-IDENTITY.md`
- `server/identity.env.example`
- `server/identity_dev.go`
- `server/identity_http.go`
- `server/identity_http_test.go`
- `server/identity_model.go`
- `server/identity_postgres_test.go`
- `server/identity_providers.go`
- `server/identity_providers_test.go`
- `server/identity_restart_test.go`
- `server/identity_security_regression_test.go`
- `server/identity_selection.go`
- `server/identity_signature.go`
- `server/identity_signature_test.go`
- `server/identity_store.go`
- `server/testdata/BIP322-SOURCES.md`
- `server/testdata/bip322-basic.json`
- `server/testdata/bip322-generated.json`
- `src/identity/IdentityGate.tsx`
- `src/identity/api.ts`
- `src/identity/identity.css`
- `src/identity/lifecycle.ts`
- `src/identity/model.ts`
- `src/identity/recovery.ts`
- `src/identity/types.ts`
- `src/identity/wallet/MockWalletAdapter.ts`
- `src/identity/wallet/UniSatWalletAdapter.ts`
- `src/identity/wallet/types.ts`
- `tests/identity-lifecycle.test.ts`
- `tests/identity.test.ts`

Modified:
- `README.md`
- `server/README.md`
- `server/auth.go`
- `server/go.mod`
- `server/go.sum`
- `server/http.go`
- `server/http_test.go`
- `server/model.go`
- `server/store.go`
- `server/websocket.go`
- `src/game/api.ts`
- `src/game/types.ts`
- `src/main.tsx`
- `src/world/freetower/FreeTowerApp.tsx`
- `src/world/freetower/World.tsx`
