package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

const stagingFixtureID = "812e807292195582243f7370fde8155dd1e21ff6cb9841bc1cf543a2c0859f92i0"

func TestStagingCharacterConfigurationAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, env, setting  string
		canonical, accepted bool
	}{
		{"explicit", "staging", stagingFixtureID, true, true}, {"absent", "staging", "", true, false}, {"unknown", "staging", strings.Repeat("a", 64) + "i0", true, false}, {"legacy", "staging", stagingFixtureID, false, false}, {"production", "production", stagingFixtureID, true, false}, {"development", "development", stagingFixtureID, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FREETOWER_STAGING_CHARACTER_INSCRIPTION_ID", tc.setting)
			t.Setenv("FT_CHARACTER_INSCRIPTION_ALLOWLIST", stagingFixtureID)
			c := identityConfigForRuntime(RuntimeConfig{Environment: tc.env, CanonicalEnvironment: tc.canonical}, nil)
			if len(c.Assets) != 0 {
				t.Fatal("staging asset leaked into normal allowlist")
			}
			p := newIdentityServiceWithConfig(c).assets.(*UniSatFractalOrdinalsProvider)
			p.key = "fixture"
			p.http = fixtureHTTP(func(*http.Request) (int, string) {
				return 200, fmt.Sprintf(`{"code":0,"data":{"inscriptionId":%q,"address":%q,"height":123}}`, stagingFixtureID, providerAddress)
			})
			ok, e := p.VerifyOwnership(context.Background(), providerAddress, stagingFixtureID)
			if e != nil || ok != tc.accepted {
				t.Fatalf("acceptance %v want %v", ok, tc.accepted)
			}
		})
	}
}
func TestForeignEnvironmentAuthorityDenied(t *testing.T) {
	for _, kind := range []string{"identity", "session", "missing-identity", "missing-session"} {
		t.Run(kind, func(t *testing.T) {
			h := newIdentityHarness(t)
			h.fixtureProviders()
			h.login()
			i := h.selectIdentity(200)
			_, e := h.app.store.identityState(context.Background(), func(d *identityData) error {
				environment := "staging"
				if strings.HasPrefix(kind, "missing-") {
					environment = ""
				}
				if strings.HasSuffix(kind, "identity") {
					raw, _ := json.Marshal(d.Identities[i.ID])
					var v map[string]any
					json.Unmarshal(raw, &v)
					v["environment"] = environment
					raw, _ = json.Marshal(v)
					var changed CharacterIdentity
					json.Unmarshal(raw, &changed)
					d.Identities[i.ID] = changed
				} else {
					for k, s := range d.Sessions {
						raw, _ := json.Marshal(s)
						var v map[string]any
						json.Unmarshal(raw, &v)
						v["environment"] = environment
						raw, _ = json.Marshal(v)
						json.Unmarshal(raw, &s)
						d.Sessions[k] = s
					}
				}
				return nil
			})
			if e != nil {
				t.Fatal(e)
			}
			h.call("GET", "/api/rooms", nil, 401, nil)
			h.call("GET", "/api/rooms/foreign-room/ws", nil, 401, nil)
		})
	}
}

func TestStagingRealProviderSelectionAndSafety(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong-owner", "unconfirmed", "provider-failure", "unknown", "no-name", "no-auth"} {
		t.Run(scenario, func(t *testing.T) {
			h := newIdentityHarness(t)
			f := h.fixtureProviders()
			h.app.identity.config.Environment = "staging"
			h.app.identity.config.CanonicalEnvironment = true
			h.app.identity.config.StagingCharacterInscriptionID = stagingFixtureID
			h.app.identity.config.DevEnabled = false
			p := &UniSatFractalOrdinalsProvider{key: "fixture", stagingID: stagingFixtureID, allow: map[string]bool{}}
			p.http = fixtureHTTP(func(r *http.Request) (int, string) {
				if scenario == "provider-failure" {
					return 503, `{}`
				}
				owner := h.address
				height := 123
				if scenario == "wrong-owner" {
					owner = providerAddress
				}
				if scenario == "unconfirmed" {
					height = 0
				}
				if strings.Contains(r.URL.Path, "/info/") {
					return 200, fmt.Sprintf(`{"code":0,"data":{"inscriptionId":%q,"address":%q,"height":%d}}`, stagingFixtureID, owner, height)
				}
				return 200, fmt.Sprintf(`{"code":0,"data":{"total":1,"cursor":0,"inscription":[{"inscriptionId":%q}]}}`, stagingFixtureID)
			})
			h.app.identity.assets = p
			if scenario != "no-auth" {
				h.login()
			}
			if scenario == "no-name" {
				f.nameOwned = false
				f.names = 0
			}
			id := stagingFixtureID
			status := 200
			switch scenario {
			case "wrong-owner", "unconfirmed", "unknown", "no-name":
				status = 403
			case "provider-failure":
				status = 503
			case "no-auth":
				status = 401
			}
			if scenario == "unknown" {
				id = strings.Repeat("a", 64) + "i0"
			}
			var selected struct{ Identity CharacterIdentity }
			h.call("POST", "/api/identity/select", map[string]string{"characterAssetId": id, "btcName": "alice.fb"}, status, &selected)
			if status != 200 {
				h.call("GET", "/api/rooms", nil, 401, nil)
				return
			}
			if selected.Identity.Environment != "staging" || !selected.Identity.StagingTestCharacter || selected.Identity.Dev {
				t.Fatal("missing real staging metadata")
			}
			var options struct{ Assets []CharacterAsset }
			h.call("GET", "/api/identity/options", nil, 200, &options)
			if len(options.Assets) != 1 || !options.Assets[0].StagingTestCharacter {
				t.Fatal("options missing staging label")
			}
			h.call("GET", "/api/rooms", nil, 200, nil)
			// Preserve boot ID and persisted data deliberately: environment authority must stand alone.
			h.app.identity.config.Environment = "production"
			h.call("GET", "/api/identity/me", nil, 401, nil)
			h.call("GET", "/api/rooms", nil, 401, nil)
			// Even a production session cannot inherit the staged identity or reserved asset.
			_, e := h.app.store.identityState(context.Background(), func(d *identityData) error {
				for k, s := range d.Sessions {
					s.Environment = "production"
					d.Sessions[k] = s
				}
				return nil
			})
			if e != nil {
				t.Fatal(e)
			}
			h.call("GET", "/api/rooms", nil, 401, nil)
			h.call("POST", "/api/identity/select", map[string]string{"characterAssetId": stagingFixtureID, "btcName": "alice.fb"}, 403, nil)
		})
	}
}
func TestWalletChallengeCannotCrossEnvironment(t *testing.T) {
	h := newIdentityHarness(t)
	c := h.challenge()
	h.app.identity.config.Environment = "production"
	h.call("POST", "/api/auth/wallet/verify", h.proof(c), 401, nil)
}
func TestEnvironmentScopedSelectionDoesNotReuseForeignIdentity(t *testing.T) {
	h := newIdentityHarness(t)
	h.fixtureProviders()
	h.login()
	old := h.selectIdentity(200)
	h.app.identity.config.Environment = "test"
	h.login()
	selected := h.selectIdentity(200)
	if selected.ID == old.ID || selected.Environment != "test" {
		t.Fatal("reused foreign identity")
	}
	d, e := h.app.store.identityState(context.Background(), nil)
	if e != nil || d.Identities[old.ID].Environment != "development" {
		t.Fatal("changed foreign identity")
	}
}

func TestPostgresEnvironmentIdentityIsolation(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal("test database initialization failed")
	}
	defer admin.Close()
	schema := "ft006_environment_" + newID()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal("test schema creation failed")
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	u, e := url.Parse(raw)
	if e != nil {
		t.Fatal("test database URL invalid")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	store, e := openStore(u.String(), "")
	if e != nil {
		t.Fatal("test store initialization failed")
	}
	defer store.close()
	service := newIdentityServiceWithConfig(identityConfig{Environment: "production", CanonicalEnvironment: true})
	_, e = store.identityState(ctx, func(d *identityData) error {
		d.Wallets["fixture"] = WalletAccount{ID: "wallet", Network: identityNetwork, Address: providerAddress}
		d.Identities["fixture"] = CharacterIdentity{ID: "fixture", WalletAccountID: "wallet", Environment: "staging", CharacterAssetID: stagingFixtureID, Status: "active", BtcName: "fixture.fb", StagingTestCharacter: true}
		d.Sessions["hash"] = walletSession{Environment: "staging", WalletID: "wallet", IdentityID: "fixture", BootID: service.bootID, ExpiresAt: nowMS() + 100000, Network: identityNetwork}
		return nil
	})
	if e != nil {
		t.Fatal("fixture persistence failed")
	}
	reopened, e := openStore(u.String(), "")
	if e != nil {
		t.Fatal("test store reopen failed")
	}
	defer reopened.close()
	app := &App{store: reopened, identity: service}
	request := httptest.NewRequest("GET", "/api/rooms", nil)
	// Seed a syntactically valid cookie hash using the real cookie hashing path.
	token := "fixture-token"
	hash := tokenHash(token)
	request.AddCookie(&http.Cookie{Name: walletCookie, Value: token})
	_, e = store.identityState(ctx, func(d *identityData) error {
		s := d.Sessions["hash"]
		delete(d.Sessions, "hash")
		d.Sessions[hash] = s
		return nil
	})
	if e != nil {
		t.Fatal("fixture persistence failed")
	}
	if _, _, _, e = app.walletCurrent(ctx, hash); e == nil {
		t.Fatal("production restored staging session")
	}
	_, e = store.identityState(ctx, func(d *identityData) error {
		s := d.Sessions[hash]
		s.Environment = "production"
		d.Sessions[hash] = s
		return nil
	})
	if e != nil {
		t.Fatal("fixture persistence failed")
	}
	if _, _, i, e := app.walletCurrent(ctx, hash); e != nil || i != nil {
		t.Fatal("production inherited staged identity")
	}
	if _, e = app.currentUser(request); e == nil {
		t.Fatal("production game authority accepted staging identity")
	}
	// Even rewritten identity environment cannot smuggle the reserved asset into production.
	_, e = store.identityState(ctx, func(d *identityData) error {
		i := d.Identities["fixture"]
		i.Environment = "production"
		d.Identities["fixture"] = i
		return nil
	})
	if e != nil {
		t.Fatal("fixture persistence failed")
	}
	if _, e = app.currentUser(request); e == nil {
		t.Fatal("production accepted reserved staging asset")
	}
}
