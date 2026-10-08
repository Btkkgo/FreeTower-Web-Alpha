# FT-WEB-006 — Public staging deployment

Baseline: `381d7af5dee3e10b0033de31de16e9003a095da6`.
Branch: `codex/ft-web-006-staging-deployment`.
Runtime source: `e6f4bc8`; final documentation commit is separate from the deployed runtime SHA.
Scope: staging only. Production launch remains closed; production Character allowlist remains empty.

## Architecture and cloud resources

Internet → one Railway HTTPS/WSS origin → one Go server serving Vite production `dist/`, REST and WebSocket → private PostgreSQL. Both services and the database volume are in Singapore `asia-southeast1-eqsg3a`. No Vite development server, Pages/R2 split, Redis, scaling or GitHub auto-deploy.

| Resource | Verified value |
| --- | --- |
| Project | FreeTower-Staging · `35fa2b1d-b27c-47ed-8d71-06731c320e57` |
| Environment | staging · `a214d06e-011f-4f34-ae83-34dfba274e47` |
| Game Server | `a0837999-6c62-4ee3-a782-ea6c123d8dea`, one replica |
| PostgreSQL | `8100c9ba-86ad-4bf1-8e14-faa4f64a1f15`, PostgreSQL 16.15, private connection |
| Volume | `4e5c3979-0ff9-4590-b672-d3de924ca5ad`, 500 MB, `/var/lib/postgresql/data` |
| PostgreSQL deployment | `1fff6b3f-c20e-444b-8575-f9f242e46025`, retained throughout app maintenance |
| Public URL | <https://game-server-staging-7a18.up.railway.app/> |
| Plan | Existing HOBBY; no upgrade |
| Default production environment | No service or volume instances; no production deployment |

The multi-stage Dockerfile uses Node 24, Go 1.26 and a non-root Alpine runtime with CA certificates. It copies the four shared game-definition JSON files into the frontend stage. No node_modules, local save, .env or provider credential goes into the runtime image. Dynamic `PORT` and static `dist/` serving work; public mode fails startup if the required database or canonical HTTPS origin is absent. Schema initialization is idempotent and does not erase saved rooms.

## Official Railway configuration migration

Executed official CLI 5.63.4 `railway config migrate --service 'Game Server' --show`, then `--apply`, reviewed the generated TypeScript and plan, and applied the plan during a stopped-app maintenance window. The single authoritative configuration is `.railway/railway.ts` using official `railway/iac`; old `railway.json` was removed and there is no railway.toml. Migration status confirms no staging service reads old Config as Code.

The partial owns only Game Server and explicitly rejects non-staging environments. Equivalent settings were verified against the former JSON and the actual deployment manifest: Dockerfile builder, Singapore one replica, `/api/health`, 120-second health timeout, ON_FAILURE with three retries, 120-second drain, zero overlap and sleeping disabled. Database reference remains `${{Postgres.DATABASE_URL}}`; other cloud-held configuration/secrets use `preserve()`. Fingerprint comparison of all 23 existing variables passed without printing values. PostgreSQL remains outside this service partial and unchanged.

CLI plan still renders null-to-default changes for replica/restart/sleep after apply, although the actual deployed manifest contains the intended values. This is recorded as plan/default normalization noise, not proof of lost runtime configuration. Do not repeatedly apply such a plan or recreate resources. Official reference: [Railway IaC migration](https://docs.railway.com/infrastructure-as-code#migrating-from-config-as-code).

Environment variable names: `FREETOWER_ENV`, `FREETOWER_PUBLIC_ORIGIN`, `FREETOWER_REQUIRE_DATABASE`, `FREETOWER_GIT_SHA`, `FREETOWER_STAGING_CHARACTER_INSCRIPTION_ID`, `DATABASE_URL`, `UNISAT_FRACTAL_API_KEY`, `PORT`, `STATIC_DIR`, `FT_CHARACTER_INSCRIPTION_ALLOWLIST`, `FT_ENABLE_DEV_IDENTITY`, `FT_ENABLE_LEGACY_AUTH`, `FT_ENABLE_OWNERSHIP_ACCEPTANCE`. Secret values live only in Railway Variables, never under VITE_ or in Git.

## Identity and production boundary

The reserved staging inscription is `812e807292195582243f7370fde8155dd1e21ff6cb9841bc1cf543a2c0859f92i0`. It is not a Production Character, Alpha Character or production allowlist member. Only canonical staging with this exact configuration can admit it, after real UniSat Fractal owner matching and normal verified .fb selection. Production ignores staging configuration and rejects staging identities. A Vite production build is not a production server environment.

Native Chrome with real UniSat completed Fractal Bitcoin mainnet connection, origin-bound BIP-322 sign-in, server verification, test ordinal ownership, yihoon.fb verification and selection, then game entry. UI displayed `Staging Test Character · yihoon.fb`; authenticated API reported real `unisat-fractal-satsnames`, dev false and environment staging. PostgreSQL contained one staging identity and zero production identities. The .fb implementation was not modified. Addresses, cookies and signatures are omitted from evidence.

Real Cookie flags were inspected in native Chrome: Secure, HttpOnly and SameSite=Lax. No third-party origin or permissive CORS was introduced. Public legacy login, DEV wallet and local ownership-acceptance routes return 403; missing/wrong Origin is rejected. Missing-cookie WSS returns 401; wrong-Origin WSS returns 403. Real authenticated TLS WSS upgrade returned 101. Two WSS connections for the same real identity received a chat broadcast. Independent-wallet multiplayer remains PARTIAL; a second wallet was not fabricated.

Replay rejection passes automated identity tests; a captured real public signature replay is NOT RUN. Routine wallet unlock remains a user action, and a locked wallet never falls back to mock identity.

## Gameplay and jitter

Public browser observed walking/click movement, swimming, camera rotation/reset, hand/tool selection, tree gathering, feeding a chicken and collecting an egg, catching Coastal Snapper, and eating a carrot. Public authenticated WSS protocol acceptance additionally verified rock gathering, building a campfire with gathered resources, its construction timer and move, building a farm plot, till/plant/water/grow/harvest, drinking water and chat. Protocol operations used the actual browser-authenticated cookie over the public endpoint and retained all server ownership/range/cost/cooldown checks. They are distinct from browser-UI acceptance.

Final checkpoint: campfire at x=5,z=17, rotation π/6; farm plot at x=4,z=25, tilled after harvest; shared wood=0/stone=1; personal wheat=3, egg=1, Coastal Snapper=1, carrot=2, bait=17, wheat seed=4, water flask=4, chicken feed=4. Four chickens and their feed/egg history remain stored. Early helper attempts correctly received cooldown and invalid-rotation errors; the helper was corrected to wait 700 ms and send radians. Server validation was not weakened.

The reported movement jitter had a concrete reconciliation defect: delayed self-presence was compared with the character's current predicted position, causing rewinds. The deployed fix attaches per-connection move sequences, compares each acknowledgement with its original sent position, applies only actual server corrections to current/pending prediction, and ignores stale/duplicate acknowledgements. Targeted regression tests pass. Final user assessment and sustained native Run/Jump verification remain pending; static screenshots alone do not establish smooth motion.

## Persistence, restart and backup

SSH inspection succeeded using the user-registered Btkkgo public key. PostgreSQL was not recreated. The first controlled stop/migration/upload retained the staging identity and survival state; the user reauthenticated and re-entered the game. The second maintenance comparison PASS: room/identity hashes, buildings, plots, resource-node hash, animal feed/egg history, shared and personal inventory all match. Health/energy remained 100; hunger and thirst decreased by exactly the expected 328.004 seconds of simulation. Detailed comparisons are in `gameplay-*.json` and `redeploy-persistence-result.json`. Current deployment is `7ba6fa06-377a-47d6-92f5-64c5a5e8ee4d`; public health reports e6f4bc8, database true, environment staging.

The second `railway redeploy` attempt rebuilt a source with the old missing-JSON Docker error and failed before starting a container. Recovery used `railway up` from the tested checkout. Do not use ambiguous latest-source redeploy for this service; verify the upload revision and resulting health SHA. The database remained running throughout.

Manual custom-format pg_dump and isolated transactional restore both PASS, including the latest gameplay archive with one room, two buildings and zero production identities. Dumps are outside Git in a 0700 private directory, mode 0600. Automatic DAILY volume backup is BLOCKED: Railway returned Not Authorized; the schedule is empty and no upgrade/access bypass occurred. See [backup runbook](STAGING_BACKUP.md). An application restart is not itself backup evidence.

## Maintenance and writer safety

One replica alone does not prevent overlapping in-memory world writers. Public startup acquires a database/schema-scoped PostgreSQL advisory lock before loading rooms. Its physical session performs all world writes. A competing app fails startup; loss of the session or lock fails closed, stops simulation and closes clients. No pool fallback is permitted. Use the direct PostgreSQL connection, not transaction pooling; do not attach old development binaries to this database.

1. Verify project/environment/service identifiers and passing test/build gates; capture a sanitized checkpoint.
2. Stop only Game Server and wait for `deploymentStopped=true`; PostgreSQL and volume stay running.
3. Upload the tested checkout using `railway up -s 'Game Server' -e staging --detach` only after the old process stops. Configuration apply can trigger a deployment: stop and verify that deployment too before a subsequent upload.
4. Verify actual deployment manifest, health SHA/environment/database, persistent state, reauthentication and WSS. Accept maintenance downtime including build time.

Go drains HTTP, stops ticker/client work, saves rooms and closes the store on SIGTERM. Local PostgreSQL tests cover active websocket shutdown/final-state handoff. Cloud SIGTERM and stopped status were observed; the second browser session was locked/disconnected before stop, so no public active-client close-frame claim is made. Infrastructure-initiated overlapping replacement may fail closed and require recovery; seamless availability is not claimed.

## Security and reliability

CSP supports same-origin assets/WSS and existing Google Fonts; frame-ancestors none, X-Frame-Options DENY, nosniff and Referrer-Policy are set. Real world rendering and injected wallet flow succeeded. Public API limit is 180/minute by socket peer; wallet challenge 30/5 minutes, verify 60/5 minutes; game/action, building/gathering/life cooldowns and chat interval remain enforced. Proxy pooling may share peer-address budgets; multi-client capacity is unvalidated.

Provider HTTP timeout is 8 seconds with a 12-second options context. Health has bounded database checks and returns only ok/database/environment/short commit. Startup log says FreeTower. Go's normal stderr startup line is tagged error by Railway ingestion; it is not itself a runtime exception. Logs must never expose provider secrets, wallet addresses, signatures, cookies or DB URLs.

## Validation, performance and cost

Fresh frontend suite: 93/93 PASS, zero failed/skipped. TypeScript/Vite production build PASS. Uncached Go race run: 254 passing test/subtest events, 109 top-level, zero failed/skipped, including 12 PostgreSQL-backed top-level tests. `go vet ./...` PASS. Production gate tests reject staging assets/identities outside canonical staging and wrong-owner/provider-failure paths remain closed. Targeted movement sequence/reconciliation tests pass. No code changed after these gates; final changes are documentation/evidence.

Optimized static assets are bundled in dist. Public HTML, JS/CSS, GLB/images and SPA fallback were fetched successfully with verified TLS. Final public transfer samples: HTML 493 bytes/0.898 s; Three.js encoded response 493,968 bytes/0.702 s; cottage GLB 1,677,316 bytes/1.270 s; inventory atlas 525,176 bytes/0.604 s. All 18 final page/API/asset requests returned 200, with 3,602,761 bytes across the enumerated JS/CSS/art/model transfers. These are individual requests on this connection, not page-load totals. Approximate raw dist is 4.88 MB. Browser debug FPS observed roughly 60–122; no controlled benchmark or precise time-to-visible-world was captured. Real server-side identity requests were approximately 0.65–1.27 s. WSS upgrade succeeded; accurate handshake latency was not separately recorded.

Existing Hobby plan is $5/month including $5 resource usage ([official pricing](https://docs.railway.com/pricing/plans)); overage depends on actual use. CLI partial-period observation: project $0.0032603, Game Server $0.0021096, Postgres $0.0011508. Metering lag and the short partial period make a monthly forecast unreliable. Both services are one Singapore replica; volume 500 MB; no plan upgrade. No fixed monthly usage promise is made.

## Evidence and remaining limits

Evidence: `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-006`. Scope labels distinguish automated tests, public browser observations, authenticated public WSS protocol acceptance and SSH database evidence.

Remaining acceptance: UniSat is currently auto-locked and awaits the user; final native Run/Jump and sustained jitter assessment, a captured real replay rejection, final unlocked-wallet re-entry after the second restart, and authenticated-game console inspection. Multiplayer uses one wallet only. Automatic backup schedule is not enabled. Build logs report a high-severity source-map-js advisory in the build chain; no Node dependencies are installed in the final runtime image. IaC plan has default-normalization noise. Browser/host automation has a canvas coordinate mismatch, so protocol checks are not labelled full UI acceptance.

NOT READY FOR CLOSED ALPHA until remaining public browser checks are resolved. No FT-WEB-007 work and no Production Launch.

## Exact file inventory relative to baseline

Created (13):

- `.railway/railway.ts`
- `docs/freetower/FT-WEB-006-STAGING-DEPLOYMENT.md`
- `docs/freetower/STAGING_BACKUP.md`
- `docs/superpowers/plans/2026-10-08-freetower-staging.md`
- `server/identity_staging_test.go`
- `server/movement_sequence_test.go`
- `server/runtime_config.go`
- `server/runtime_config_test.go`
- `server/runtime_shutdown_test.go`
- `server/world_writer.go`
- `server/world_writer_test.go`
- `src/world/freetower/movement-reconciliation.ts`
- `tests/movement-reconciliation.test.ts`

Modified (22):

- `.dockerignore`
- `Dockerfile`
- `package-lock.json`
- `package.json`
- `server/auth.go`
- `server/http.go`
- `server/identity.env.example`
- `server/identity_http.go`
- `server/identity_model.go`
- `server/identity_providers.go`
- `server/identity_selection.go`
- `server/identity_store.go`
- `server/main.go`
- `server/model.go`
- `server/store.go`
- `server/websocket.go`
- `src/game/types.ts`
- `src/identity/IdentityGate.tsx`
- `src/identity/model.ts`
- `src/identity/types.ts`
- `src/world/freetower/FreeTowerApp.tsx`
- `tests/identity.test.ts`

The intermediate railway.json was removed during migration and is absent from the final tree. Scoped secret scan and git diff --check PASS. The temporary real browser-session file was deleted after protocol acceptance and its in-memory browser binding cleared. No credential, session file or database dump is included in Git.
