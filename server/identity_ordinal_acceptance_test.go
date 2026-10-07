package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

const acceptanceFixtureID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaai0"
const acceptanceRoute = "/api/acceptance/ordinal-ownership"

// Ownership of an arbitrary test inscription must never mint game authority.
func TestOrdinalAcceptanceOwnershipIsNotEligibility(t *testing.T) {
	t.Setenv("FT_ENABLE_OWNERSHIP_ACCEPTANCE", "true")
	t.Setenv("FT_CHARACTER_INSCRIPTION_ALLOWLIST", "")
	h := newIdentityHarness(t)
	calls := 0
	p := &UniSatFractalOrdinalsProvider{key: "fixture-key", allow: map[string]bool{}, http: fixtureHTTP(func(r *http.Request) (int, string) {
		calls++
		if r.Method != "GET" || r.URL.Host != "open-api-fractal.unisat.io" || r.URL.Path != "/v1/indexer/inscription/info/"+acceptanceFixtureID {
			t.Fatal("wrong read-only network/route")
		}
		return 200, fmt.Sprintf(`{"code":0,"data":{"inscriptionId":%q,"address":%q,"height":100}}`, acceptanceFixtureID, h.address)
	})}
	h.app.identity.assets = p
	h.login()
	before, e := h.app.store.identityState(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Exists, OwnerResolved, ConnectedWalletMatchesOwner, OwnershipVerified, GrantsCharacterEligibility bool
		Network, OwnerMasked                                                                              string
	}
	status, body := h.request(h.client, "POST", acceptanceRoute, map[string]string{"inscriptionId": acceptanceFixtureID}, h.server.URL)
	if status != 200 {
		t.Fatalf("ownership endpoint got %d want 200", status)
	}
	if json.Unmarshal(body, &result) != nil || !result.Exists || !result.OwnerResolved || !result.ConnectedWalletMatchesOwner || !result.OwnershipVerified || result.GrantsCharacterEligibility || result.Network != "fractal-mainnet" {
		t.Fatalf("ownership result %s", body)
	}
	if strings.Contains(string(body), h.address) || result.OwnerMasked != h.address[:6]+"…"+h.address[len(h.address)-6:] {
		t.Fatal("wallet address not masked")
	}
	after, e := h.app.store.identityState(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(after)
	if string(b1) != string(b2) || len(after.Identities) != 0 {
		t.Fatal("ownership wrote identity/session state")
	}
	if len(h.app.identity.config.Assets) != 0 || len(p.allow) != 0 {
		t.Fatal("ownership changed allowlist")
	}
	ok, e := p.VerifyOwnership(context.Background(), h.address, acceptanceFixtureID)
	if e != nil || ok {
		t.Fatal("non-eligible asset became Character")
	}
	if calls != 1 {
		t.Fatal("eligibility should deny locally")
	}
	h.call("POST", "/api/identity/select", map[string]string{"characterAssetId": acceptanceFixtureID, "btcName": "yihoon.fb"}, 403, nil)
	h.call("GET", "/api/rooms", nil, 401, nil)
	h.call("POST", "/api/rooms", map[string]string{"name": "must-not-enter"}, 401, nil)
	h.call("GET", "/api/rooms/test/ws", nil, 401, nil)
	after, _ = h.app.store.identityState(context.Background(), nil)
	if len(after.Identities) != 0 {
		t.Fatal("selection created an identity")
	}
}
func TestOrdinalAcceptanceFailuresAndIsolation(t *testing.T) {
	for _, scenario := range []string{"wrong-wallet", "unknown", "provider-failure", "wrong-id", "unconfirmed", "missing-key", "unauthenticated", "dev-wallet", "disabled", "production", "bad-id", "wrong-origin"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("FT_ENABLE_OWNERSHIP_ACCEPTANCE", "true")
			t.Setenv("FT_CHARACTER_INSCRIPTION_ALLOWLIST", "")
			if scenario == "disabled" {
				t.Setenv("FT_ENABLE_OWNERSHIP_ACCEPTANCE", "false")
			}
			h := newIdentityHarness(t)
			calls := 0
			p := &UniSatFractalOrdinalsProvider{key: "fixture-key", allow: map[string]bool{}, http: fixtureHTTP(func(r *http.Request) (int, string) {
				calls++
				if scenario == "unknown" {
					return 404, `{}`
				}
				if scenario == "provider-failure" {
					return 503, `{"secret":"must not leak"}`
				}
				address, id, height := h.address, acceptanceFixtureID, 100
				if scenario == "wrong-wallet" {
					address = providerAddress
				}
				if scenario == "wrong-id" {
					id = strings.Repeat("b", 64) + "i0"
				}
				if scenario == "unconfirmed" {
					height = 0
				}
				return 200, fmt.Sprintf(`{"code":0,"data":{"inscriptionId":%q,"address":%q,"height":%d}}`, id, address, height)
			})}
			h.app.identity.assets = p
			if scenario == "missing-key" {
				p.key = ""
			}
			if scenario == "dev-wallet" {
				h.call("POST", "/api/auth/wallet/dev", map[string]string{"persona": "alice"}, 200, nil)
			} else if scenario != "unauthenticated" {
				h.login()
			}
			if scenario == "production" {
				h.app.identity.config.Development = false
			}
			id, origin := acceptanceFixtureID, h.server.URL
			if scenario == "bad-id" {
				id = "../not-an-inscription"
			}
			if scenario == "wrong-origin" {
				origin = "https://unrelated.example"
			}
			status, raw := h.request(h.client, "POST", acceptanceRoute, map[string]string{"inscriptionId": id}, origin)
			want := 200
			switch scenario {
			case "unknown":
				want = 404
			case "provider-failure", "wrong-id", "missing-key":
				want = 503
			case "unauthenticated":
				want = 401
			case "dev-wallet", "disabled", "production", "wrong-origin":
				want = 403
			case "bad-id":
				want = 400
			}
			if status != want {
				t.Fatalf("got %d want %d", status, want)
			}
			var result map[string]any
			json.Unmarshal(raw, &result)
			if result["ownershipVerified"] == true || result["grantsCharacterEligibility"] == true || strings.Contains(string(raw), "must not leak") {
				t.Fatal("failure granted ownership/eligibility or leaked provider data")
			}
			if (scenario == "disabled" || scenario == "production" || scenario == "dev-wallet" || scenario == "unauthenticated" || scenario == "bad-id" || scenario == "wrong-origin" || scenario == "missing-key") && calls != 0 {
				t.Fatal("rejected request called provider")
			}
			state, _ := h.app.store.identityState(context.Background(), nil)
			if len(state.Identities) != 0 || len(p.allow) != 0 || len(h.app.identity.config.Assets) != 0 {
				t.Fatal("acceptance changed authority")
			}
		})
	}
}
