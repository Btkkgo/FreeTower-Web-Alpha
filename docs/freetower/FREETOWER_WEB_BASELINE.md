# FreeTower Web baseline — FT-WEB-000

Recorded on 2026-10-07 (Asia/Shanghai). This establishes an unchanged Little Flock engineering baseline for **FreeTower**. It does not implement the FreeTower game, identity system, map, art, or UI.

## Provenance and scope

| Field | Verified value |
| --- | --- |
| GitHub account / fork owner | `Btkkgo` |
| Upstream | https://github.com/songkeys/little-flock |
| Upstream commit | `eaaa56184ad8f1b513ae81960075846663803886` |
| Fork | https://github.com/Btkkgo/FreeTower-Web-Alpha |
| Fork date | 2026-10-07 |
| GitHub fork relationship | `isFork: true`, parent `songkeys/little-flock` |
| Default branch / visibility | `main` / PUBLIC |
| Working branch | `codex/freetower-web-alpha` |
| Local checkout | `/Users/btkkgo/Documents/FreeTower-Web` |
| `origin` | `https://github.com/Btkkgo/FreeTower-Web-Alpha.git` |
| `upstream` | `https://github.com/songkeys/little-flock.git` |

GitHub already contained `Btkkgo/Freetower` (names are case-insensitive), so the authorized fallback repository name was used. The existing repository was not overwritten. The old project, Godot content, and old Web prototype were not migrated or edited. Only this baseline document is intended for this branch's commit. The upstream package name, branding, artwork, gameplay, server, dependency versions, and lockfiles remain unchanged.

## Licenses and attribution

- [LICENSE](../../LICENSE): MIT, Copyright (c) 2026 Little Flock contributors. The complete copyright and license remain intact.
- [ASSETS.md](../../ASSETS.md): artwork in `public/art/`, cottage model in `public/models/`, and demo media use [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Credit **Little Flock contributors**, link to the [upstream repository](https://github.com/songkeys/little-flock) and license, and indicate changes when adapting them. They are not claimed as FreeTower originals.
- The upstream attribution describes OpenAI-generated imagery, a Tripo Studio cottage optimized in Blender, and HyperFrames-edited game capture. This records the upstream declaration; it is not an independent audit of the generation accounts or provider contracts.
- Procedural sheep, shepherds, plants, landscape, and synthesized audio are source code under MIT. Noto Sans SC retains SIL OFL; dependencies retain their own licenses.
- Neither license file was changed. SHA-256: `LICENSE` = `1888766c9735a965849d5c77b73c4cd58d14736d709e8803c8c1b0f94fd9cb92`; `ASSETS.md` = `8bd44a61a4674ac01515521da96061b88a47cff09a328c885d23099f52427b5e`.

## Engineering structure and actual stack

Versions below are resolved versions checked against `package-lock.json` and installed packages, with backend dependencies from `server/go.mod`; manifest ranges are not reported as installed versions.

| Frontend component | Resolved version |
| --- | --- |
| React / React DOM | 19.2.8 / 19.2.8 |
| TypeScript | 5.9.3 |
| Three.js | 0.180.0 (r180) |
| React Three Fiber | 9.7.0 |
| Drei | 10.7.8 |
| `@react-three/postprocessing` | 3.1.1 |
| `postprocessing` | 6.39.4 |
| Vite | 7.3.6 |
| Zustand | 5.0.15; declared/installed, no direct import found in `src/` |

| Path | Responsibility observed in source |
| --- | --- |
| `src/` | React entry, App state, input, API/socket lifecycle and UI composition |
| `src/world/` | R3F scene, Three.js geometry, shepherd/sheep, cottage, garden, regions, atmosphere and seasons |
| `src/ui/` | Welcome/auth, avatar editor, panels, story guide, maps and item artwork |
| `src/game/` | Types, catalog, progression, regions, appearance, API client and synthesized audio |
| `server/` | Go HTTP/REST, auth, rooms, authoritative game actions, WebSocket, file/PostgreSQL stores and tests |
| `public/` | Static artwork/model assets |
| `docs/` | Upstream demo media plus this baseline record |

Backend: Go standard-library HTTP routing and JSON REST, `github.com/gorilla/websocket v1.5.3`, PostgreSQL via `github.com/jackc/pgx/v5 v5.7.6`, and `golang.org/x/crypto v0.38.0`. `go.mod` declares Go 1.24.0; verification ran with **go1.27.1 darwin/arm64**. Node **v24.19.0** and npm **10.9.4** were used for installation/build/dev. The shell's default Node was v22.22.0, so the already-installed Node 24 runtime was placed first in PATH for these commands; no global runtime or dependency upgrade was performed.

## Database and realtime architecture

`server/main.go` opens the store using `DATABASE_URL`; when unset, `SAVE_PATH` defaults to `data/farm.json` relative to the server working directory. The file store persists accounts, sessions, room membership and farm state, writing a temporary file with mode 0600 and renaming it. It is not PostgreSQL by default.

The running browser baseline used **file storage** at `server/data/farm.json`. Existing local PostgreSQL **16.13 (Homebrew)** was used only for an additional isolated integration-test database, `freetower_ftweb000_20261007`. No existing database was reused for test data, and no PostgreSQL installation or global configuration change was needed. This test database is retained locally; it is not part of Git. A full browser run with the server switched to PostgreSQL was NOT RUN.

`vite.config.ts` serves port 4318 and proxies `/api` (including WebSocket) to `http://localhost:8080`. REST endpoints cover health, register/login/logout/me, profile, rooms/join and room actions. The original username/password system remains in place: bcrypt password hashes, HTTP-only session cookies, and hashed persisted session tokens.

The browser connects to `/api/rooms/{id}/ws`. The server checks authentication, room membership and origin. JSON messages carry presence, state, movement, jump, chat, events and whistle activity. Game actions are validated server-side and durable farm changes are saved before state broadcast. Coins, inventory, sheep, crops, buildings and progression are shared at farm level; chat and live player positions are ephemeral. The browser has reconnect logic and snapshot ordering. The source's 24-connections-per-room limit is a configured cap, **not** a measured capacity or scalability guarantee.

## Reproduce locally

Use Node 24 and a compatible Go toolchain. From the checkout:

```sh
npm ci
npm run dev
```

In a separate terminal:

```sh
cd server
env -u DATABASE_URL -u TEST_DATABASE_URL go run .
```

Open http://localhost:4318/ . For the upstream scene statistics overlay, use http://localhost:4318/?debug=1 . Backend health: http://localhost:8080/api/health ; the same endpoint through the Vite proxy is http://localhost:4318/api/health . The upstream dev/server listeners were not changed; this is a local development baseline, not a deployment.

```sh
npm run build
cd server
env -u DATABASE_URL -u TEST_DATABASE_URL go test -race ./...
go vet ./...
```

For optional PostgreSQL tests, set `TEST_DATABASE_URL` to a dedicated local test database before `go test -race -count=1 ./...`. Do not point tests at a production database. Local account passwords and session data are excluded from this document and Git.

## Verification results

PASS means an observed result for the stated scope, not coverage of every game feature. Browser A was Codex's in-app browser; browser B was a real Google Chrome incognito window with separate cookies and a different local test account. Testing used actual UI input, not injected game state. Two test farms were retained: the initial farm for crops/inventory and a fresh farm named `FreeTower FT-WEB-000` for building synchronization.

| Check | Result and evidence |
| --- | --- |
| `npm ci` | PASS; 137 packages added, lockfile unchanged; audit caveat below |
| Frontend / backend / Vite proxy | PASS; Vite 4318, Go 8080, proxied `/api/health` returned `ok:true` |
| Registration / login | PASS; two local accounts registered; A explicitly logged out, logged back in with its original password, and re-entered the saved farm |
| Create / join / leave / re-enter farm | PASS; A created the named test farm, B joined by invite code, both appeared online |
| World / player / models / UI | PASS; rendered cottage, trees, terrain, sheep, crops and players; no white screen; audio initialization did not prevent startup |
| Keyboard movement | PASS for sampled W/A/S/D inputs and observed displacement; not a collision/pathfinding exhaustiveness test |
| Click-to-move | PASS; observed travel on ground and to an original tutorial target; remote avatar moved from crop area toward the riverside fence |
| Jump | PASS for Space and visible raised avatar above its ground ring; full remote animation timing not measured |
| Zoom | PASS for wheel input with changed framing; no quantitative camera-distance measurement |
| Camera rotation | NOT RUN; source maps right-drag to OrbitControls, but the available UI drag API did not expose a right-button drag. Manual acceptance remains required |
| Sheep | PASS; visible cloud sheep, pet raised happiness to 100; shearing reset wool to 0 and awarded three wool |
| Farming / crops | PASS; carrot sowing consumed a seed, watering started growth, maturity appeared; two mature clover plots were harvested through the UI |
| Inventory / economy | PASS; shared carrot seeds 3→2; clover seed purchase 0→1 and coins 70→64; clover harvest and a neighbor order updated shared inventory/progression, order raised coins to 134 |
| Building | PASS for original fixed-location shelter; fresh test farm changed from build-shelter objective to adopt-first-sheep objective and cottage appeared in both browsers. Arbitrary placement/moving buildings is not claimed |
| Regions | PARTIAL; map showed home/meadow discovered and forest/highland locked; navigation to meadow entrance was observed. Complete regional gameplay/unlock chain was not completed |
| Day/night | PASS; natural clock advanced from daytime to 18:56 dusk and 19:17+ night, with lighting and house lamps changing; no clock manipulation |
| Chat UI | PASS; A→B and B→A distinct test messages visible on both sides |
| Save / page reload | PASS; automatic save, refresh and re-entry preserved the sampled farm |
| Server restart | PASS; graceful TERM of this task's Go server, clean exit, `go run .` restart, automatic reconnection and browser refresh succeeded |
| Durable-state comparison | PASS; pre/post restart snapshots of both farms matched for names, membership counts, buildings, inventory, coins, XP, level, plots, stats, regions and progression. Volatile clock/sheep animation were excluded intentionally |
| `npm run build` | PASS; TypeScript build plus Vite, 2,172 modules; Vite build 2.84 s |
| `go test -race` without PostgreSQL | PASS: 12 passed, 2 PostgreSQL tests skipped, 0 failed; package 10.598 s |
| `go test -race -count=1` with isolated PostgreSQL | PASS: 14 passed, 0 skipped, 0 failed; package 16.852 s, including persistence and original-schema avatar migration tests |
| `go vet ./...` | PASS; exit 0, no diagnostics |

### Multiplayer scope

Both independent browser sessions simultaneously displayed a count of two and the other account's avatar. A's ground movement was visible in B. Bidirectional chat worked. A's clover harvest changed B's planted-plot count from 3 to 2 to 1; A's seed purchase updated B's seed count and coins live while B's garden panel was open. B displayed A's watered carrot and remaining growth time. Completing the order updated B's coins, XP and next objective. In the second farm, A built the shelter and B displayed the new house and matching next objective. These are real two-browser checks of presence, position, chat, farm, crop, building and shared inventory synchronization. They do not establish cross-network latency, scale, long-session reliability, all action types, or anti-cheat completeness.

### Persistence scope

After server restart and a browser refresh, the named farm still had its shelter and both members. After explicit logout/login, the original farm still showed one sheep, six plots (one mature carrot), level 2 / 143 XP, 134 coins and the meadow unlocked. The stored comparison also retained clover 6, feed 1, carrot seeds 2, clover seed 1 and cloud wool 6. Account/session raw contents and passwords were not published. Chat history and player positions are not asserted as durable saves.

## Browser and performance baseline

Machine: Apple M4 (`Mac16,10`), 16 GiB RAM, macOS 27.0. Chrome 154.0.8037.98 incognito plus Codex in-app browser, default upstream graphics with `?debug=1`. Screenshots were approximately 1611×1134 for A and 3006×1314 for Chrome's entire window; browser viewport and application window sizes differed. These samples are not a benchmark with controlled thermal/load conditions.

| Measurement | Observation |
| --- | --- |
| Initial-ever navigation | NOT INSTRUMENTED; no invented first-visit time |
| Cache-disabled Chrome reload, local Vite dev server | DOMContentLoaded **578 ms**, Load **1.99 s**, Network Finish **3.36 s**; these events do not measure first interactive 3D frame |
| Network after that reload | **87 requests**, **15.5 MB transferred**, **16.0 MB resources** (DevTools totals, includes development modules/fonts/assets and socket entries) |
| Major observed resources | Drei dev module ~3,707 kB; cottage GLB ~1,678 kB; one dev chunk ~1,283 kB; inventory atlas ~525 kB; neighbors atlas ~443 kB |
| Scene FPS | A initially ~61–67 FPS; with both live 3D browsers, common samples A ~32–40 and Chrome ~22–36 FPS. Brief fresh-scene samples reached ~68 FPS. No fixed-60-FPS claim |
| Draw calls | Sampled ~79–146 depending on farm, day/night, camera and actors, from upstream debug overlay |
| Triangles | Roughly 0.87–1.09 million in sampled scenes |
| JS memory | Chrome DevTools Performance monitor sample **43.8 MB JS heap**; this is not total tab memory, GPU memory, or a leak test |
| Desktop responsiveness | Inputs and menus remained usable; dual-browser 3D rendering had visibly lower frame rate. Not universally smooth at 60 FPS |
| Console | A captured warning/error log was empty before intentional restart. Chrome authenticated reload showed only the React DevTools informational message at default levels. Before reload Chrome had a one-error badge whose original message was not captured, so all-session zero errors is NOT claimed. No observed fatal startup/runtime error |

Production build output, separate from the development network sample: Three chunk **1,488.84 kB / 484.29 kB gzip**, app index **121.67 / 43.79 kB**, World **58.81 / 21.85 kB**, CSS **71.15 / 15.83 kB**. Build succeeds; this task did not optimize bundles or benchmark a production deployment. DevTools cache disabling was restored after measurement.

## Known limitations and follow-up boundaries

1. `npm audit` reports **one high-severity indirect vulnerability** in `source-map-js@1.2.1`, through Vite→PostCSS: [GHSA-68fv-2mgg-jv7q](https://github.com/advisories/GHSA-68fv-2mgg-jv7q), indexed-source-map event-loop denial of service. No `npm audit fix`, package update or lockfile change was made. This is an upstream dependency finding, not a demonstrated exploit in this game.
2. Camera right-drag rotation awaits manual validation. Full regions/progression, mobile/touch, long sessions, WAN behavior, load capacity, production hosting, backup recovery and a browser-on-PostgreSQL deployment were not tested.
3. Performance results are local development observations with two live renderers. The first-ever load was not timed; the reproducible cache-disabled reload is reported instead. The early Chrome console badge remains unclassified.
4. Building is the upstream progression/fixed-location system. Do not infer arbitrary construction, movable buildings, fishing, wallets, Ordinals, or FreeTower-specific mechanics from a baseline PASS.
5. Original package/UI naming is deliberately preserved. **FreeTower** is the product name used for new planning and this document; renaming the game is a future task.

## Available systems and work still required

Observed source and sampled runtime provide React/R3F world rendering; desktop movement/click navigation/jump; orbit camera; original avatar customization; username/password sessions; farm creation/invite membership; shared authoritative farm state; sheep care and wool; crop sow/water/grow/harvest; shared inventory/shop/orders; fixed-location progression buildings; region gates; day/night; chat; WebSocket reconnect; and file/PostgreSQL persistence implementations. Only the checks above have runtime PASS coverage.

FreeTower still needs separately approved work for:

- Fractal Bitcoin Wallet and wallet-signature authentication.
- Ordinals Character Identity and `.btc` Character Name.
- FreeTower Map and FreeTower landmark.
- Resource gathering changes, including the intended gathering loop.
- Building system changes, including placement/movement requirements.
- Fishing changes / a fishing system; none was established by this baseline.
- Dual-hand system and radial wheel.
- FreeTower-specific UI and later original assets.

Recommended **FT-WEB-001**: define and implement the wallet→signature→character identity→`.btc` entry contract in a separate task, including account/farm ownership migration and verifiable acceptance criteria. Do not remove the current login until the replacement and migration behavior are agreed and tested. No wallet SDK, signature request, blockchain call, identity rewrite, map rewrite or art change was performed in FT-WEB-000. Stop here for human acceptance; do not merge this branch into `main` as part of this task.

## Evidence handling

Screenshots, sanitized browser/network observations, pre/post persistence comparisons, npm audit/build logs, and Go JSON test logs were retained in a local **FT-WEB-000** evidence folder outside both repositories. `node_modules`, `dist`, `server/data`, database contents, credentials and screenshots are not committed. The document records test results; it does not embed local credentials or publish browser storage. The historical project's dirty working tree is preserved independently.
