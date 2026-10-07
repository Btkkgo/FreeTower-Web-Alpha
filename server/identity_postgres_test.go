package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/url"
	"os"
	"sync"
	"testing"
)

func TestPostgresWalletIdentityPersistenceAndAtomicity(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := "ft005_identity_" + newID()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := openStore(u.String(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	other, e := openStore(u.String(), "")
	if e != nil {
		t.Fatal(e)
	}
	defer other.close()
	key := walletKey(identityNetwork, providerAddress)
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store := s
			if n%2 == 0 {
				store = other
			}
			_, e := store.identityState(ctx, func(d *identityData) error {
				w := d.Wallets[key]
				if w.ID == "" {
					w = WalletAccount{ID: newID(), Address: providerAddress, Network: identityNetwork, Provider: "unisat"}
					d.Wallets[key] = w
				}
				d.Identities["character"] = CharacterIdentity{ID: "character", WalletAccountID: w.ID, CharacterAssetID: "asset", BtcName: "alice.fb", Status: "active"}
				d.Sessions["session"] = walletSession{TokenHash: "session", WalletID: w.ID, IdentityID: "character", Network: identityNetwork, ExpiresAt: nowMS() + 100000}
				return nil
			})
			failures <- e
		}(n)
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	d, e := other.identityState(ctx, nil)
	if e != nil || len(d.Wallets) != 1 || d.Identities["character"].BtcName != "alice.fb" || len(d.Sessions) != 1 {
		t.Fatalf("roundtrip %v", e)
	}
	_, e = s.identityState(ctx, func(d *identityData) error { delete(d.Sessions, "session"); return errIdentity })
	if e != errIdentity {
		t.Fatal(e)
	}
	d, _ = other.identityState(ctx, nil)
	if len(d.Sessions) != 1 {
		t.Fatal("rollback failed")
	}
	_, e = s.identityState(ctx, func(d *identityData) error { delete(d.Sessions, "session"); return nil })
	if e != nil {
		t.Fatal(e)
	}
	d, _ = other.identityState(ctx, nil)
	if len(d.Sessions) != 0 {
		t.Fatal("logout not persisted")
	}
}
