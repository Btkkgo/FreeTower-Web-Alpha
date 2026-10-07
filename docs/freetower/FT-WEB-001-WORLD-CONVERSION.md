# FT-WEB-001 — FreeTower World Conversion

## Scope and baseline

- Baseline: `b666458d8781290e9a687fc2494098c6bc945fec` (FT-WEB-000).
- Branch: `codex/ft-web-001-world-conversion`.
- The supplied FT-WEB-001 task is the approved implementation scope. This phase replaces the default presentation and locomotion while retaining account, room, chat and durable shared-state services.
- Original `App.tsx`, sheep, farming, building, tutorial, day/night and assets remain available at `/?experience=little-flock`. No wallet, economy redesign, gathering, combat or FT-WEB-002 implementation.
- All new geometry, character, landmark and icon are original procedural primitives. No Islandbound code, assets, map or branding was imported. Existing MIT and asset notices remain intact.

## World and layout

Coordinates use meters, X east and positive Z south. Nominal playable map is 150 × 150 m, with a ±73 m safety boundary; terrain and distant scenery extend beyond the playable area.

| Area | Position / implementation |
|---|---|
| Dawn Grassland | Dry open spawn `(0,18)`, natural soil path toward sea and inland |
| Whispering Forest | Main forest around `(-37,-23)`, irregular deterministic distribution, open winding path, trunk collision and sliding |
| Stone Hills | Walkable height field centered `(34,-22)`, about 12 m above grassland; instanced blue-gray rocks |
| Ancient Ruins | Northern rise around `(0,-57)`, broken stone arch, columns and platform |
| Silver River | Sinuous center `z=-4+4 sin(0.06x)`, 8 m gameplay corridor, submerged channel approximately 4–5 m wide; shallow margins and deep center |
| Ancient Coast | Curved southern shore near z40, sand, sloping seabed and deep water |
| Camp reservation | Open grass between spawn and coast; no building placement system |
| FreeTower | `(16,340)`, original 72 m high blockout, about 322 m from spawn; beyond traversal bounds |

FreeTower has a heavy octagonal base, five tapering stages, gold-edged ledges, three large rings and a floating pale-cyan core. It can be seen from spawn, hill and shore. Forest trunks and crowns naturally obscure it.

Trees use three instanced meshes; rocks and small ground details are instanced. Directional shadows are limited to a 1024 map; hemisphere light, blue sky background and distance fog create fixed bright daylight, represented as 14:00. The legacy simulation clock remains unchanged.

## Camera and input

- Smooth third-person follow, default 17 m distance, approximately 35.5° pitch, 75° FOV and an upward framing offset that keeps the distant tower in view.
- Right-button pointer drag orbits; wheel clamps distance to 9–30 m. Pitch is bounded, and camera height is constrained above the terrain.
- `[` / `]` rotate in 45° steps and `Home` resets the camera; these are also useful accessible alternatives.
- WASD/arrows move relative to the camera, Shift runs, Space jumps, left click navigates to a terrain/water target.
- Directional input cancels navigation. Targets are clamped to the playable bounds; blocked movement cancels after 0.5 seconds. Leaving a room or losing connection clears input/navigation.
- Canopy sphere proxies shorten the follow camera before trees. No navmesh or general camera-obstacle solver. Trunks have collision; decorative rock/ruin meshes are not a complete physics environment.

## Water and locomotion

`server/freetower_world.json` holds shared world/movement parameters. `model.ts` is independent of React and Three.js. `WaterZone` supports river, lake and ocean kinds through a reusable containment function and water level. This map instantiates river and ocean zones; no lake is placed.

| State | Speed | Behavior |
|---|---:|---|
| land | 4.6 m/s | Height-field walking, jump permitted |
| running on land | 7.4 m/s | Shift modifier |
| shallow-water | 3.2 m/s | Feet follow submerged bed; wading indication |
| swimming | 2.8 m/s | Depth >1.1 m; body floats at water level minus 0.35 m; sprint/jump disabled |

Transitions are automatic. Position is interpolated vertically for rendering; shallow/deep transitions change pose instead of dropping the explorer to the bed. Directional and click movement use the same state calculation. The upstream procedural shepherd has no swim animation, so the new explorer uses a procedural horizontal torso and alternating arm/leg strokes. No hat or shepherd staff is rendered in the default experience.

## HUD and sessions

Compact translucent dark HUD: FreeTower and region upper left, online count/settings upper right, small chat lower left, bottom control hints, temporary wading/swimming indicator. No farm diary, grandmother tutorial, sheep adoption or farm shelter is rendered by the default scene.

The same cookie-authenticated REST registration/login and room create/join/list APIs are used. A session-local room ID restores the selected world on reload after checking membership. Positions remain ephemeral and reset at reconnect, as permitted by this phase. WebSocket reconnect uses bounded retry delay and waits for authoritative spawn presence before allowing movement.

## Multiplayer and compatibility

- FreeTower sockets opt in with `?world=freetower`; old clients omit it and retain their original coordinate/movement rules.
- Optional presence fields: `world`, `movementMode`, `moving`, `running`, `elevation`; x/z, facing and server jump timestamps remain compatible.
- The server computes water mode and elevation from the position instead of trusting the client mode. Position range and movement-rate limits still apply.
- Presence is broadcast per coordinate space. Original gameplay spatial queries only use legacy clients; same-account legacy/FreeTower sessions cannot randomly replace one another's gameplay coordinates.
- Membership, chat and shared farm state stay in the same room. No database migration or replacement save format.
- Remote players interpolate position/facing and animate walk/run/swim. A queued jump packet survives the move interval; idle duplicate movement packets are suppressed.
- New client accepts only non-regressing shared state versions/timestamps.

## Validation

Validation on 2026-10-07, macOS, Node 24 and Go 1.27.1:

| Check | Result |
|---|---|
| `npm run build` | PASS |
| `npm test` | PASS, 12 tests, 0 skipped |
| `go test -race -v ./...` with dedicated `TEST_DATABASE_URL` | PASS, 19 tests, 0 skipped |
| PostgreSQL integration tests | PASS, 2 included above, isolated schemas in the existing test database |
| `go vet ./...` | PASS |
| `git diff --check` | PASS |
| Independent code review | Four movement/compatibility findings fixed and re-reviewed; no remaining P1/P2 reported |
| File-store restart | PASS: both accounts, both rooms/memberships and protected farm state retained; both browser sessions automatically reconnected |

Frontend coverage includes land/shallow/deep modes, both shore transition directions, speed configuration, manual cancellation, serialization, bounds, camera-relative controls, queued jumps and canopy camera avoidance. New server coverage includes authoritative water modes, legacy-coordinate isolation, two real WebSocket clients crossing water, and preservation of shared farm state.

Restart evidence hashes accounts and room metadata/memberships plus buildings, regions, coins, XP, level, inventory, sheep, plots and upgrades. Those hashes match before/after the final controlled restart. A preliminary whole-room hash differs because the legacy simulation clock and time-dependent state continue advancing; it is not used as a byte-for-byte persistence claim.

### Browser checklist

Two independent cookie sessions were used: Codex in-app browser and Chrome incognito. Browser observations are separate from code/test proof. Human approval of the visual direction remains pending.

| # | Acceptance | Result / observation |
|---:|---|---|
| 1 | No original large farm tutorial | PASS, default scene |
| 2 | FreeTower upper left | PASS |
| 3 | World no longer a farm | PASS, procedural exploration blockout; final taste approval pending |
| 4 | Open grassland spawn | PASS |
| 5 | Forest visible and entered | PASS |
| 6 | Stone hills visible and reached | PASS |
| 7 | River visible | PASS |
| 8 | Coast visible | PASS |
| 9 | Distant FreeTower visible | PASS |
| 10 | Tower distant | PASS, approximately 322 m from spawn, unreachable |
| 11 | Lower exploration camera | PASS |
| 12 | WASD movement | PASS |
| 13 | Shift running | PASS, browser movement plus speed regression test |
| 14 | Space jump | PASS, airborne screenshot |
| 15 | Click movement | PASS, land and water journeys |
| 16 | Right-button drag orbit | NEEDS_VALIDATION: implementation present; automation supports only left-button drag. Physical mouse confirmation requested, not received |
| 17 | Scroll zoom | PASS, real browser wheel input changed framing |
| 18 | Shallow wading | PASS |
| 19 | Automatic deep-water swimming | PASS, river and ocean |
| 20 | Swimming directional control | PASS |
| 21 | Return to land | PASS, ocean exit returned land mode |
| 22 | No sinking to bed | PASS, observed floating swim pose |
| 23 | Shore traversable | PASS on tested river/ocean routes |
| 24 | Chat | PASS, A message received by B |
| 25 | Concurrent independent sessions | PASS, 2 Online |
| 26 | Remote movement | PASS |
| 27 | Remote swimming | PASS, B saw A swimming and then upright on shore |
| 28 | Reload | PASS, room restored; backend restart also reconnected |
| 29 | No fatal console error | PASS, final default-page warning/error capture empty |
| 30 | No obvious resource 404 | PASS in final browser session |

Browser total: **29 PASS, 1 NEEDS_VALIDATION**. Do not represent this as complete human acceptance.

### Performance and screenshots

Single Chrome incognito renderer at the coast: two HUD samples **121 / 120 FPS**, **73 draw calls**, **90,258 triangles**; screenshot surface 3006 × 1314. With both sessions running, representative coastal observations were 85–98 FPS in Chrome and 124–136 FPS in the in-app browser, approximately 84–85 calls and 91–94k triangles depending on view. These are short development-build samples on this machine, not sustained percentile benchmarks or a controlled baseline comparison.

Evidence directory (outside Git): `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-001/`.

- Spawn: `01-spawn.png`, `01b-spawn-default.png`
- Forest: `02-forest.png`, `02b-forest-tower-occlusion.png`
- Hills: `03-stone-hills.png`
- River: `04-river.png`
- Swimming: `05-swimming.png`, `05b-ocean-swimming.png`
- Coast/tower: `06-coast-tower.png`
- Multiplayer: `07-two-player-swimming.png`, `07b-two-player-shore.png`
- Jump: `08-jump.png`; single-renderer performance: `09-single-browser-performance.png`
- Test/build logs, final console capture, performance samples and sanitized persistence hashes accompany the screenshots. No credentials or save files are committed.

## Follow-up interfaces and limits

- `terrainHeight`, `waterAt`, `locomotionAt`, `advanceMovement`, `movementSpeed`, `MovementSender`, `WaterZone` are the reusable locomotion boundary.
- Terrain behavior exists in TypeScript and Go against one parameter file; formula changes must be kept aligned.
- The renderer is an original blockout, not final production art. Physics, camera object avoidance, mobile touch controls, navmesh routing and ocean travel remain limited.
- The world is finite and the tower is deliberately unreachable. No diving, oxygen or swimming stamina.
- Frame-rate measurements are local development observations, not a cross-device performance guarantee.
- FT-WEB-002 may add gathering, tools, dual hands and wheels **only after human acceptance of this phase**. This branch does not begin that work.
