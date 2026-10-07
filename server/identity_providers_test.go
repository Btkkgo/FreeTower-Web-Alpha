package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type identityTransport func(*http.Request) (*http.Response, error)

func (f identityTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixtureHTTP(fn func(*http.Request) (int, string)) *http.Client {
	return &http.Client{Transport: identityTransport(func(r *http.Request) (*http.Response, error) {
		status, b := fn(r)
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(b)), Request: r}, nil
	})}
}

const providerAddress = "bc1q9vza2e8x573nczrlzms0wvx3gsqjx7vavgkx0l"

func TestUniSatProviderAuthority(t *testing.T) {
	id := strings.Repeat("a", 64) + "i0"
	ctx := context.Background()
	for _, scenario := range []string{"valid", "empty-allowlist", "no-key", "owner", "unconfirmed", "wrong-id", "status", "api-code", "missing-code", "pagination", "duplicate", "changing-total", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			requests := 0
			p := &UniSatFractalOrdinalsProvider{key: "test-public-fixture-key", allow: map[string]bool{id: true}}
			p.http = fixtureHTTP(func(r *http.Request) (int, string) {
				requests++
				if r.URL.Scheme != "https" || r.URL.Host != "open-api-fractal.unisat.io" || r.Header.Get("Authorization") != "Bearer test-public-fixture-key" {
					t.Fatal("provider endpoint/credential routing")
				}
				if scenario == "status" {
					return 500, `{"secret":"never surface me"}`
				}
				if scenario == "oversized" {
					return 200, strings.Repeat("x", 2*1024*1024+1)
				}
				if scenario == "api-code" {
					return 200, `{"code":-1,"msg":"private provider text"}`
				}
				if scenario == "missing-code" {
					return 200, `{"data":{}}`
				}
				if strings.Contains(r.URL.Path, "/info/") {
					owner := providerAddress
					height := 123
					rid := id
					if scenario == "owner" {
						owner = "bc1pss0zhytly75awhm6x2hhvd5lnzv3vssgrf9axfheq8ldyzn88ges79fler"
					}
					if scenario == "unconfirmed" {
						height = -1
					}
					if scenario == "wrong-id" {
						rid = "wrong"
					}
					return 200, fmt.Sprintf(`{"code":0,"data":{"inscriptionId":%q,"address":%q,"height":%d,"contentType":"text/html","metadata":{"title":"<script>inert</script>"},"collection":{"name":"Fixture collection"}}}`, rid, owner, height)
				}
				if scenario == "pagination" {
					return 200, `{"code":0,"data":{"total":5,"cursor":0,"inscription":[]}}`
				}
				if scenario == "duplicate" {
					return 200, fmt.Sprintf(`{"code":0,"data":{"total":2,"cursor":0,"inscription":[{"inscriptionId":%q},{"inscriptionId":%q}]}}`, id, id)
				}
				if scenario == "changing-total" {
					if r.URL.Query().Get("cursor") == "0" {
						return 200, fmt.Sprintf(`{"code":0,"data":{"total":2,"cursor":0,"inscription":[{"inscriptionId":%q}]}}`, id)
					}
					return 200, `{"code":0,"data":{"total":3,"cursor":1,"inscription":[]}}`
				}
				return 200, fmt.Sprintf(`{"code":0,"data":{"total":1,"cursor":0,"inscription":[{"inscriptionId":%q}]}}`, id)
			})
			if scenario == "empty-allowlist" {
				p.allow = map[string]bool{}
			}
			if scenario == "no-key" {
				p.key = ""
			}
			assets, e := p.GetOwnedCharacterAssets(ctx, providerAddress)
			switch scenario {
			case "valid":
				if e != nil || len(assets) != 1 || assets[0].ContentType != "text/html" || len(assets[0].Metadata) == 0 || len(assets[0].Collection) == 0 {
					t.Fatalf("valid provider: %v %v", assets, e)
				}
			case "empty-allowlist":
				if e != nil || len(assets) != 0 || requests != 0 {
					t.Fatal("empty allowlist not denied locally")
				}
			case "owner", "unconfirmed", "wrong-id":
				if e != nil || len(assets) != 0 {
					t.Fatal("ineligible record returned")
				}
			default:
				if e != errProvider || assets != nil {
					t.Fatalf("provider failure not sanitized: %v %v", assets, e)
				}
			}
		})
	}
}
func TestBNRPForwardReverseOwnerAuthority(t *testing.T) {
	ctx := context.Background()
	id := strings.Repeat("a", 64) + "i0"
	for _, scenario := range []string{"valid", "none", "not-verified", "wrong-owner", "route-only", "wrong-reverse-address", "wrong-name", "non-btc", "missing-inscription", "status", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			p := &BNRPResolver{}
			p.http = fixtureHTTP(func(r *http.Request) (int, string) {
				if r.URL.Host != "api.bnrp.name" || r.Header.Get("Authorization") != "" {
					t.Fatal("resolver credential leak")
				}
				if scenario == "status" {
					return 503, `{"error":"private failure"}`
				}
				if scenario == "malformed" {
					return 200, "not-json"
				}
				if strings.Contains(r.URL.Path, "/reverse/") {
					address := providerAddress
					name := `"alice.btc"`
					verified := true
					if scenario == "none" {
						name = "null"
						verified = false
					}
					if scenario == "not-verified" {
						verified = false
					}
					if scenario == "wrong-reverse-address" {
						address = "bc1pss0zhytly75awhm6x2hhvd5lnzv3vssgrf9axfheq8ldyzn88ges79fler"
					}
					if scenario == "non-btc" {
						name = `"alice.eth"`
					}
					return 200, fmt.Sprintf(`{"address":%q,"name":%s,"verified":%t}`, address, name, verified)
				}
				owner := providerAddress
				name := "alice.btc"
				inscription := id
				if scenario == "wrong-owner" || scenario == "route-only" {
					owner = "bc1pss0zhytly75awhm6x2hhvd5lnzv3vssgrf9axfheq8ldyzn88ges79fler"
				}
				if scenario == "wrong-name" {
					name = "wrong.btc"
				}
				if scenario == "missing-inscription" {
					inscription = ""
				}
				return 200, fmt.Sprintf(`{"name":%q,"owner":%q,"inscription_id":%q,"addresses":{"btc_segwit":%q},"resolved_at":"2026-10-07T00:00:00Z"}`, name, owner, inscription, providerAddress)
			})
			names, e := p.ReverseLookup(ctx, providerAddress)
			switch scenario {
			case "valid":
				if e != nil || len(names) != 1 || names[0].Network != "bitcoin-mainnet" {
					t.Fatal("valid record denied")
				}
			case "none", "not-verified", "wrong-owner", "route-only", "non-btc":
				if e != nil || len(names) != 0 {
					t.Fatalf("unowned record: %v %v", names, e)
				}
			default:
				if e != errProvider {
					t.Fatalf("wanted sanitized failure: %v", e)
				}
			}
		})
	}
}
func TestBTCNamesAndProductionConfig(t *testing.T) {
	for _, name := range []string{"alice.btc", "你好.btc", "a-b.btc"} {
		if !validBTCName(name) {
			t.Errorf("valid %s", name)
		}
	}
	for _, name := range []string{"Alice.btc", "alice.eth", ".btc", "alice.btc.evil", "../alice.btc", "a b.btc", "a\nb.btc", "a.b.btc"} {
		if validBTCName(name) {
			t.Errorf("unsafe %s", name)
		}
	}
	t.Setenv("FT_ENV", "production")
	t.Setenv("FT_ENABLE_DEV_IDENTITY", "true")
	t.Setenv("FT_ENABLE_LEGACY_AUTH", "true")
	t.Setenv("FT_PUBLIC_ORIGIN", "http://localhost:4318")
	c := loadIdentityConfig()
	if c.DevEnabled || c.LegacyEnabled || c.Origins[c.Origin] {
		t.Fatal("development escaped into production")
	}
}
