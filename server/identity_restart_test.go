package main

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// A real new App/service and freshly reopened persistent store model process
// restart. The browser's cookies and key remain unchanged.
func restartIdentityHarness(t *testing.T, h *identityHarness) *identityHarness {
	t.Helper()
	h.server.Close()
	h.app.shutdown()
	h.app.store.close()
	store, e := openStore("", h.path)
	if e != nil {
		t.Fatal(e)
	}
	app, e := newApp(store, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(app.routes())
	app.identity.config.Origin = server.URL
	app.identity.config.Origins = map[string]bool{server.URL: true}
	next := &identityHarness{t: t, app: app, server: server, client: h.client, key: h.key, address: h.address, path: h.path}
	t.Cleanup(func() { server.Close(); app.shutdown(); store.close() })
	return next
}
func TestWalletServerRestartRequiresFreshAuthentication(t *testing.T) {
	h := newIdentityHarness(t)
	h.fixtureProviders()
	h.login()
	identity := h.selectIdentity(200)
	var room detail
	h.call("POST", "/api/rooms", map[string]string{"name": "persistent world"}, 201, &room)
	next := restartIdentityHarness(t, h)
	next.fixtureProviders()
	next.call("GET", "/api/identity/me", nil, 401, nil)
	next.call("GET", "/api/rooms", nil, 401, nil)
	next.call("GET", "/api/rooms/"+room.Room.ID+"/ws", nil, 401, nil)
	next.login()
	next.call("GET", "/api/rooms", nil, 401, nil)
	again := next.selectIdentity(200)
	if again.ID != identity.ID {
		t.Fatal("restart changed stable character ID")
	}
	next.call("GET", "/api/rooms/"+room.Room.ID, nil, 200, nil)
}
func TestWalletFailedRevocationCannotReviveAfterRestart(t *testing.T) {
	for _, kind := range []string{"asset", "domain"} {
		t.Run(kind, func(t *testing.T) {
			h := newIdentityHarness(t)
			f := h.fixtureProviders()
			h.login()
			identity := h.selectIdentity(200)
			var room detail
			h.call("POST", "/api/rooms", map[string]string{"name": "preserved world"}, 201, &room)
			blocked := filepath.Join(t.TempDir(), "blocked")
			if e := os.WriteFile(blocked, []byte("block"), 0600); e != nil {
				t.Fatal(e)
			}
			original := h.app.store.path
			h.app.store.mu.Lock()
			h.app.store.path = filepath.Join(blocked, "save.json")
			h.app.store.mu.Unlock()
			if kind == "asset" {
				f.assetOwned = false
			} else {
				f.nameOwned = false
			}
			h.selectIdentity(503)
			h.app.store.mu.Lock()
			h.app.store.path = original
			h.app.store.mu.Unlock()
			// Confirm this test really leaves an old active identity/session on disk.
			disk, e := openStore("", h.path)
			if e != nil {
				t.Fatal(e)
			}
			state, e := disk.identityState(context.Background(), nil)
			disk.close()
			if e != nil || state.Identities[identity.ID].Status != "active" || len(state.Sessions) != 1 {
				t.Fatal("test did not retain persisted pre-failure authority")
			}
			next := restartIdentityHarness(t, h)
			next.app.identity.assets = f
			next.app.identity.names = &fixtureNames{f}
			next.call("GET", "/api/identity/me", nil, 401, nil)
			next.call("GET", "/api/rooms", nil, 401, nil)
			next.call("GET", "/api/rooms/"+room.Room.ID+"/ws", nil, 401, nil)
			next.login()
			next.call("GET", "/api/rooms", nil, 401, nil)
			next.selectIdentity(403)
			state, e = next.app.store.identityState(context.Background(), nil)
			if e != nil || state.Identities[identity.ID].Status != kind+"-invalid" {
				t.Fatal("fresh authentication failed to persist transfer")
			}
			rooms, e := next.app.store.loadRooms(context.Background())
			if e != nil || len(rooms) != 1 || !rooms[0].Members[identity.ID] {
				t.Fatal("restart changed world membership")
			}
		})
	}
}
