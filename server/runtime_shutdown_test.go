package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Exercise the same shutdown-before-store-close ordering as main with a live
// authenticated websocket and an unsaved authoritative world change.
func TestConnectedWebsocketShutdownPersistsBeforeDatabaseClose(t *testing.T) {
	for _, public := range []bool{false, true} {
		name := "local"
		if public {
			name = "public-owner"
		}
		t.Run(name, func(t *testing.T) { testConnectedWebsocketShutdown(t, public, false) })
	}
}
func TestPublicWorldWriterClosesConnectedWebsocketOnLoss(t *testing.T) {
	testConnectedWebsocketShutdown(t, true, true)
}
func testConnectedWebsocketShutdown(t *testing.T, public, loseOwner bool) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal("test database connection failed")
	}
	defer admin.Close()
	schema := "ft006_shutdown_" + newID()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal("test schema creation failed")
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error("test schema cleanup failed")
		}
	}()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	store, err := openStore(u.String(), "")
	if err != nil {
		t.Fatal("test store initialization failed")
	}
	defer store.close()
	account := Account{ID: newID(), Username: "ShutdownPlayer", PasswordHash: "test-only"}
	if err = store.createAccount(ctx, account); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 64)
	if err = store.putSession(ctx, Session{TokenHash: tokenHash(token), UserID: account.ID, ExpiresAt: nowMS() + 60000}); err != nil {
		t.Fatal(err)
	}
	record := RoomRecord{Room: Room{ID: newID(), Code: "SHUTDOWN", Name: "Shutdown world", OwnerID: account.ID}, Members: map[string]bool{account.ID: true}, State: initialState(nowMS())}
	if err = store.saveRoom(ctx, record); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FT_ENV", "development")
	t.Setenv("FT_ENABLE_LEGACY_AUTH", "true")
	var app *App
	if public {
		app, err = newAppWithConfig(store, t.TempDir(), RuntimeConfig{Environment: "production", CanonicalEnvironment: true, PublicOrigin: "https://shutdown.example.test", RequireDatabase: true})
	} else {
		app, err = newApp(store, t.TempDir())
	}
	if err != nil {
		t.Fatal(err)
	}
	defer app.shutdown()
	handlerDone := make(chan struct{})
	routes := app.routes()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		routes.ServeHTTP(w, r)
	}))
	defer server.Close()
	origin, cookie := server.URL, "flock_session="+token
	if public {
		origin = app.runtime.PublicOrigin
		cookie = walletCookie + "=" + token
		_, err = store.identityState(ctx, func(d *identityData) error {
			wallet := WalletAccount{ID: "shutdown-wallet", Address: "synthetic-shutdown-owner", Network: identityNetwork}
			d.Wallets[walletKey(identityNetwork, wallet.Address)] = wallet
			d.Identities[account.ID] = CharacterIdentity{ID: account.ID, WalletAccountID: wallet.ID, Environment: "production", CharacterAssetID: "synthetic-test-asset", BtcName: "shutdown.fb", Status: "active"}
			d.Sessions[tokenHash(token)] = walletSession{TokenHash: tokenHash(token), WalletID: wallet.ID, IdentityID: account.ID, Network: identityNetwork, Environment: "production", BootID: app.identity.bootID, ExpiresAt: nowMS() + 60000}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	headers := http.Header{"Origin": []string{origin}, "Cookie": []string{cookie}}
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/rooms/"+record.Room.ID+"/ws", headers)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if err = ws.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// Receiving initial state proves registration and the read/write pumps exist.
	for {
		var msg map[string]any
		if err = ws.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg["type"] == "state" {
			break
		}
	}
	rr := app.rooms[record.Room.ID]
	rr.mu.Lock()
	if len(rr.clients) != 1 {
		rr.mu.Unlock()
		t.Fatal("connected client missing")
	}
	var client *Client
	for c := range rr.clients {
		client = c
	}
	rr.record.State.Coins = 7654
	rr.lastPersist = nowMS()
	if loseOwner {
		if err = store.saveRoom(ctx, rr.record); err != nil {
			rr.mu.Unlock()
			t.Fatal(err)
		}
	}
	rr.mu.Unlock()
	before, err := store.loadRooms(ctx)
	if !loseOwner && (err != nil || len(before) != 1 || before[0].State.Coins == 7654) {
		t.Fatal("world change was not pending before shutdown")
	}
	if loseOwner {
		store.worldWriter.mu.Lock()
		pid := store.worldWriter.conn.PgConn().PID()
		store.worldWriter.mu.Unlock()
		if _, err = store.db.Exec(ctx, "SELECT pg_terminate_backend($1)", pid); err != nil {
			t.Fatal("test writer termination failed")
		}
		if err = store.worldWriterHealthy(ctx); err == nil {
			t.Fatal("terminated writer still healthy")
		}
		select {
		case <-client.done:
		case <-time.After(3 * time.Second):
			t.Fatal("writer-loss monitor did not close websocket")
		}
	}
	shutdownDone := make(chan struct{})
	go func() { app.shutdown(); close(shutdownDone) }()
	select {
	case <-shutdownDone:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not drain connected websocket")
	}
	select {
	case <-client.done:
	default:
		t.Fatal("shutdown returned with client still open")
	}
	rr.mu.Lock()
	remaining := len(rr.clients)
	rr.mu.Unlock()
	if remaining != 0 {
		t.Fatal("shutdown returned before websocket handler removed client")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("websocket HTTP handler did not complete")
	}
	// The connection must fail due to server closure, not the read deadline.
	for {
		_, _, err = ws.ReadMessage()
		if err != nil {
			break
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatal("client only timed out; server did not close websocket")
	}
	if err = store.db.Ping(ctx); err != nil {
		t.Fatal("database closed before final persistence verification")
	}
	var saved []RoomRecord
	if !loseOwner {
		saved, err = store.loadRooms(ctx)
	}
	if !loseOwner && (err != nil || len(saved) != 1 || saved[0].State.Coins != 7654) {
		t.Fatal("final authoritative world was not persisted before database close")
	}
	store.close()
	reopened, err := openStore(u.String(), "")
	if err != nil {
		t.Fatal("test store reopen failed")
	}
	defer reopened.close()
	restored, err := reopened.loadRooms(ctx)
	if err != nil || len(restored) != 1 || restored[0].State.Coins != 7654 {
		t.Fatal("final world did not survive database pool close and reopen")
	}
}
