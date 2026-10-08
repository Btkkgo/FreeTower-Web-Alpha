# FT-WEB-006 — Public staging deployment

Baseline: `381d7af5dee3e10b0033de31de16e9003a095da6`.
Branch: `codex/ft-web-006-staging-deployment`.
Scope: one staging instance; production launch remains closed.

## Architecture and authority

Internet → Railway HTTPS/WSS endpoint → one FreeTower Go service serving Vite `dist/`, REST and websocket → PostgreSQL in the same Singapore region. The target project is `FreeTower-Staging`, environment `staging`, region `asia-southeast1-eqsg3a`. No Pages/R2 split, Redis or horizontal scaling.

Vite's optimized production **build mode** is used for the static bundle, as requested. Character authority belongs to the server's **deployment environment**, not the JavaScript build mode. The exact staging acceptance inscription is `812e807292195582243f7370fde8155dd1e21ff6cb9841bc1cf543a2c0859f92i0`. It is neither a production character nor a production allowlist member. Staging acceptance requires explicit configuration, real UniSat ownership matching the authenticated Fractal-mainnet wallet, and the normal verified `.fb` selection flow. Staging identities must be labelled and environment scoped. Production must ignore staging config and reject staging identities.

## Deployment record

| Item | Observed state |
| --- | --- |
| Railway CLI | npm-executed official `@railway/cli`, version 5.63.4; unauthenticated at preflight |
| Railway Dashboard | Login required; user handoff requested |
| Account plan / current usage | NOT VERIFIED |
| Project / environment / services | NOT CREATED by this task yet |
| Service / PostgreSQL region | Target Singapore; remote placement NOT VERIFIED |
| Public URL / HTTPS / WSS | NOT RUN |
| Docker build | Local Docker unavailable; remote build NOT RUN |
| Public wallet / Fractal / BIP-322 / replay | NOT RUN |
| Public ordinal / yihoon.fb / staging identity / game entry | NOT RUN |
| Restart and Railway PostgreSQL persistence | NOT RUN |
| Backup schedule / restore drill | NOT VERIFIED / NOT RUN |

Do not replace any NOT RUN above with localhost or unit-test evidence. Deployment must wait until build, frontend, Go race, PostgreSQL integration and vet gates pass.

## Configuration

Environment variable **names only** (secret values belong exclusively in Railway Variables):

- `FREETOWER_ENV`
- `FREETOWER_PUBLIC_ORIGIN`
- `FREETOWER_REQUIRE_DATABASE`
- `FREETOWER_GIT_SHA`
- `FREETOWER_STAGING_CHARACTER_INSCRIPTION_ID`
- `DATABASE_URL`
- `UNISAT_FRACTAL_API_KEY`
- `PORT`
- `STATIC_DIR`

Configure `DATABASE_URL` as a Railway reference to the PostgreSQL service. Keep the production character allowlist empty. Do not set any secret under a `VITE_` prefix. Obtain the actual Railway HTTPS domain before setting the public origin and creating wallet challenges. App and database must both use Singapore, with one app replica and no plan upgrade.

## Runtime validation

Runtime foundation is implemented and covered by local tests. Public environments require PostgreSQL and an exact HTTPS origin; obvious loopback/private/internal hostnames are rejected. Missing database configuration fails before opening a JSON store. Health exposes database connectivity, environment and a validated seven-character commit. Origin guards reject missing or mismatched origins on public mutation and websocket paths. Public identity cookies enforce Secure, HttpOnly and SameSite=Lax; legacy/DEV/local acceptance are forced off.

Static serving and dynamic PORT behavior are retained. The database schema uses idempotent creation/alter statements without clearing state. Shutdown stops the ticker, closes clients, drains websocket handlers and saves rooms before DB close. Startup uses the FreeTower name; runtime failures omit raw provider/database error details.

CSP allows same-origin resources, the configured WSS endpoint, existing Google Fonts, and the data/blob image/worker resources needed by the client. Frame policy is DENY with frame-ancestors none; nosniff and referrer policy are present. Provider HTTP/context timeouts remain bounded. Existing challenge/verify/game/chat limits remain, with an additional public API limiter. The new limiter uses socket peer addresses, so Railway proxy pooling may share a budget; actual multi-client behavior remains a public acceptance check.

These are code/local-test results; live proxy, TLS/cookie, CSP, Docker and graceful redeploy behavior still require Railway evidence.

## Public acceptance matrix

| Area | Required checks | Status |
| --- | --- | --- |
| Delivery | HTTPS HTML, JS/CSS/assets 200, no 404, health/database/version | NOT RUN |
| Identity | UniSat detection, Fractal mainnet, BIP-322, replay rejection | NOT RUN |
| Authority | Real test ordinal owner; yihoon.fb reverse/forward/ownership; staging identity | NOT RUN |
| Entry | Real game entry, one real WSS connection, no DEV/legacy route | NOT RUN |
| Movement | Walk, run, jump, swim, reload | NOT RUN |
| Resources | Tree, rock, hand switching | NOT RUN |
| Construction | Campfire placement/completion/move | NOT RUN |
| Life | Fish; farm plot, plant, water, harvest; chicken | NOT RUN |
| Social | Chat; dual sessions if available (one wallet may remain PARTIAL) | NOT RUN |
| Persistence | World/character/resources/crops/animals/survival before and after app-only restart | NOT RUN |
| Recovery | Reauthentication and WSS reconnect after restart; DB not recreated | NOT RUN |
| Diagnostics | Console fatal errors, server errors/secrets, transfer sizes and performance | NOT RUN |

## Backup, performance and cost

[Backup runbook](STAGING_BACKUP.md) covers private logical dumps, isolated restore verification and recovery. Dashboard entitlement/schedule must be recorded rather than assumed.

Record actual plan, app and database resource limits, current/estimated usage from the dashboard after authorization. No cost estimate is established by this local preparation; no cloud service was provisioned and no plan upgrade was performed. No automatic plan upgrade is authorized.

Performance baseline must distinguish HTML load, JS transfer, asset transfer, time to visible world, API latency, WSS connection time and observed FPS. Local bundle size alone is not public performance evidence. All assets remain bundled with dist; no CDN/storage migration.

## Evidence and limitations

Evidence directory: `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-006`. Public account authentication is currently the deployment blocker. No live public acceptance or closed-alpha readiness is claimed. Existing FT-WEB-005.2 ownership evidence and yihoon.fb PASS are historical local results and must be rerun on staging.

Official references: [Singapore regions](https://docs.railway.com/deployments/regions), [config as code](https://docs.railway.com/config-as-code/reference), [CLI](https://docs.railway.com/cli), [PostgreSQL](https://docs.railway.com/databases/postgresql), [backup and restore](https://docs.railway.com/guides/postgres-backups-restores).

## Final local validation (2026-10-08)

- Frontend: 88/88 PASS, zero skipped or failed; TypeScript/Vite build PASS.
- Go race suite with real local PostgreSQL disposable schemas: 253 passing test/subtest events, 108 top-level tests, zero failures/skips. Twelve PostgreSQL-backed top-level tests cover building/resource/life/identity/restart/schema, environment isolation, exclusive world ownership, connection/lock loss, active websocket shutdown and final-state handoff. `go vet ./...` PASS.
- Linux amd64 static server compilation with CGO disabled PASS. Docker image build remains NOT RUN; no local Docker engine exists and Railway authorization is pending.
- Actual compiled Go executable on a temporary port, canonical staging config and a generated isolated PostgreSQL schema: health reported database connected/environment staging/commit f3fc6fd. All 18 page/config/static-resource requests returned 200, including SPA fallback. Optimized browser identity gate loaded; zero captured warning/error logs. No Vite dev server was used for this check. The temporary server stopped gracefully and its own test schema was removed; the existing localhost wallet server was untouched.
- Production gate automated regression PASS: staging config ignored outside canonical staging, reserved asset excluded from normal production allowlist, wrong-owner/unconfirmed/provider-error paths fail closed, cross-environment sessions/identities rejected by REST and websocket authority. `.fb` resolver implementation unchanged.
- Root and worker test logs, local browser screenshot, static HTTP result and task reviews are in the external evidence directory. They do not establish public TLS/WSS or real provider acceptance.

The local optimized dist contains 15 files totalling 4,876,648 bytes. Largest files: cottage GLB 1,677,316 bytes; Three.js bundle 1,508,694 bytes (Vite gzip estimate 487.83 kB); inventory atlas 525,176 bytes; neighbors atlas 442,242 bytes; pasture art 339,330 bytes. Actual public transfer encoding, time to world, API/WSS latency and FPS remain NOT RUN.

## Compatibility and remaining acceptance

Pre-environment identity records remain stored but fail closed. Reauthentication and selection create an explicitly scoped identity; old world memberships are not automatically migrated. No database rows or saves were deleted by implementation. A newly provisioned staging database starts with its own identities/worlds.

Final review corrections set explicit Railway draining, add PostgreSQL exclusive world-writer protection, and cover connected-websocket shutdown and ownership loss. Final race/PostgreSQL tests pass; actual cloud handover remains unverified. Actual public WSS/redeploy/persistence is still mandatory before closed-alpha readiness. Railway proxy pooling may share the socket-address rate limit between users and must be observed in public multi-client acceptance.

**NOT READY FOR CLOSED ALPHA:** Railway Dashboard/CLI authorization is outstanding. Project/services/URL/Docker image, public identity/gameplay, app redeploy persistence, backups, current plan/cost and public performance have not been verified. The next operator step is completing authorized Railway login, then deploying this tested branch and executing the public matrix above. Production launch remains closed.

## Mandatory staging maintenance deployment procedure

The app owns authoritative rooms in process memory. **One configured replica does not make rolling replacement safe:** the new process can load a room before the old process completes its final save. Do not use normal rolling redeploy or GitHub auto-deploy for this staging service. Keep its GitHub source/autodeploy disconnected, use controlled CLI uploads, and disable additional deployment automation. No second Go writer may run against this database during planned maintenance.

1. Verify the exact FreeTower-Staging project, staging environment, application service and PostgreSQL service identifiers. Complete all local test/build gates before service interruption. Capture a sanitized persistence checkpoint.
2. Stop only the current **application deployment**. Leave PostgreSQL and its volume running. Wait until Railway shows that app deployment Removed/stopped and its process has completed shutdown; a submitted stop request alone is not evidence. Do not delete the service, project, environment, database or volume.
3. Only after the old application is stopped, upload/start the tested app revision. Its room load now follows completion of the previous process's final writes. This staging procedure deliberately accepts maintenance downtime, including build time.
4. Verify health/environment/commit/database, reauthenticate the real wallet, reconnect WSS, and compare the saved world/identity checkpoint. Record the stop/start deployment IDs and PostgreSQL service/volume IDs unchanged.

`railway.json` supplies a 120-second draining window and zero additional overlap. The 120 seconds allows the 15-second HTTP drain plus normal staging ticker/client/final-save work; it is not a proof of shutdown under arbitrary room counts or a stalled database. Confirm the effective setting in Railway and measure actual active-client shutdown. The [Railway teardown reference](https://docs.railway.com/deployments/deployment-teardown) and [deployment reference](https://docs.railway.com/deployments/reference) explain that the default termination grace is zero.

**Exclusive world writer:** public startup acquires a PostgreSQL advisory lock scoped to the database/schema before loading rooms. The physical connection holding that lock also executes every world save/delete; it is serialized and never falls back to the connection pool. A concurrent app cannot acquire world ownership and fails startup. The owner holds its lock through shutdown final saves and releases it when the store closes. Loss of the connection or actual lock fails world access closed, stops simulation and closes clients; health also checks this ownership.

Railway can initiate infrastructure replacements itself. A replacement that overlaps the existing owner now fails startup instead of becoming a second durable world writer. This prioritizes state safety over seamless availability; provider-managed replacement/retry behavior remains a live acceptance check, and controlled stop/drain/start is still the supported staging deployment procedure. One replica or zero overlap alone is not the safety mechanism. No scaling/distributed room execution is introduced.

## Exact file inventory

Created (10):

- `railway.json`
- `server/identity_staging_test.go`
- `server/runtime_config.go`
- `server/runtime_config_test.go`
- `server/runtime_shutdown_test.go`
- `server/world_writer.go`
- `server/world_writer_test.go`
- `docs/freetower/FT-WEB-006-STAGING-DEPLOYMENT.md`
- `docs/freetower/STAGING_BACKUP.md`
- `docs/superpowers/plans/2026-10-08-freetower-staging.md`

Modified (17):

- `.dockerignore`
- `Dockerfile`
- `server/auth.go`
- `server/http.go`
- `server/identity.env.example`
- `server/identity_http.go`
- `server/identity_model.go`
- `server/identity_providers.go`
- `server/identity_selection.go`
- `server/identity_store.go`
- `server/main.go`
- `server/store.go`
- `server/websocket.go`
- `src/identity/IdentityGate.tsx`
- `src/identity/model.ts`
- `src/identity/types.ts`
- `tests/identity.test.ts`

## Final integrity checks

Scoped secret scan PASS: tracked/untracked nonignored source, optimized dist, stage diff and textual stage evidence contain neither the configured provider secret nor the current real full wallet address; private-key marker scan also clean. Values are compared only in memory and never printed or saved by the scanner. This is not an audit of all historical Git credentials. `git diff --check` PASS. No cloud credentials or database dump were committed.

The PostgreSQL endpoint must preserve a physical session for session advisory locks: use Railway's direct PostgreSQL connection, not transaction/statement-pooling middleware. Every app that writes this database must run the guarded public-mode build; do not attach development/legacy binaries or roll back to pre-guard code against this staging data. A failed owner is replaced through the controlled deployment procedure, never silently reconnected. Final scoped code review is APPROVED after these guard/drain corrections. The compiled-server smoke was rerun on f3fc6fd: database connected, environment staging, all 18 requests 200; browser warning/error log empty; clean termination and own-schema cleanup confirmed.
