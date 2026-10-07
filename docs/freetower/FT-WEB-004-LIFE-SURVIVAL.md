# FT-WEB-004 — Life Survival

> For agentic workers: use subagent-driven-development with scoped ownership and independent review. The supplied execution specification authorizes implementation and one final feature commit/push; no intermediate commits or renewed approval gates.

Goal: complete authoritative fishing, free farm plots, light survival and chicken/egg loops with personal items, multiplayer and persistence.
Architecture: shared configuration and typed contracts; Go room-serialized atomic mutations and centralized simulation; dedicated TS controllers/domain modules and asset renderers. Existing React/R3F/Go stack, no dependencies or external assets.
Spec: /Users/btkkgo/.codex/attachments/f743b651-fd04-4cc5-bc2b-5a67f53864f4/已粘贴的文本.txt
Baseline: e68690e728f69320032b6ed04113eb936a84c443
Branch: codex/ft-web-004-life-survival

## Global constraints
Only /Users/btkkgo/Documents/FreeTower-Web. No blockchain, combat, cooking, markets, breeding or FT-WEB-005. Placeholder assets. Exact-stage final commit `feat: add FreeTower fishing farming and animals`; push feature branch only, no main merge. Keep previous rooms/progress. Existing safe PostgreSQL disposable schemas only.

InventoryOwner: shared resources remain wood/stone; personal items and survival belong to each account's character in the current world (roomId + userId), never pooled or exposed in other players' snapshots. This first world-character ownership scope deliberately does not introduce cross-world transfers/economy. Initial personal starter grant occurs once per character and persists. Private personal state is sent only to that user's sockets; public state contains plots, animal state and visible fishing sessions, no receipts/personal inventory.

## Shared wire contract
Requests `{type:"life-action",requestId,action,targetId?,itemId?,position?:{x,z},reeling?:boolean}`. Actions: `cast`, `reel`, `cancel-fishing`, `till`, `plant`, `water`, `harvest`, `feed`, `collect-egg`, `eat`, `drink`.
Replies `{type:"life-result",requestId,message}` or `{type:"life-error",requestId,code,message}`; private snapshot `{type:"life-self",personal,serverTime}`. Existing public `state.freeTower.life` holds `{plots,animals,fishing}`. Every map keyed by instance id, fishing keyed by userId. Personal `{owner:{kind:"player",roomId,userId},inventory:Record<string,number>,survival:{health,hunger,thirst,energy,updatedAt}}`.
Fishing session `{id,userId,spotId,fishId,state,position:{x,z},origin:{x,z},startedAt,biteAt,expiresAt,distance,tension,reeling,updatedAt}`. State `casting|waiting|fish-noticed|nibbling|hooked|reeling|caught|escaped|cancelled`; aiming/idle local intent only. Fish AI presentation maps these to roam/notice/approach/nibble/bite/flee. Public final cue retained briefly; reward only private owner. Reconnect/restart cancels active fishing safely without refund/duplicate rewards, terminal receipts persist. Inputs set reel intent; only server elapsed time advances distance/tension.
Plot `{id,buildingId,state,cropId,plantedAt,readyAt,wateredAt,growthModifier}`; states empty/tilled/planted/watered/growing/ready; cropId empty when no crop. Farm plot definition ID `farm-plot`, asset `building.farm-plot.basic`, 3x3, wood4/stone2,3s; footprint walkable but placement reserve remains. Completed plot created lazily/centrally. Till needs active `basic-hoe`; water needs `basic-watering-can`; seed wheat-seed/carrot-seed; harvest returns tilled. Growth starts after watering,45/60s; authoritative timestamps.
Animal `{id,definitionId,position:{x,z},home:{x,z},target:{x,z},state,fedAt,nextEggAt,eggReady,eggPosition?:{x,z},updatedAt}`; definition chicken; AI idle/wander/peck/approach-food/eat. Four chickens near camp with bounded configurable habitat; server1Hz obstacle/water-aware movement, persists positions, no restart reset. Feed chicken-feed, one egg per feeding after35s, independent ground eggPosition remains stable until collection. Feeding while egg pending/ready rejected. Range3.5m interactions.
Config server/freetower_life.json provides fish, spots (reuse silver-river/ancient-sea WaterZone), crops, starter items (bait20,wheat-seed5,carrot-seed5,chicken-feed5,carrot3,water-flask5), mild perminute decay hunger .6 thirst .8, energy regeneration2; no sprint gating/health death. Fish IDs river-minnow,river-carp,coast-mackerel,coast-snapper; asset fish.<id>. Each includes habitat,rarity,biteDelayRange,fightStrength,baseValue. Cast active basic-fishing-rod,land,range2–12m,facing cone <=100deg, water spot and segment building obstruction. Fish has initial distance12; reel about2.4m/s,tension18/s relaxed24/s,max100; manual hold/release minigame. Server deterministic injectable clock/random for tests.

## Review focus
- Same request after lost ack/reconnect/save failure must not double reward; pending client retries same identity.
- Public snapshots must never expose other characters' inventory/survival or persistent receipts.
- Live fishing cancels on swimming/tool change/moving away; reel frequency cannot accelerate server simulation.
- Growth/egg deadlines survive shutdown; saving failure does not consume items or advance published state.
- Moving a farm plot preserves linked crop and visual position; animal collisions include newly built obstacles.

### Task 1: Server simulation and persistence
Ownership: all server Go files and server/freetower_life.json. Root owns existing resources/buildings JSON and all frontend.
- [x] Write failing life domain and authority tests covering all 40 requested domain cases, atomic rollback, private serialization, replay/conflicts, reconnect/restart, plot move and animal obstacle avoidance.
- [x] Implement shared wire contract exactly above in focused life/fishing/farming/survival/animals Go modules; hook join/action/ticker/startup/disconnect. Existing clone/publicState must preserve private durable data internally and scrub externally.
- [x] Extend tool ownership checks using root-added resource definitions and private personal items. Prevent gathering/building/fishing mode conflicts server-side. Farm plot walkable player collision only; building overlap still reserved.
- [x] Run Go race/vet and safe isolated PostgreSQL test; write task report. No commit/push, no child agents, no UI.

### Task 2: Frontend domains, assets and integration (root)
Ownership: src, tests/life.test.ts, existing resources/buildings JSON.
- [x] RED tests for cast validation, water/angle/building limits, stable pending identity, reel intent/cancel, crop phase/time and inventory separation.
- [x] Implement life controller and distinct fishing/farming/survival/animals typed models; server-only rewards/state. Extend existing hand tools/wheel, no second system.
- [x] Implement compact personal inventory and context actions, survival bars, fishing aiming/bait/reel/tension HUD; scene line/bobber/fish cues/crops/chickens/eggs via AssetId. Shared render updates, no per-object expensive loops.
- [x] Connect world input, actual water clicks, farming/chicken selection, movement cancellation, building conflicts, private WS state and network-rate debug.
- [x] All frontend tests/build pass; task review after completion.

### Task 3: Review, acceptance and delivery
- [x] Independent whole-change review; fix confirmed defects and scoped re-review.
- [x] Real two-session48-item browser acceptance; safe local test room setup documented, no appstate injection to satisfy gameplay. Actual restart while growth/egg pending, reload/relogin personal independence, regression gathering/building/move/swim/chat.
- [x] Record screenshot/log/performance/network evidence externally; all frontend/build/Go race/vet/PG and diff checks.
- [x] Finalize documentation and known limits; exact-stage commit/push and verify remote SHA/clean; strict68-field report, stop before FT-WEB-005.


## Delivered implementation and authority

Go modules `life.go`, `life_fishing.go`, `life_farming.go`, `life_animals.go` own authoritative domain state. The frontend `life` controller/presentation orchestrates typed `fishing`, `farming`, `survival`, and `animals` modules. Definition JSON is shared, and gameplay uses IDs rather than meshes. `onFishingCue` is an optional audio-adapter hook for bite/catch/escape transitions; no audio asset is shipped.

Fishing uses a 1 Hz room simulation; input only changes reel intent. A full catch requires alternating reel/release. A missed hook escapes after eight seconds, high tension also escapes. Catch adds a raw fish item only to its owner. Active sessions cancel safely on disconnect/restart; restarting does not resume a fight or refund consumed bait. New casts clear prior reel intent. Four fish definitions use configurable rarity, delay and strength.

Room-serialized clone → mutate → save → publish protects every reward/consumption transaction. Failed saves leave original state unchanged. Durable request receipts include player and payload identity, so successful replays cannot double rewards. The client retains the original request on persistence/rate-limit errors. Changed life actions have a 250 ms per-player bound; unchanged reel intent is cached in a bounded 128-entry ephemeral receipt map. Durable replay resync/acks are requester-only and bounded to five per second per socket. Release/cancel remain available for safety. This limits input amplification; it is not a claim of load-tested production scalability.

Personal state belongs to `(roomId, userId)` and is persisted separately from shared wood/stone. Public HTTP/WS snapshots strip Personal and LifeReceipts. Only the owner's socket receives `life-self`; neither UI nor direct client messages set rewards/survival. Moving a farm plot preserves its ID, crop and deadline. Ground egg position stays fixed even when its chicken moves.

Health remains 100 without combat; hunger/thirst decay 0.6/0.8 per minute. Carrot restores hunger and Water Flask restores thirst, with item consumption. Energy has authoritative regeneration/interface only; sprint is not gated. All are clamped to 0–100.

## Assets and performance

Asset registry: `tool.fishing-rod.basic`, `tool.hoe.basic`, `tool.watering-can.basic`, `building.farm-plot.basic`, four `fish.<id>`, `crop.wheat.stage.{seed,sprout,growing,ready}`, `crop.carrot.stage.{seed,sprout,growing,ready}`, `animal.chicken.basic`, `item.egg.basic`. Original procedural placeholders, no downloaded assets/dependencies, no KayKit import. Replace registry renderers without changing domain logic.

One central client frame update interpolates animals/fishing visuals; geometry/materials are shared. Low-rate scene/HUD updates derive server time; no per-animal expensive React loop. On this Mac, two IAB viewports (1611×1134 and 1280×720), representative observed FPS 108–178, draw calls 64–186, triangles 92,010–96,480; final camp view ~124 FPS / 174 calls / 94,276 triangles. These are observed debug samples, not percentile benchmarks. WS lifetime averages were about 3.1–3.6 inbound messages/s and 0.0–0.2 outbound/s per tab (movement/action bursts averaged over session). Burst limiting is verified separately by server tests.

## Validation — 2026-10-07

- Frontend: **54/54 PASS**, 0 skipped, including ten new life tests and 44 baseline tests.
- `npm run build`: **PASS**, TypeScript + Vite production build.
- `go test -race -count=1 -json ./...`: **PASS**, 60 top-level tests / 112 total test results including subtests, 0 failed/skipped, 22.559 s.
- PostgreSQL: **5/5 PASS**, isolated test schemas in the existing local FT test database. Includes life persistence, building, gathering, original persistence and account schema migration. No production database touched.
- `go vet ./...`: **PASS**.
- Independent code review: four initial findings corrected (cast spot wire, replay/write amplification, retry identity, E-feed item); final scoped reviews PASS, plus post-acceptance small changes reviewed. Reviewer did not independently rerun full suites.
- Browser: **48/48 observed PASS**, two separate signed-in sessions on localhost / 127.0.0.1. Agent-driven real UI acceptance, not a substitute for the user's final visual acceptance. Deep-water animal avoidance is bounded natural-habitat observation plus automated boundary tests, not a forced browser edge case.
- Actual backend stop/start while wheat was growing preserved plantedAt/wateredAt/readyAt. Wheat became ready after the original deadline. Existing fed chicken egg state and fixed eggPosition persisted; personal items survived. The restart snapshots show post-start animal movement, not unchanged coordinates.
- Local QA fixture: a new room created through UI, shared wood/stone seeded to 100/100 with backend stopped; unrelated saved-room hashes checked unchanged during setup. Personal items came from the normal once-per-character starter grant. No UI runtime injection used for gameplay acceptance.
- Browser console warning/error capture after final refresh/HMR: both empty; no fatal or asset-404 errors observed.

### Browser acceptance matrix

| # | Check | Result / evidence |
|---|---|---|
| 1 | Equip Fishing Rod | PASS — 13,14,17; existing tool wheel and held rod |
| 2 | Approach river | PASS — 17; physically walked to Silver River |
| 3 | Cast | PASS — 13,17; legal water clicks, bait 20→16 across four casts |
| 4 | Float visible | PASS — 14,17-casting; rod/line/bobber rendered |
| 5 | Fish reacts | PASS — 17-fish-noticed; fish approach cue |
| 6 | Bite occurs | PASS — 17-hooked |
| 7 | Reel interaction works | PASS — river-reel-trace.json; reel 0 → release 65 → reel 17 |
| 8 | Catch fish | PASS — 15,17-caught; coastal mackerel and river carp |
| 9 | Inventory receives fish | PASS — 16,20,30; one of each fish, player A only |
| 10 | B sees A fishing | PASS — 14,18; B saw line/float and Caught river-carp |
| 11 | Ocean/coast fishing works | PASS — 13–16; actual coastal catch |
| 12 | Swimming blocks fishing | PASS — 19; swimming + Return to dry land, rod hidden |
| 13 | Build Farm Plot | PASS — 04; free plot construction and wood/stone debit |
| 14 | Equip Hoe | PASS — 05; existing wheel Basic Hoe |
| 15 | Till | PASS — 05; E empty→tilled before planting |
| 16 | Choose Wheat Seed | PASS — 05; Wheat Seed selected |
| 17 | Plant | PASS — 05; seed consumed once |
| 18 | Equip Watering Can | PASS — 06; existing wheel Watering Can |
| 19 | Water | PASS — 06; wheat watered and 45s deadline |
| 20 | Crop visually grows | PASS — 05–07; planted/green/golden stages |
| 21 | Reload does not reset | PASS — before-restart.json / after-restart.json; unchanged crop deadline; browser refreshed |
| 22 | Crop becomes ready | PASS — 07; ready after actual backend shutdown/restart |
| 23 | Harvest | PASS — 09,10; +3 wheat and plot returned tilled |
| 24 | B sees same crop states | PASS — 07,09,29; B saw growing/ready/harvest and moved carrot plot |
| 25 | Health/Hunger/Thirst/Energy visible | PASS — 01,10,31; four compact bars |
| 26 | Hunger changes | PASS — 01→10→30; elapsed-time hunger decline |
| 27 | Eat Carrot | PASS — 10; Eat Carrot button |
| 28 | Hunger restores | PASS — 10; A hunger restored to 100 |
| 29 | Survival values remain after reload | PASS — 30; refresh kept fish/items/survival, no starter reset |
| 30 | A/B values independent | PASS — 08,10,30; B starters unchanged, A food/catches distinct |
| 31 | Chickens visible | PASS — 01; four chickens |
| 32 | Chickens wander | PASS — 01,02,24,31; authoritative changing positions |
| 33 | Chicken does not walk into deep water | PASS — Observed camp habitat remained on dry land throughout session; deep-water edge/obstacle stress additionally covered by Go test, not forced in browser |
| 34 | Feed Chicken | PASS — 02 E feed; 24 second feed visible to B |
| 35 | Egg becomes available | PASS — 11,24; 35s deadline then ground egg |
| 36 | Collect Egg | PASS — 11→12; egg vanished for B, A inventory Egg=1 |
| 37 | B sees feed/egg changes | PASS — 24; B eat + Egg 35s, 11→12 ground egg removal |
| 38 | Server restart retains state | PASS — before/after restart; fedAt,nextEggAt,eggReady,eggPosition preserved; crop deadline preserved |
| 39 | Chop Tree works | PASS — 22; tree depleted, shared wood 96→100 |
| 40 | Mine Rock works | PASS — 23; rock depleted, shared stone 98→101 |
| 41 | Build Campfire works | PASS — 25; wood100→96/stone101→98, 5s construction |
| 42 | Move Building works | PASS — 26,27; campfire moved; 28,29 mature carrot plot moved preserving readyAt |
| 43 | Swimming works | PASS — 19; river swimming and return to land |
| 44 | Q hand switch works | PASS — 21; active right rod→left pickaxe |
| 45 | Chat works | PASS — 14,18,22; B test message displayed on A |
| 46 | Two players remain synchronized | PASS — 18,24,27,29; same public state and world changes |
| 47 | Console has no fatal error | PASS — console-A.json,console-B.json: [] after final refresh/HMR |
| 48 | No asset 404 | PASS — No asset-load/404 errors observed; all new visuals procedural, build resolves bundled imports |


## Evidence and limitations

Evidence is local outside Git: `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-004`. It includes screenshots, reel traces, sanitized restart snapshots, browser matrix, console logs, full test outputs and independent review. `03-feed-B.png` selected another chicken and is not direct feed proof; use `24-B-sees-feed-timer.png`. Final source excludes test credentials, room save data, artifacts, caches and dependencies.

Limitations: placeholders and visual/audio interfaces only (no shipped sound); no combat/death/cooking/breeding/economy. Raw fish is inventory-only. Watering can capacity/refill omitted. Four bounded chickens use simple collision-aware movement, not NavMesh. Personal items are world-character scoped; cross-world transfer is not implemented. Wood/stone remain shared compatibility resources. Active fishing cancels on reconnect; it does not resume a fight. The small context panel can obscure part of near water; rotate/zoom or choose an unobstructed surface. Test-time crop/egg durations are configurable. Server tick persists world changes at 1 Hz; database throughput at large population remains unbenchmarked. Desktop-only acceptance; mobile and human art acceptance remain outside this task.

FT-WEB-005 recommendation (NOT STARTED): after user acceptance, Fractal Bitcoin wallet signature + Ordinals character + .btc name + character identity, replacing username/password. No blockchain code changed in FT-WEB-004.

## Exact file inventory

Created (19):

- `docs/freetower/FT-WEB-004-LIFE-SURVIVAL.md`
- `server/freetower_life.json`
- `server/life.go`
- `server/life_animals.go`
- `server/life_farming.go`
- `server/life_fishing.go`
- `server/life_postgres_test.go`
- `server/life_test.go`
- `src/world/freetower/animals/model.ts`
- `src/world/freetower/farming/model.ts`
- `src/world/freetower/fishing/model.ts`
- `src/world/freetower/life/LifeAssets.tsx`
- `src/world/freetower/life/LifeHUD.tsx`
- `src/world/freetower/life/LifeScene.tsx`
- `src/world/freetower/life/controller.ts`
- `src/world/freetower/life/life.css`
- `src/world/freetower/life/types.ts`
- `src/world/freetower/survival/model.ts`
- `tests/life.test.ts`

Modified (17):

- `server/building.go`
- `server/building_validation.go`
- `server/freetower_buildings.json`
- `server/freetower_resources.json`
- `server/gathering.go`
- `server/http.go`
- `server/websocket.go`
- `src/game/types.ts`
- `src/world/freetower/Explorer.tsx`
- `src/world/freetower/FreeTowerApp.tsx`
- `src/world/freetower/World.tsx`
- `src/world/freetower/building/BuildingAssets.tsx`
- `src/world/freetower/building/BuildingHUD.tsx`
- `src/world/freetower/building/validation.ts`
- `src/world/freetower/gathering/Assets.tsx`
- `src/world/freetower/gathering/GatheringHUD.tsx`
- `src/world/freetower/gathering/model.ts`
