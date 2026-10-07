# FT-WEB-002 — Gathering and hand tools

## Implementation plan and scope

Goal: tree/rock gathering with server-owned durability, shared rewards and timed respawn; two hands, tap/hold Q, independent eight-slot tool/item wheels, remote tools and actions.

Baseline: `fe930c0d039d41242131ba06c9b5cdc54bb6ff85`. Branch: `codex/ft-web-002-gathering-tools`.

The user-supplied FT-WEB-002 spec authorizes this implementation and feature-branch push. Preserve the accepted world, legacy farm/account/room services and original inventory. No asset downloads, building placement, crafting, blockchain, combat or FT-WEB-003. Finish with one exact-scope commit after verification.

Architecture: shared JSON definitions; Go resource transactions under the room lock; optional `FarmState.freeTower` durable state; frontend pure gathering/input model; separate resource/held-item rendering and DOM wheel. Resource inventory is **room shared**, isolated from the legacy farm inventory. Every user temporarily owns both basic tools. Held wood/stone is a visual sample available only when the shared inventory has that resource; equipping does not spend resources.

### Task 1: Server authority

- [x] Add failing Go tests for tools, range, cooldown, windup, swimming, depletion, one reward, simultaneous hits, rollback, save/restart and respawn.
- [x] Implement `server/gathering.go`, extend model/WebSocket/ticker. Embed `freetower_resources.json`.
- [x] Verify Go race and PostgreSQL adapters; review server transaction behavior.

Contract: `HandState {left:string,right:string,active:"left"|"right"}`; empty hand uses `"empty"`. `GatherAction {id,resourceId,toolId,startedAt,hitAt,endsAt}` uses server Unix milliseconds. Player has optional `hands` and `gatherAction`. `FarmState.freeTower {version,nodes:Record<string,ResourceNodeState>,inventory:Record<string,number>}`. Nodes include id/definitionId/position/durability/maxDurability/depleted/respawnAt.

Client messages: `hands-set {hands}`, `gather-start {resourceId,toolId,actionId}`, `resource-hit {resourceId,toolId,actionId}`, `gather-cancel`. Server messages retain presence/state and add `resource-hit {resourceId,userId,actionId,at,depleted}` and `gather-error {code,message}`. Reject start/hit if not an authenticated FreeTower room participant, wrong range/tool/hand, swimming, depleted or cooldown; validate windup and action identity. Persist before announcing success and roll back failed writes. Action cues are transient; resources/inventory persist. Cooldown is shared per user within a room to prevent multiple socket bypass.

### Task 2: Client gameplay and presentation

- [x] Test tap/hold Q (including repeat and cancellation), both wheel categories, equipment selection, gather timing, range/wrong-tool/swim rejection and manual approach cancellation.
- [x] Add `gathering/model.ts`, controller hook, `GatheringHUD.tsx`, asset mapping and resource scene. Extend TypeScript wire types and app integration.
- [x] Attach original primitives at left/right wrists; animate windup/hit/recovery; hide held items while swimming. Existing movement controls yield to wheel/action locks.
- [x] Use five existing trees and four hill rocks. Highlight, restrained durability display, shake/flash/chips, smooth depletion and timestamped respawn.

### Task 3: Integration and acceptance

- [x] Run frontend tests/build, Go race/vet/PostgreSQL tests and independent code review.
- [x] Use two real browser sessions for tree/rock, hands/wheels, wrong tools, swimming, remote actions/state, reload/chat and restart. Save screenshots/logs outside Git.
- [x] Record performance and all 34 acceptance items with truthful PASS/FAIL/NEEDS_VALIDATION boundaries.
- [x] Exact stage, commit `feat: add FreeTower gathering and hand tools`, push feature branch, verify SHA/clean status, stop.

Review focus: duplicate/racing hits, failed-save rollback, multi-socket cooldown, menu/focus input cleanup, replay after reconnect. Server tests cover the first three; client tests and browser checks cover the latter two.

## Validation and limitations

Implemented and verified as detailed below. Manual long-Q validation remains pending; do not interpret implementation completion as unqualified manual acceptance.

### Configuration and asset mapping

`server/freetower_resources.json` is shared by Go and TypeScript. It defines 5 gatherable trees selected from the existing forest and 4 rocks in Stone Hills. Definitions and durable runtime nodes are separate. Decorative trees remain instanced; selected gatherable instances are rendered by `ResourceScene.tsx`.

| Definition | Durability | Tool | Drop | Respawn |
| --- | --- | --- | --- | --- |
| tree-basic | 3 | Basic Axe | wood ×4 | 60 s |
| rock-basic | 4 | Basic Pickaxe | stone ×3 | 75 s |

Both tools have gatherPower 1, 800 ms start-to-start cooldown and one-hand usage. Development characters own both tools by default. Default equipment is left pickaxe, right axe, active right. Wood and stone are resource-category ItemDefinitions; held samples require positive shared inventory and never consume it.

The renderer registry maps `environment.tree.basic`, `environment.rock.basic`, `tool.axe.basic`, `tool.pickaxe.basic`, `item.wood.basic` and `item.stone.basic` to original procedural primitives. Future approved GLTF replacements can change that mapping without modifying gameplay. No downloaded assets or new package dependencies were added.

### Input, timing and feedback

Tap Q switches active hand once on keyup, ignoring key repeat. Hold Q for 350 ms opens Tool Wheel; release selects the highlighted slot and closes without a quick switch. Esc cancels. Both tool/item categories have eight slots. Tools expose Axe/Pickaxe/Empty; items expose Empty/Wood/Stone. Mouse direction or 1–8 selects, Tab switches category. HUD buttons also open each wheel. Focus returns to the canvas after selection/dismissal; chat focus and window blur clear held input.

Click selects a resource and approaches it using existing movement, stopping at about 2.5 m inside the authoritative 2.8 m range. Manual movement, opening the wheel or getting blocked cancels approach. E or another click starts gathering; E can select the nearest valid node. Wrong tool/range/swimming failures give feedback without changing durability.

Windup is 280 ms and recovery 370 ms, with movement locked during the 650 ms action. The client waits for the server's confirmed action and sends the hit after hitAt; cooldown reconciles to server startedAt. Wrist attachments follow the character arms; the active hand swings. Swimming cancels pending gathering and stows held meshes, restored on land.

Authoritative hit events drive shake, flash, instanced chips and restrained durability feedback. `gathering/audio.ts` provides an injectable sound sink, intentionally silent by default. Depletion shrinks the node, clears the target and removes its movement collision; server respawn restores it. One centralized resource frame loop and shared geometry/materials avoid per-tree React simulation loops.

### Authority, persistence and concurrency

Every gathering mutation runs under the room mutex. Server checks live authenticated membership, FreeTower participation, resource availability, range, terrain/player swimming mode, active hand and ownership, tool compatibility, action identity, windup/expiry and cooldown. Hands/actions/cooldown are user-scoped within a room across sockets; replay and simultaneous final hits cannot duplicate rewards.

Hit and respawn paths clone state, save before publication, then install runtime state and broadcast. Failed writes preserve durability, inventory, action consumption and versions. Successful hits and respawns increment both FarmState.Version and FreeTower.Version. Client snapshots reject stale FreeTower versions. Remote presence carries both hands, active hand and server-timed action cues alongside position/facing/movementMode.

File and PostgreSQL saves retain absolute server respawnAt. Restart before deadline retains depletion; at/after deadline server ticker restores full durability, including idle rooms. Reload cannot bypass the timer. Hands/actions/cooldown are transient and reset on server restart. Resource inventory remains room shared and separate from legacy farm inventory; personal economy migration is deferred.

### Automated verification — 2026-10-07

Evidence directory (outside Git): `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-002`.

- Frontend: **25/25 PASS**, 13 new gathering/input tests plus 12 existing movement/swimming tests (`frontend-tests.log`). Includes tap/hold/repeat/cancel, wheel categories/ownership, wrong tool/range/swimming, confirmed hit timing/single hit, stale snapshots, manual approach cancellation, hand payload and cooldown reconciliation.
- `npm run build`: **PASS**, TypeScript and production Vite build (`build.log`).
- `go test -race ./...` with safe local PostgreSQL fixture: **PASS**, 31 top-level tests / 49 including subtests, zero failures or skips (`go-race.jsonl`). Includes 12 new top-level gathering tests, racing final hits, failed-save rollback, replay, cooldown boundaries, file/PG restart and real two-cookie WebSocket protocol.
- PostgreSQL: **3/3 top-level PASS**: original persistence, original-schema compatibility and gathering restart. Disposable schemas were used in the existing local test database.
- `go vet ./...`: **PASS** (`go-vet.log`).
- `git diff --check`: **PASS**; staged diff rechecked before commit.
- Independent review: no remaining actionable P1/P2 in final scoped follow-up (`review.md`). Earlier remote animation fallback and cooldown issues were fixed; sound hook added.

### Real browser acceptance

Two independent IAB cookie sessions used localhost and 127.0.0.1. An existing idle Chrome session also remained online, so some screenshots show three participants. Native Chrome was not used for final acceptance. The operated IAB sessions verified matching resource durability, shared inventory and remote actions.

| # | Check | Result / evidence |
| --- | --- | --- |
| 1 | Login | PASS; fresh login, 13-login-persistence.png |
| 2 | Enter FreeTower | PASS; 14-final-world.png |
| 3 | Axe visible | PASS; wrist attachment and remote view |
| 4 | Pickaxe visible | PASS; wrist attachment in hill view |
| 5 | Quick Q changes active hand | PASS; changed once |
| 6 | Physical long Q | **NEEDS_VALIDATION**; controller tests PASS, browser tool does not expose held-key duration |
| 7 | Choose Axe | PASS; wheel mouse selection |
| 8 | Approach tree | PASS; tree-5 approached to 2.49 m |
| 9 | Tree highlight | PASS; selected ring and compact target display |
| 10 | Chop tree | PASS; E, 04-remote-gather-action.png |
| 11 | Tree feedback | PASS; 06-tree-hit-feedback.png |
| 12 | Tree durability lowers | PASS; 3 → 2 → 1 → 0 |
| 13 | Tree depletion | PASS; shrinks/disappears, target clears |
| 14 | Wood reward | PASS; shared wood 0 → 4 |
| 15 | Choose Pickaxe | PASS; numeric wheel selection and active-left Q |
| 16 | Approach rock | PASS; rock-1 approached to about 2.4 m |
| 17 | Mine rock | PASS; 10-rock-hit.png, 4 → 3 → 2 → 1 → 0 |
| 18 | Rock depletion | PASS; 11-rock-depleted.png |
| 19 | Stone reward | PASS; shared stone 0 → 3 on both clients |
| 20 | Axe rejected on rock | PASS; 09-wrong-tool-rock.png, unchanged durability |
| 21 | Pickaxe rejected on tree | PASS; 05-wrong-tool-tree.png, unchanged durability |
| 22 | Deep water | PASS; 02-swimming-blocks-gather.png |
| 23 | Swimming blocks gathering | PASS; stowed tools and rejection |
| 24 | Land restores tools | PASS; 03-shore-restores-tools.png |
| 25 | Two sessions same room | PASS; separate cookies, matching resource state |
| 26 | B sees A holding Axe | PASS; 04-remote-gather-action.png |
| 27 | B sees A gathering | PASS; raised arm and Gathering cue |
| 28 | Matching durability A/B | PASS; matching tree updates |
| 29 | Matching disappearance A/B | PASS; depleted tree absent, target cleared |
| 30 | Chat | PASS; B sent local test message, A received |
| 31 | Reload | PASS; room restored, wood 4 / stone 3 retained |
| 32 | Server restart | PASS; restart-before.json and restart-after.json have identical FreeTower state while tree-5 depleted; 07-depleted-after-restart.png |
| 33 | No fatal console errors | PASS for final reload observation; console-final.json has zero warnings/errors |
| 34 | No resource 404 | PASS in observed load/console; no missing assets observed |

Additional respawn observation: tree-5 returned; rock-1 returned at 4/4 with wood 4 / stone 3 unchanged (`12-rock-respawn.png`). Wheel layouts: `01-tool-wheel.png`, `08-item-wheel.png`. Default URL hides debug information; `?debug=1` exposes tool, active hand, target id/type, distance, durability, cooldown and world performance.

### Performance

Local development samples with two operated IAB sessions and the existing idle Chrome session: about **119–149 FPS**. Hill view: **52–55 draw calls**, **92,122–92,242 triangles**. Spawn/remote view: **74–76 calls**, about **90,454–90,498 triangles**. These are camera/viewport-dependent smoke samples, not a controlled comparison to FT-WEB-001 or a cross-device guarantee.

### Known limitations and stop boundary

- Physical long-Q acceptance is pending. Tap/hold separation and repeat behavior pass automated tests; button-opened wheel, mouse/numeric selection and quick Q passed browser interaction.
- Inventory is deliberately room shared. Tools are free development defaults. Personal economy/crafting is deferred.
- Audio sink is silent by default; all visual assets and actions are placeholders.
- Approach is straight-line, not pathfinding, and may stop at obstacles. Camera avoidance still approximates original tree bounds after depletion; player collision excludes depleted gatherable trees.
- Hit requests must arrive in the 370 ms server window after windup. High latency/background throttling may safely reject a late hit; latency compensation is deferred.
- Single-process room serialization is tested. Multi-process coordination and production hardening are outside scope.
- Manual acceptance totals **33 PASS / 1 NEEDS_VALIDATION**.

Stop after authorized feature-branch commit/push. FT-WEB-003 is only the proposed next stage: free building placement, ghost preview, moving buildings and construction timer. It has not started.
