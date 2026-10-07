package main

import (
	"context"
	"errors"
	"net/url"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLifePostgresPersistence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "flock_test_" + newID()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := openStore(u.String(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	account := Account{ID: newID(), Username: "TestFarmer", PasswordHash: "bcrypt-is-tested-by-auth-tests"}
	if err = s.createAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	duplicate := account
	duplicate.ID = newID()
	duplicate.Username = "testfarmer"
	if err = s.createAccount(ctx, duplicate); !errors.Is(err, errExists) {
		t.Fatalf("duplicate result = %v", err)
	}
	session := Session{TokenHash: tokenHash("test-only-token"), UserID: account.ID, ExpiresAt: nowMS() + 3600000}
	if err = s.putSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	r, c := lifeFixture(t)
	r.record.Room = Room{ID: newID(), Code: "LIFETEST", Name: "life", OwnerID: account.ID}
	r.record.Members = map[string]bool{account.ID: true}
	c.user.ID = account.ID
	c.player.ID = account.ID
	r.clients = map[*Client]bool{c: true}
	if e := r.joinLife(c, 2000, lifeOK); e != nil {
		t.Fatal(e)
	}
	record := r.record
	if err = s.saveRoom(ctx, record); err != nil {
		t.Fatal(err)
	}
	records, e := s.loadRooms(ctx)
	if e != nil || len(records) != 1 || !reflect.DeepEqual(records[0].State, cloneState(record.State)) {
		t.Fatalf("life PG roundtrip %v", e)
	}
	r.record = records[0]
	if e := r.lifeCommand(c, LifeRequest{RequestID: "pg-eat", Action: "eat", ItemID: "carrot"}, 3000, func(record RoomRecord) error { return s.saveRoom(ctx, record) }); e != nil {
		t.Fatal(e)
	}
	records, e = s.loadRooms(ctx)
	if e != nil {
		t.Fatal(e)
	}
	r.record = records[0]
	if e := r.lifeCommand(c, LifeRequest{RequestID: "pg-eat", Action: "eat", ItemID: "carrot"}, 4000, lifeOK); e != nil {
		t.Fatal(e)
	}
	if r.record.State.FreeTower.Personal[account.ID].Inventory["carrot"] != 2 {
		t.Fatal("PG replay")
	}
}
