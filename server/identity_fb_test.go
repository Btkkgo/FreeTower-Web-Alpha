package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// These tests catch accepting a duplicate name inscription, stale ownership,
// underconfirmed transfers, incomplete enumeration, or a provider failure.
func TestFractalNamesAuthority(t *testing.T) {
	id := strings.Repeat("a", 64) + "i0"
	other := "bc1pss0zhytly75awhm6x2hhvd5lnzv3vssgrf9axfheq8ldyzn88ges79fler"
	for _, scenario := range []string{"valid", "two-names", "unrelated-text", "duplicate-registration", "owner", "confirmations", "wrong-protocol", "wrong-name", "status", "missing-code", "pagination", "no-key", "wrong-indexer-owner"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("UNISAT_FRACTAL_API_KEY", "public-test-fixture")
			if scenario == "no-key" {
				t.Setenv("UNISAT_FRACTAL_API_KEY", "")
			}
			old := http.DefaultTransport
			defer func() { http.DefaultTransport = old }()
			http.DefaultTransport = fixtureHTTP(func(r *http.Request) (int, string) {
				if r.Method != "GET" {
					t.Fatal("names must only read")
				}
				if scenario == "status" {
					return 503, `{"msg":"private upstream detail"}`
				}
				if scenario == "missing-code" {
					return 200, `{"data":{}}`
				}
				owner, conf := providerAddress, 3
				if scenario == "owner" {
					owner = other
				}
				if scenario == "confirmations" {
					conf = 2
				}
				rec := func(rid string) string {
					return fmt.Sprintf(`{"inscriptionId":%q,"address":%q,"contentType":"text/plain;charset=utf-8","contentLength":40,"utxoHeight":100,"utxoConfirmation":%d}`, rid, owner, conf)
				}
				switch r.URL.Host {
				case "wallet-api-fractal.unisat.io":
					if r.Header.Get("Authorization") != "" {
						t.Fatal("Open API key leaked to wallet API")
					}
					switch r.URL.Path {
					case "/v5/address/inscriptions":
						if r.URL.Query().Get("address") != providerAddress {
							t.Fatal("wrong wallet")
						}
						if scenario == "pagination" {
							return 200, `{"code":0,"data":{"total":1,"list":[]}}`
						}
						list := rec(id)
						total := 1
						if scenario == "two-names" || scenario == "unrelated-text" {
							list += "," + rec(strings.Repeat("b", 64)+"i0")
							total = 2
						}
						return 200, fmt.Sprintf(`{"code":0,"data":{"total":%d,"list":[%s]}}`, total, list)
					case "/v5/address/search":
						rid := id
						if scenario == "duplicate-registration" || r.URL.Query().Get("domain") == "second.fb" {
							rid = strings.Repeat("b", 64) + "i0"
						}
						return 200, `{"code":0,"data":` + rec(rid) + `}`
					}
				case "fractal-static.unisat.io":
					if r.Header.Get("Authorization") != "" {
						t.Fatal("key leaked to content host")
					}
					if scenario == "unrelated-text" && strings.Contains(r.URL.Path, strings.Repeat("b", 64)) {
						return 200, "ordinary non-JSON inscription"
					}
					protocol, name := "sns", "alice.fb"
					if strings.Contains(r.URL.Path, strings.Repeat("b", 64)) {
						name = "second.fb"
					}
					if scenario == "wrong-protocol" {
						protocol = "not-sns"
					}
					if scenario == "wrong-name" {
						name = "spoof.btc"
					}
					return 200, fmt.Sprintf(`{"p":%q,"op":"reg","name":%q}`, protocol, name)
				case "open-api-fractal.unisat.io":
					if r.Header.Get("Authorization") != "Bearer public-test-fixture" {
						t.Fatal("key missing from indexer")
					}
					if scenario == "wrong-indexer-owner" {
						owner = other
					}
					rid := strings.TrimPrefix(r.URL.Path, "/v1/indexer/inscription/info/")
					return 200, fmt.Sprintf(`{"code":0,"data":{"inscriptionId":%q,"address":%q,"height":100}}`, rid, owner)
				}
				t.Errorf("unexpected provider route %s", r.URL.Host+r.URL.Path)
				return 500, `{}`
			}).Transport
			p := newIdentityService().names
			names, e := p.ReverseLookup(context.Background(), providerAddress)
			switch scenario {
			case "valid", "two-names", "unrelated-text":
				want := 1
				if scenario == "two-names" {
					want = 2
				}
				if e != nil || len(names) != want {
					t.Fatalf("owned names count=%d err=%v", len(names), e)
				}
				for _, n := range names {
					if n.Owner != providerAddress || n.Network != "fractal-mainnet" || n.Provider != "unisat-fractal-satsnames" {
						t.Fatal("wrong provenance")
					}
				}
				ok, e := p.VerifyOwnership(context.Background(), "alice.fb", providerAddress)
				if e != nil || !ok {
					t.Fatal("fresh ownership rejected")
				}
				ok, e = p.VerifyOwnership(context.Background(), "alice.btc", providerAddress)
				if e != nil || ok {
					t.Fatal("old namespace accepted")
				}
			case "status", "missing-code", "pagination", "no-key":
				if e == nil || len(names) != 0 {
					t.Fatal("provider failure not closed")
				}
			default:
				if e != nil || len(names) != 0 {
					t.Fatalf("invalid name accepted count=%d err=%v", len(names), e)
				}
			}
		})
	}
}
