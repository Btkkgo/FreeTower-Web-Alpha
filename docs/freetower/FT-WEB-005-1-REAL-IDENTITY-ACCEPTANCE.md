# FT-WEB-005.1 — Real Fractal Identity Acceptance

Date: 2026-10-07 (Asia/Shanghai). Baseline: `7ad08775c32929498f58a99d0a02e9869c7db2aa`.
Branch: `codex/ft-web-005-1-real-identity-acceptance`.

**Wallet authentication: REAL PASS. `.fb` ownership: REAL PASS (`yihoon.fb`). Character: BLOCKED — NO APPROVED CHARACTER ASSET. Identity Gate: BLOCKED AS DESIGNED. NOT READY FOR FT-WEB-006.**

The user explicitly replaced `.btc` names with `.fb`, retained the independent Character asset requirement, and supplied `yihoon.fb`. This changes the name provider and validation only. No arbitrary inscription was designated a Character. No game/map/art changes, mint, transfer, FB send, PSBT, purchase, or other transaction was performed by the agent. FT-WEB-006 was not started.

## Real acceptance

| Item | Status | Observed evidence / boundary |
| --- | --- | --- |
| UniSat detected/version | PASS | Actual Chrome extension, version **1.7.22**, read from extension details. |
| Address/type | PASS | Official wallet connection, **P2TR**, `bc1pzp…u3qz60` (first 6 + last 6). |
| Real connect/reject/retry | PASS | Actual UniSat connection popup; rejection displayed recoverable cancellation; retry connected. Initial approval emits an account-change event and can require clicking Connect again. No authentication bypass. |
| Network/switch | PASS | Initially BITCOIN_MAINNET; actual wallet switch popup approved; FreeTower subsequently read Fractal Bitcoin Mainnet. Signed message network: `fractal-mainnet`. |
| Real challenge | PASS | Multiple actual challenges; origin `http://localhost:4318`, correct address/network; each observed nonce distinct, length 64 hex characters; TTL 300000 ms. Cryptographic randomness also checked in implementation/tests. |
| Signature popup/rejection/retry | PASS | Real FreeTower Sign-In popup inspected; reject returned recoverable cancellation. A newly issued challenge was then signed successfully. No full nonce or signature retained in evidence. |
| Real signature verification/session | PASS | Server BIP-322 verification HTTP 200; `dev=false`; wallet account and session established. This is a wallet session, **not** an active CharacterIdentity. |
| Consumed-proof replay | PASS | Same real proof and challenge binding replayed: HTTP 401 `invalid_challenge`, no second session cookie. |
| UniSat key/API | PASS | User configured the server-only environment key. Real Fractal Open API HTTP 200 / code 0. Key was preserved in process environment through local restarts, never written to Git, bundle, report, or screenshots. |
| Owned inscriptions | PASS | Initial read: 149; latest successful reads: **150**. These are time-separated observations, not an eligibility count or an agent transaction. |
| Approved Character configuration | BLOCKED | No independent Character inscription ID was approved. Exact-ID allowlist remains empty. Name approval does not imply Character approval. |
| Character eligibility/ownership/selection | BLOCKED | Real options returned `assets: []`; UI says No eligible Character asset found; Enter FreeTower disabled. No positive Character ownership claim. |
| Real `.fb` reverse discovery | PASS | Enumerated current address inscriptions through official UniSat Fractal wallet API, read bounded SNS registration content, and matched the canonical forward inscription. Returned `yihoon.fb`. This is owned-name enumeration, not a primary-name reverse endpoint. |
| Real `.fb` forward/ownership | PASS | Official domain search resolves `yihoon.fb` to this wallet. Content has `p=sns`, `op=reg`, matching name; 24 confirmations at focused inspection. Fractal Open API independently confirmed inscription ID/current owner; real server options returned the verified name. |
| Provider/network | PASS | `unisat-fractal-satsnames`, **Fractal Bitcoin mainnet**. Old BNRP/Bitcoin-mainnet `.btc` probe returned no verified primary name; it is historical evidence and no longer the active gate. |
| Multiple names | NOT RUN | One verified real `.fb` found. Two-name behavior covered by automated provider tests, not claimed as real multiple-name acceptance. |
| CharacterIdentity/game entry | BLOCKED | Wallet and name proofs are insufficient without independently approved Character eligibility/ownership. No active identity created. |
| Player/online/chat labels | NOT RUN | Real Character could not enter world. Automated identity/WebSocket tests use `.fb` fixtures and are separate evidence. |
| Wallet privacy | PASS | Gate displays no full address; saved auth evidence/report mask the real address. No real in-world privacy claim because entry was blocked. |
| Disconnect/session revocation | PASS | Real Disconnect returned to Connect Wallet; logout HTTP 200; old cookie queried immediately afterward returned HTTP 401. No game WebSocket existed to exercise socket closure. |
| Re-login | PASS | After real logout, new connect/challenge/signature established a fresh real session; options re-fetched `yihoon.fb` with zero approved Characters. |
| Ownership revalidation | PASS | Fresh `.fb` reverse/forward/indexer checks after re-login; no name cache or success-history shortcut. Positive Character revalidation remains BLOCKED. |
| Real gameplay regression | NOT RUN | Walk/swim/gather/build/fish/farm require a real eligible Character. Automated gameplay suite remains separate. |

Challenge examples (UTC, sanitized): rejected signature challenge issued `2026-10-07T15:32:16Z`, expiry `15:37:16Z`; retry issued `15:33:25Z`, expiry `15:38:25Z`; `.fb` gate login issued `15:40:40Z`, expiry `15:45:40Z`. The local audit recorded only metadata, status, counts, masked address and public name. Replay used the real proof in memory; no proof/cookie/token was saved.

## Minimal implementation and provider contract

- Replaced the production BNRP resolver with a Fractal `.fb` resolver; selection validates `.fb`; DEV fixtures and UI use `.fb` consistently.
- Kept existing JSON/storage field `btcName` and Go type names for compatibility; their current value is a verified `.fb` name. No schema migration or internal identifier change. Identity IDs remain independent of the name. Previously stored `.btc` authority does not satisfy the new resolver on reauthentication.
- Kept Character's exact-ID allowlist and ownership provider independent. No `.fb` inscription is automatically eligible as a Character.
- Names require valid lowercase NFC labels, confirmed SNS `reg` JSON in bounded text/plain content, at least 3 current-UTXO confirmations, canonical forward inscription equality, wallet owner equality, and fresh Fractal indexer ownership.
- All external calls are GET. Hosts/paths are fixed; provider-supplied content URLs are ignored. Redirects are refused, responses bounded and requests time-limited. The Open API key is sent only to `open-api-fractal.unisat.io`, never the wallet API or content CDN. Provider failures close the gate.
- Ordinary non-JSON text inscriptions are ignored, rather than breaking discovery. Enumeration is capped at 2000 inscriptions, with stable totals/duplicate checks. Unsupported content, larger wallets, upstream changes/timeouts or missing credentials cannot fall back to mock success. Only SNS JSON registrations are currently accepted; this is not a claim of support for every historical naming encoding.
- The wallet API is used because the official wallet SDK documents the resolution route; this remains an upstream dependency, not a project-controlled canonical indexer. No marketplace/trading APIs are used.

Sources: [UniSat name protocols, networks and 3-confirmation rule](https://docs.unisat.io/misc/knowledge-base/ordinals/.unisat-names/name-recognized-and-resolved); [official extension Fractal endpoint configuration](https://github.com/unisat-wallet/extension/blob/master/src/shared/constant/index.ts); [official extension wallet API integration](https://github.com/unisat-wallet/extension/blob/master/src/background/service/walletapi.ts); [`@unisat/wallet-api` 1.4.0 SDK](https://www.npmjs.com/package/@unisat/wallet-api) (`src/services/domain.ts`, `src/services/inscriptions.ts`); [official inscription indexer contract](https://github.com/unisat-wallet/unisat-dev-docs/blob/master/open-api/swagger-source/inscription-indexer.yaml).

## Production mock check — PASS

Final code/build ran in a separate process on loopback port 8087 with `FT_ENV=production`, no API key, empty allowlist and isolated save path. Deliberately setting both enable flags true still yielded `development=false`, `devEnabled=false`, `legacyEnabled=false`.

- DEV identity POST: 403 `dev_disabled`.
- Legacy login POST: 403 `legacy_disabled`.
- Unauthenticated identity options: 401 `unauthenticated`.
- Production browser URL with `?identityDev=1&legacyAuth=1` showed only Connect Wallet, with `.fb` labels; no DEV banner/persona/mock or legacy login.
- Real authenticated localhost gate independently demonstrated empty-allowlist refusal.

This is a local production build/negative-authority check, not an HTTPS deployment certification. Real localhost acceptance uses development transport solely for HTTP origin support, with DEV and legacy identity explicitly disabled.

## Fresh automated regression — PASS

- `npm test`: **87/87**, zero failures/skips.
- `npm run build`: TypeScript and Vite PASS; entry 105.90 kB raw / 33.81 kB gzip.
- `TEST_DATABASE_URL=<authorized local test DB> go test -race -count=1 -json ./...`: **200 test/subtest pass events; 86 top-level tests; zero failures/skips**.
- PostgreSQL: all **6** integration tests PASS, including wallet identity persistence/atomicity and game persistence/migration.
- `go vet ./...`: PASS.

Tests were run after the last provider fix. New provider regression cases include canonical versus duplicate inscriptions, two names, wrong owner/protocol/name, underconfirmed transfers, missing key/code, provider failure, incomplete pagination, independent-indexer disagreement and unrelated text. Tests first failed on the absent `.fb` provider; the unrelated-text regression separately failed before its fix. These generated fixtures do not substitute for the real-wallet evidence above.

## Security, runtime and evidence

No real key, raw authentication proof, nonce, cookie/session token or full real address is included in the tracked change. Actual-key/address and sensitive-payload scans cover changed files, production JS, captured logs and textual evidence. Production browser console: zero warning/error entries. A complete native Chrome real-signing DevTools console capture was NOT RUN; sanitized server-side evidence was captured instead, so no blanket real-console-clean claim is made.

Evidence (local, not published with Git): `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-005-1`.

Includes real `.fb` gate screenshot/AX, signature-popup and rejection AX with nonce/address redacted, UniSat version, sanitized real audit, production screenshot/AX/negative endpoint results/console, final test/build/race/vet logs and scan manifest.

Initial user-started server lacked `FT_PUBLIC_ORIGIN`, causing origin rejection. Local runtime configuration was corrected without weakening origin validation: `FT_PUBLIC_ORIGIN=http://localhost:4318`, `FT_ALLOWED_ORIGINS=http://127.0.0.1:4318`, `FT_ENABLE_DEV_IDENTITY=false`, `FT_ENABLE_LEGACY_AUTH=false`. The temporary audit proxy is removed at closeout; the current source runs directly on port 8080 with the key preserved in environment. That restart invalidates existing wallet sessions by design. Isolated production-check processes are stopped; world data is preserved.

Remaining prerequisite: user approval of an **independent** existing Character inscription ID, configured outside business code with ID, purpose=`FreeTower Alpha Test Character`, and explicit environment. Then rerun Character ownership/selection, active identity, world labels/chat/multiplayer, gameplay, and in-world logout/re-login. No mint or automatic role promotion is authorized.
