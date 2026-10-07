# FT-WEB-005.2 — Real Test Ordinal Ownership Acceptance

Baseline: `d4bdb5b294a68653bc4792bd6a488d25fe1cf3ae`.
Branch: `codex/ft-web-005-2-test-ordinal-ownership`.

**Real test ordinal ownership: PASS. Production Character eligibility, active CharacterIdentity and production game entry: BLOCKED AS DESIGNED.** Ownership and eligibility are independent results. This inscription was not approved or configured as a Production/Alpha Character.

Test inscription: `812e807292195582243f7370fde8155dd1e21ff6cb9841bc1cf543a2c0859f92i0`.

## Real authenticated acceptance

The localhost server was restarted to load the acceptance helper, invalidating its previous wallet sessions by design. A fresh real UniSat FreeTower Sign-In message was shown and signed, with Fractal Mainnet selected. Server verification returned HTTP 200 and `dev=false`. No transaction/PSBT, mint, transfer, payment or asset-role promotion was performed.

A temporary loopback audit proxy used that newly established session cookie **in memory only** to invoke `POST /api/acceptance/ordinal-ownership`. It supplied only the test inscription ID. The handler obtained the wallet address from the authenticated session, not a caller-provided address.

The server used its environment-only `UNISAT_FRACTAL_API_KEY` to GET `https://open-api-fractal.unisat.io/v1/indexer/inscription/info/{id}`. Lookup succeeded with HTTP 200 / provider code 0, exact inscription ID, canonical owner and confirmed height. Network provenance is the fixed Fractal Mainnet API endpoint; it is not inferred from the shared `bc1` address prefix.

Sanitized result:

```json
{
  "inscriptionId": "812e807292195582243f7370fde8155dd1e21ff6cb9841bc1cf543a2c0859f92i0",
  "network": "fractal-mainnet",
  "exists": true,
  "ownerResolved": true,
  "ownerMasked": "bc1pzp…u3qz60",
  "confirmed": true,
  "connectedWalletMatchesOwner": true,
  "ownershipVerified": true,
  "grantsCharacterEligibility": false
}
```

After ownership verification, the same real wallet session produced:

| Check | Actual result |
| --- | --- |
| Identity session | HTTP 200, `dev=false`, no active CharacterIdentity |
| Eligible Character options | HTTP 200, zero assets |
| Name verification | `yihoon.fb` returned again, PASS |
| Game API `/api/rooms` | HTTP 401 |
| Browser | Wallet verified; Owned `.fb` = `yihoon.fb`; No eligible Character asset found; Enter FreeTower disabled |
| Production Character allowlist | Empty; unchanged |

## Isolation and behavior

- Added a separate ownership-evidence helper on the real UniSat provider. It deliberately does not consult or modify the Character allowlist and does not return a CharacterAsset or create identity authority. Existing Character eligibility/ownership behavior is unchanged.
- Added an acceptance-only HTTP handler. It is disabled by default, requires both `FT_ENV=development` and `FT_ENABLE_OWNERSHIP_ACCEPTANCE=true`, loopback access, an approved Origin, and a live real Fractal wallet session. DEV wallet sessions are rejected, and provider failure cannot select a DEV provider.
- The helper uses the existing bounded, redirect-refusing server-side HTTP client and fixed Fractal endpoint. It verifies the exact returned inscription ID and owner, requires confirmed height for ownership PASS, masks the owner, and emits no key/proof/cookie. Invalid/unknown/provider-failed results do not grant ownership or eligibility.
- The handler is read-only with respect to the identity store, issues no session cookie, rechecks the session after provider I/O, and returns `Cache-Control: no-store`. Its result is evidence only; the selection/game routes continue to require independent Character eligibility.
- Production mode denies this endpoint even if the acceptance flag is explicitly true. A separate real production-mode process confirmed HTTP 403 `acceptance_disabled` with that flag enabled.
- `.fb` implementation, frontend/game code and Character allowlist were not changed. No production/Alpha asset configuration was added.

## Automated verification

Tests first failed because the independent acceptance route did not exist. After implementation:

- Non-eligible inscription ownership succeeds without changing allowlist, identity/session state or Character eligibility.
- Attempted Character selection still fails; game listing, creation and WebSocket routes remain unauthorized.
- Wrong wallet returns ownership mismatch; unknown inscription fails safely; unconfirmed ownership does not PASS.
- Provider failure, wrong returned ID and missing key fail closed; no DEV fallback.
- Unauthenticated/DEV wallets, disabled mode, production mode, invalid ID and wrong Origin are denied.
- All existing identity tests still pass.

Fresh full suite: frontend **87/87**; TypeScript/Vite build **PASS**; Go race **214 test/subtest pass events, 88 top-level tests, zero failures/skips**; PostgreSQL **6/6** integration tests; `go vet ./...` **PASS**.

Security scan checked actual API-key and full real-address literals plus raw signature/nonce/session patterns across changed source, production bundle and captured textual evidence/logs: no findings. Synthetic test fixture credentials are not production secrets.

## Evidence and closeout

Local evidence: `/Users/btkkgo/.codex/visualizations/2026/10/07/01a11498-da43-7443-8f74-f9737f13bf29/FT-WEB-005-2`.

Contains sanitized real server audit, real browser gate screenshot/AX, production-mode denial, full test/build/race/vet logs, security scan and final status. Raw auth payloads were neither saved nor published.

The temporary audit proxy and isolated production-check process are removed/stopped at closeout. Port 8080 runs the current source directly with the key preserved in environment, acceptance disabled, DEV identity disabled, legacy authentication disabled and Character allowlist empty. Restart invalidates test sessions; accounts/world data remain intact.

**READY FOR STAGING DEPLOYMENT of the closed-gate build**, with `FT_ENV=production`, acceptance/DEV/legacy disabled and the Character allowlist still empty. This is readiness for staging verification of this scoped change, not a deployment result, playable Character release, or permission to merge/deploy. No deployment or main merge was performed.
