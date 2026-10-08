# FreeTower public staging implementation plan

> For agentic workers: use superpowers:subagent-driven-development for this plan.

**Goal:** Deploy and verify one HTTPS/WSS Railway Singapore staging instance with PostgreSQL and real wallet identity while keeping production closed.
**Architecture:** Go serves optimized Vite dist plus API and websocket on one origin. Environment-scoped staging character authority is distinct from the empty production allowlist.
**Tech Stack:** React/Three.js/Vite, Go, PostgreSQL, Docker, Railway.
**Spec:** /Users/btkkgo/.codex/attachments/b41a2401-4341-4111-8bb8-26e2b5ed98fd/已粘贴的文本.txt

## Global Constraints

- Repository /Users/btkkgo/Documents/FreeTower-Web; branch codex/ft-web-006-staging-deployment; baseline 381d7af5dee3e10b0033de31de16e9003a095da6. Work in the user-designated clean checkout and branch.
- Production Character Allowlist remains empty. No mint, payment, transfer, allow-all, production launch, main merge, force push, destructive git commands, or git add .
- No API key, full real wallet address, cookie, signature, session token or database URL in output, reports, Git or logs. Do not inspect real server/data identity contents or process environment in worker agents.
- Real public acceptance cannot be inferred from unit tests or previous localhost evidence. Distinguish PASS, FAIL, BLOCKED and NOT RUN.
- Only the root agent handles live browser, Railway account, provider secrets and deployment. No worker subagents.
- Root coordinates final tests/build, exact final commit chore: deploy FreeTower staging, push, and strict 63-field report.

## Review Focus

Check environment variables, HTTP inputs/origins, provider responses, persisted identity records, and deployment configuration independently. Fail closed on malformed/missing environment data, unavailable providers, stale or foreign-environment sessions and absent PostgreSQL. Tests must exercise authority boundaries and persistence rather than merely assert source text.

### Task 1: Public runtime and deployment foundation

Files: server/main.go, server/http.go, server/websocket.go, server/identity*.go as needed for shared runtime config only, new focused server/runtime_config.go and tests, Dockerfile, .dockerignore, railway.json.

1. Inspect existing runtime and write failing behavioral tests before changes. Add canonical FREETOWER_ENV, FREETOWER_PUBLIC_ORIGIN, FREETOWER_REQUIRE_DATABASE, FREETOWER_GIT_SHA handling; maintain local legacy FT_ compatibility but canonical settings win. Public staging/production must force DEV identity, legacy auth and local ownership-acceptance endpoint off regardless old flags. Expose runtime environment consistently for Task 2.
2. Staging requires nonempty DATABASE_URL and exact configured HTTPS public origin; fail startup with sanitized errors. No JSON fallback in staging or explicit require-database mode. Health returns ok/database connectivity/environment/short valid commit only. Existing schema remains idempotent and data preserving.
3. Strict configured origin for public REST mutation, wallet challenge, websocket. Empty or wrong origins denied for mutations/websocket; safe GET health/static work. Secure HttpOnly SameSite=Lax cookies on public environments. Existing local dev/test compatibility may be retained explicitly.
4. Audit/add minimal protection for identity query, challenge/verify, mutations/chat; preserve provider timeout behavior. Set compatible CSP/nosniff/referrer/frame policy (inspect external fonts/assets). Sanitize logging, startup 'FreeTower listening on :PORT', retain PORT support and improve shutdown if needed to save state/close websockets/DB in safe order.
5. Final review addition: public PostgreSQL world ownership must be exclusive before room loading. Use one advisory-locked dedicated connection for all room writes/deletes, hold through final persistence and close, reject a contender, and fail closed on owner connection loss without pool fallback. Test active websocket shutdown, competing owners, forced connection loss and final-state handoff. Railway drainingSeconds is 120 and overlapSeconds is 0; deployment procedure stops/drains the old app before starting a replacement. This enforces one writer and does not introduce scaling.
6. Docker Node 24 multistage, Go compilation with embedded runtime resources, nonroot minimal runtime CA/dist/binary; exclude secrets, development databases, node_modules, Git and irrelevant context. Add useful Railway config healthcheck and one replica in asia-southeast1-eqsg3a. No cloud deployment by worker.
7. Run focused tests during development, then server suite, vet, frontend test/build once if touched/affected; local PostgreSQL integration URI may be provided separately. Docker unavailable locally: document not run, do not install Docker. Self-review and precisely commit code as a preparatory commit.
8. Report exact test evidence, runtime config interfaces consumed by Task 2, changed files and unresolved cloud checks.

### Task 2: Staging-only real character authority

Files: server identity configuration/providers/session/store/tests; src/identity/types.ts, src/identity/IdentityGate.tsx and related focused UI/tests.

1. Add explicit staging-only config FREETOWER_STAGING_CHARACTER_INSCRIPTION_ID, accepting only 812e807292195582243f7370fde8155dd1e21ff6cb9841bc1cf543a2c0859f92i0 and only canonical FREETOWER_ENV=staging. Do not put it in CharacterEligibilityConfig or production allowlist. No automatic default acceptance unless configured.
2. Use real UniSat Fractal API and fresh current owner matching authenticated real wallet, with existing confirmation/error safety. Keep .fb logic unchanged and revalidate names normally. Local acceptance endpoint remains disabled publicly.
3. Staging-created active CharacterIdentity has environment=staging; production rejects it even if persisted data/session crosses environments. Enforce environment on session restoration, selection/reuse, query and every game authority boundary; production never accepts staging config/asset by inheritance. Wallet sessions/challenges should also be environment-bound where necessary.
4. UI explicitly labels Staging Test Character from server metadata; no DEV acceptance buttons or legacy UI in public staging; no secret or hardcoded authority embedded in frontend. Optimized production frontend build is shared, but authority is server environment scoped. Public production environment cannot use staging identities.
5. TDD behavioral cases: staging exact asset accepted with genuine-provider-shaped responses and matching owner; wrong owner/provider failure/unconfirmed/unknown asset rejected; production with staging config rejects; production allowlist still empty; foreign environment identity/session cannot enter game; staging ownership without name or wallet auth cannot enter; existing identity and frontend tests remain passing. Add meaningful PostgreSQL coverage for environment identity isolation if appropriate.
6. Run focused then complete required suites, self-review and precise preparatory commit; report evidence and config instructions without secrets.

### Task 3: Cloud deployment, real acceptance and documentation

Root-owned operational task after code review and full build/frontend/race/PostgreSQL/vet PASS. Inspect Railway authenticated workspace and plan; require human login/authorization/payment/plan confirmation per spec. Create FreeTower-Staging/staging services in Singapore without upgrade, configure only Railway Variables, deploy Docker single origin and healthcheck. Verify real HTTPS, assets, UniSat Fractal/BIP322/replay, ordinal, yihoon.fb, staging identity and WSS/gameplay. Restart app only, verify PostgreSQL state and reconnect. Record backup and cost status, transfer/performance baseline, sanitized evidence. Write docs/freetower/FT-WEB-006-STAGING-DEPLOYMENT.md and docs/freetower/STAGING_BACKUP.md with real results and explicit NOT RUN/BLOCKED for unavailable operations. Final broad review, secret scan, diff check, exact commit/push. Never claim closed-alpha ready without required public evidence.
