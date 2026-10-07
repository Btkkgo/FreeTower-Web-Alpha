# FT-WEB-003 — Free Building Placement

> For agentic workers: use subagent-driven-development with scoped ownership and independent review. User supplied the execution spec and authorized final feature-branch commit/push; do not add redundant design approval gates or intermediate commits.

Goal: three freely placed buildings, authoritative cost/validation, construction timers, moving with exclusive locks, multiplayer and persistent state.
Architecture: shared JSON definitions; pure TS/Go footprint validation; Go room-serialized save-before-publish mutations; separate frontend building controller, scene/ghost/assets and HUD. Existing gathering and farm state preserved.
Tech stack: current React/Three/R3F/TypeScript and Go, no dependencies/assets added.
Spec: /Users/btkkgo/.codex/attachments/eaf42ac2-4c33-424b-bf34-c8788d426aa4/已粘贴的文本.txt
Baseline: 83b03d308b4aec79e5437e8f36abb6f29df1a380
Branch: codex/ft-web-003-building-placement

## Global constraints and decisions

Only /Users/btkkgo/Documents/FreeTower-Web is product scope. Placeholder art. Room shared wood/stone. No wallet, combat, fishing, farming, NFT, production assets or FT-WEB-004. Exact-stage final commit: feat: add FreeTower free building placement. Push feature branch only.

Shared configuration: server/freetower_buildings.json. Costs wood/stone 4/3,8/2,16/8; construction 5/10/20 s; rotationStep degrees 30 (wire rotationY radians). Range 14 m; lock TTL 30 s; successful mutation rate limit 500 ms; player safety radius 0.65 m. Shelter 5x4, bench 3x2, campfire 2x2 footprints. Rule maxSlope tangent .35, maxHeightDelta .8 m, minElevation .7 m, riverBuffer1.2, coastBuffer2. Protected circles tower(16,340)r32 and ruins(0,-58)r16.

Footprint oriented rectangle SAT for buildings, circle-vs-OBB for resources/protected/player; resources reserved even depleted. Sample footprint at <=1 m spacing, include corners/center. Reject bound, protected, water/river/coast, slope/height delta, overlaps, player distance, inventory. Height center derived by server, ignore client Y. Decorative trees should also block placement using deterministic existing tree generator (not only gatherable nodes). Completed/moving building core footprints block player movement; construction sites reserve placement area.

Wire: BuildingInstance {id,definitionId,position:{x,y,z},rotationY,status:"constructing"|"completed"|"moving",constructionStartedAt,constructionEndsAt,ownerId,moveOwnerId?,moveExpiresAt?}. FarmState.freeTower.buildings?:Record<string,BuildingInstance>; same version domain as gathering. Server-only persistent receipts may be separate map. Requests: place-building {requestId,definitionId,position:{x,z},rotationY}; begin-building-move {requestId,buildingId}; move-building {requestId,buildingId,moveToken,position:{x,z},rotationY}; cancel-building-move {buildingId,moveToken}; renew-building-move {buildingId,moveToken}. Server replies building-result {requestId,buildingId,moveToken?} or building-error {requestId?,code,message}; authoritative state broadcasts for mutations. Server-minted token ties lock to owner/session; other users cannot finish/cancel. Begin is idempotent by same request. Place/move receipts bind payload and user, survive restart; duplicate success replay has no extra charge/build. Move no cost/no construction reset. Invalid/cancel/expired/disconnected move restores original position/status. Original building remains a reserved obstacle while moving; ghost local only. Locks recover to completed on restart. Construction finalization saves before publication.

## Review focus

- Reconnect / dropped acknowledgement: retained request IDs prevent extra charges and orphan locks; test durable replay and stale token rejection.
- Failed save: inventory/buildings/timers/locks remain consistent; test injected store failure.
- Cross-client simultaneous placement/move: serialized overlap/cost and exclusive locks; test both.
- Rotated footprints near shore/resource: matching frontend/server sampling and SAT; test rotated overlaps and thin water crossing.
- Input mode conflict: B/R/E/Q/right-click/chat/blur and swimming must not accidentally gather/place; controller tests plus real browser.

### Task 1: Server authority

Ownership: server/building.go, server/building_validation.go, server/building_test.go; modify server/gathering.go, websocket.go, http.go and related server-only files as necessary. Consume shared config unchanged.
- [x] Add failing tests for all placement rejection classes, rotated footprints, once-only cost, durable idempotency, construction completion/restart, move/cancel/conflict/expiry/disconnect/restart, failed-save rollback, serialization and swimming.
- [x] Implement contracts above with room mutex + cloned state save-before-broadcast. Existing FreeTower resource clone must preserve buildings/receipts.
- [x] Integrate completed building collision with existing server movement validation; test crossing main footprint rejected.
- [x] Run Go race/vet and isolated PostgreSQL integration. Report evidence outside Git, no commit.

### Task 2: Client module and integration

Ownership: root handles src/world/freetower/building/{types,buildingMath,validation,controller}.ts, BuildingAssets/Ghost/Scene/HUD.tsx, CSS, tests/building.test.ts; existing World, FreeTowerApp and wire types get narrow integration.
- [x] RED tests for matching placement geometry, cancellation, rotation, swim block, persistent pending request identity, lock conflict recovery and movement collision.
- [x] Implement pure placement validator using same shared config/rules, controller consumes authoritative snapshots, send requests only (no local inventory/world writes).
- [x] Implement three procedural AssetId renderers, translucent cyan/red ground-following ghost, footprint and scaffolds, shared time readout, small build menu/actions/debug panel.
- [x] Connect actual ground raycast, B/R/Esc/right-click, preserve camera/gameplay keys and chat; suppress gather/navigation clicks while building. Client collision against completed/moving core footprint.
- [x] Run all frontend tests/build; produce independent review package for server/client.

### Task 3: Acceptance and delivery

- [x] Run fresh full frontend/build/Go race/vet/PG checks; address independent review.
- [x] Real two-session browser checks for all 38 requested items, record PASS/FAIL/NEEDS_VALIDATION honestly; screenshots and logs outside Git.
- [x] Verify real restart while constructing and after move; collect performance and console evidence.
- [x] Complete architecture/tests/limits documentation, exact-stage commit/push, verify remote SHA and clean worktree; stop before FT-WEB-004.


## Implemented module map

- `building/types.ts`: shared definitions and typed BuildingInstance/request/reply contracts.
- `building/buildingMath.ts`: Three Y-rotation transforms, rotated rectangles, circle footprint checks and sample generation.
- `building/validation.ts`: pure preview validation and swept player collision matching Go.
- `building/controller.ts`: menu/preview/move intent, stable request IDs, retry/cancel/reconnect and lock recovery; no authoritative world or inventory mutations.
- `building/BuildingScene.tsx`: a single ground-raycast/ghost frame loop, building instances and derived construction labels. After first actual pointer movement the ghost reprojects every frame, including camera/zoom changes. Initial move shows the original position until pointer movement/deliberate ground click. No-ground rays clear the preview.
- `building/BuildingAssets.tsx`: AssetId registry for original procedural fire, bench, shelter and scaffold variants. Transparent preview material and footprint show current validity; gameplay imports no meshes.
- `building/BuildingHUD.tsx` and CSS: compact B menu with configured costs/times, R rotation, Place, Cancel/Escape/right-click, selected-building Move and debug-only readout.
- `server/building.go`: authoritative commands, durable receipts, atomic save/publication, construction finalization, exclusive expiring socket-bound move locks.
- `server/building_validation.go`: terrain and footprint rules, canonical resource reservation, movement sweeps.
- `server/building_test.go`, `tests/building.test.ts`: regression/authority/persistence/input/math coverage.
- Existing World/FreeTowerApp/wire types integrate these modules; gathering pauses during building mode and its existing inventory/state remains compatible.

BuildingInstance persists id, definitionId, server-derived XYZ, rotationY, status, constructionStartedAt, constructionEndsAt and ownerId. Moving instances retain old position as a reserved obstacle and expose moveOwnerId/moveExpiresAt. Durable request receipts bind the complete payload and account; they are scrubbed from public HTTP/WebSocket snapshots. The authoritative server recomputes placement, takes the room mutex, clones the world, deducts cost/creates or updates the instance, saves, then publishes. Save failure leaves the previous state intact. Move has no resource cost or new construction timer.

Cancel/expiry/disconnect/restart releases the lock and preserves the original building. Recoverable move failures keep client authority for retry/cancel; cancellation retries until a server snapshot confirms terminal state. Construction uses absolute server deadlines, continues with no browser, and finalizes through the ticker/startup recovery. A player occupying a scaffold at completion may move toward the nearest exit but cannot move deeper into the completed footprint.

## Verification — 2026-10-07

External evidence directory: `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-003`.

- All frontend tests: **44/44 PASS**, zero failed/skipped; 19 building tests plus the 25 accepted movement/swimming/gathering tests (`frontend-tests.log`).
- Production `npm run build`: **PASS**, TypeScript and Vite (`build.log`).
- Fresh `go test -race -count=1 -json ./...`: **44 top-level / 79 including subtests PASS**, zero failures/skips, 20.733 s (`go-race.jsonl`, `go-summary.json`).
- PostgreSQL: **4/4 top-level PASS**, including building restart, gathering restart, existing persistence and original-schema compatibility. Existing safe local database with disposable schemas; no production database used.
- `go vet ./...`: **PASS** (`go-vet.log`).
- Independent review identified four defects: legacy resource radius from missing scale; lost client lock tokens on recoverable errors; client/server occupied-building collision mismatch; stale ghost after camera motion. All four fixed with regressions and scoped review marked addressed (`review.md`, `client-fix-report.md`, server report).
- `git diff --check`: checked before exact staging and again on staged changes.

### Browser acceptance

Two independent local IAB cookie sessions used localhost and 127.0.0.1. Their existing development accounts joined **FT-WEB-003 Building QA**, a new isolated room. The stopped local server's QA room received **100 wood / 100 stone** as an explicit test fixture. This grant is not claimed as earned gathering output. Other rooms were hash-verified unchanged during that fixture write; no account/session data was exported. See `qa-fixture.json`. No runtime state injection or fabricated browser events were used for gameplay acceptance.

| # | Check | Result / evidence |
| --- | --- | --- |
| 1 | Login and enter | PASS; fresh login using existing local test account, 25-login-persistence.png |
| 2 | Wood / Stone present | PASS; fixture 100 / 100 visible |
| 3 | B opens menu | PASS; 01-build-menu.png |
| 4 | Choose Campfire | PASS; actual menu selection |
| 5 | Ghost appears | PASS; 02-ghost-rotation.png |
| 6 | Ghost follows ground | PASS; ground raycast selection at free world coordinates; camera-only rotation also updated projection |
| 7 | R rotation | PASS; 30° ghost and later 60° moved shelter |
| 8 | Valid green/cyan | PASS; legal campfire preview |
| 9 | Water red | PASS; 03-water-rejected.png |
| 10 | Tree red | PASS; 19-tree-rejected.png, decorative tree reservation; resource reservation separately covered by automated tests |
| 11 | Rock red | PASS; 22-rock-rejected.png, near rock-1 footprint. The steep-ground check is the first rejection at this location; specific resource-overlap rejection also covered by Go/TS tests |
| 12 | Building overlap red | PASS; 06-building-overlap.png |
| 13 | Successful placement | PASS; Campfire created once |
| 14 | Correct resource cost | PASS; 100/100 → 96/97 → 88/95 → 72/87 |
| 15 | Construction timer | PASS; Campfire 5s, Workbench 10s, Shelter 20s |
| 16 | Refresh keeps timer | PASS; Workbench reloaded with 4s remaining, 08-workbench-after-reload.png |
| 17 | Completed solid asset | PASS; 11-three-buildings-completed.png |
| 18 | Workbench full loop | PASS; both clients constructing then completed |
| 19 | Shelter full loop | PASS; both clients 20s then completed, including actual backend restart |
| 20 | Shelter blocks player | PASS; route across footprint stopped at X -3.11 / Z 23.01 outside main body, 18-shelter-collision.png |
| 21 | Click Shelter | PASS; selected-building Move panel |
| 22 | Move | PASS; server-granted exclusive session |
| 23 | Ghost at old position | PASS; 14-move-ghost-original.png, X 6.97 / Z 24.36, 30° |
| 24 | Select new location | PASS; ground raycast at X -6.369 / Z 24.780 |
| 25 | Confirm | PASS; actual world click |
| 26 | Position changes | PASS; 15-shelter-moved-A.png |
| 27 | Reload keeps new position | PASS; 17-move-restart-reload.png, exact stored state match after restart |
| 28 | Two sessions online | PASS; separate cookie origins in same QA room |
| 29 | A placement visible to B | PASS for all three buildings; 05, 07, 10 screenshots |
| 30 | A move visible to B | PASS; 16-shelter-moved-B.png |
| 31 | Concurrent move conflict | PASS; simultaneous UI requests: B won, A received server rejection; 12-move-conflict-A.png / 13-move-lock-B.png. B cancelled, then A acquired and moved successfully |
| 32 | Swimming blocks build | PASS; B key in deep sea shows Return to dry land; 23-swimming-build-blocked.png |
| 33 | Ruins protected | PASS; 21-ruins-protected.png |
| 34 | Chat | PASS; B message received by A |
| 35 | Gathering | PASS; B depleted tree-5 after building/moving, inventory 72/87 → 76/87, all three buildings retained; 24-gathering-regression.png |
| 36 | Q dual hand | PASS; quick Q selected pickaxe/left then axe/right |
| 37 | No fatal console | PASS in final reload/login observation; both sessions zero warnings/errors in console-final.json |
| 38 | No resource 404 | PASS in observed load/console; no missing scene assets observed |

All **38 checks PASS** within the observation scope above. This is local development acceptance, not cross-browser/device certification. Physical long-Q was not part of this phase's acceptance and its prior limitation is not silently promoted to a new pass.

### Actual persistence evidence

`construction-before-restart.json` recorded Shelter constructing with 8.8s remaining. After restarting the actual backend, `construction-after-restart.json` retained all original deadlines and inventory; Shelter had correctly completed when the next observation occurred. Workbench refresh independently showed countdown continuation before completion.

`move-before-restart.json` and `move-after-restart.json` match exactly for all building instances and inventory. Shelter moved from (6.97,24.36),30° to (-6.3692198456,24.7797465710),60°, remained completed and retained its original construction timestamps. No duplicate instance and no extra cost appeared.

### Performance

Observed with two operated IAB sessions in one room and the existing unrelated legacy-room Chrome session left idle: generally **107–150 FPS** while building/moving; stable front view with three completed buildings around **124 draw calls / 92,798 triangles**. Moving ghost increased this to approximately **135 calls / 92,906 triangles**. North-facing final sample: **142 FPS / 138 calls / 94,480 triangles** (`26-performance-two-sessions.png`). These are camera-dependent local dev smoke samples, not a controlled hardware benchmark against FT-WEB-002. Rendering uses one ghost frame loop; timers derive from server time and centralized refresh, with no per-building RAF.

## Known limitations and next stage

- Placeholder geometry/icons/scaffolds remain intentional. AssetId registry supports future approved asset replacement; no KayKit assets added.
- Inventory is room shared; any authenticated member can build or move completed room buildings. No personal ownership economy, relocation fee, demolition/refund or upgrades.
- Placement uses an OBB footprint sampled every <=1m, tangent slope and total height delta. It is conservative around trees/shore and uses center elevation; no terrain grading or mesh physics.
- Resources remain reserved even depleted. Server gets their radius from canonical config, supporting FT-WEB-002 saves without scale metadata.
- Movement uses a conservative solid core, including the shelter interior; no doors/interiors/pathfinding were implemented.
- Socket-bound move locks expire after 30s and renew while active. On network loss the client leaves preview and the server releases/recovers the lock; pending idempotent commands retry safely.
- Successful request receipts are retained in room persistence; long-lived receipt retention/compaction and multi-process room coordination are future operational work.
- Multiple invalid conditions can choose different descriptive preview/server reason codes; the server always revalidates independently and the tested acceptance rules agree.
- Browser resource grant is isolated QA setup, not a gameplay reward. Prior stage rooms and the old Freetower project were preserved.

FT-WEB-003 ends with the authorized exact-scope commit and feature-branch push. Do not merge main or begin FT-WEB-004. Recommended next stage after user acceptance: Fishing + Farming adaptation + Survival foundation + Animals.
