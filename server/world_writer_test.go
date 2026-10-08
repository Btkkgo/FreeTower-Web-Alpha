package main

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func worldWriterTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	schema := "ft006_writer_" + newID()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal("test schema creation failed")
	}
	t.Cleanup(func() {
		defer admin.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error("test schema cleanup failed")
		}
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal("test database configuration invalid")
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	store := openWriterTestStore(t, u.String())
	return store, u.String()
}
func openWriterTestStore(t *testing.T, dsn string) *Store {
	t.Helper()
	s, err := openStore(dsn, "")
	if err != nil {
		t.Fatal("test store initialization failed")
	}
	t.Cleanup(s.close)
	return s
}
func publicWriterTestApp(t *testing.T, s *Store) *App {
	t.Helper()
	a, err := newAppWithConfig(s, t.TempDir(), RuntimeConfig{Environment: "production", CanonicalEnvironment: true, PublicOrigin: "https://example.test", RequireDatabase: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.shutdown)
	return a
}
func writerTestRecord() RoomRecord {
	return RoomRecord{Room: Room{ID: newID(), Code: "WRITER", Name: "Writer world"}, Members: map[string]bool{}, State: initialState(nowMS())}
}

func TestPublicWorldWriterSingleOwnerThroughFinalSave(t *testing.T) {
	s, dsn := worldWriterTestStore(t)
	record := writerTestRecord()
	if err := s.saveRoom(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	app := publicWriterTestApp(t, s)
	for _, store := range []*Store{s, openWriterTestStore(t, dsn)} {
		if _, err := newAppWithConfig(store, t.TempDir(), app.runtime); err == nil {
			t.Fatal("concurrent public app accepted")
		}
	}
	if _, err := newAppWithConfig(s, t.TempDir(), RuntimeConfig{Environment: "development"}); err == nil {
		t.Fatal("second app bypassed same-store ownership through local mode")
	}
	// Ownership is scoped to the effective schema, not every FreeTower test database.
	other, _ := worldWriterTestStore(t)
	publicWriterTestApp(t, other)
	rr := app.rooms[record.Room.ID]
	rr.mu.Lock()
	rr.record.State.Coins = 4567
	rr.mu.Unlock()
	app.shutdown()
	if _, err := newAppWithConfig(openWriterTestStore(t, dsn), t.TempDir(), app.runtime); err == nil {
		t.Fatal("lock released before database close")
	}
	s.close()
	successor := publicWriterTestApp(t, openWriterTestStore(t, dsn))
	saved := successor.rooms[record.Room.ID]
	saved.mu.Lock()
	coins := saved.record.State.Coins
	saved.mu.Unlock()
	if coins != 4567 {
		t.Fatal("successor loaded stale world before final save")
	}
}
func TestPublicWorldWriterConnectionLossNeverFallsBack(t *testing.T) {
	s, dsn := worldWriterTestStore(t)
	record := writerTestRecord()
	if err := s.saveRoom(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	app := publicWriterTestApp(t, s)
	s.worldWriter.mu.Lock()
	pid := s.worldWriter.conn.PgConn().PID()
	s.worldWriter.mu.Unlock()
	if _, err := s.db.Exec(context.Background(), "SELECT pg_terminate_backend($1)", pid); err != nil {
		t.Fatal("test writer termination failed")
	}
	record.State.Coins = 9999
	if err := s.saveRoom(context.Background(), record); err == nil {
		t.Fatal("lost writer fell back to pool for save")
	}
	if err := s.deleteRoom(context.Background(), record.Room.ID); err == nil {
		t.Fatal("lost writer fell back to pool for delete")
	}
	if err := s.db.Ping(context.Background()); err != nil {
		t.Fatal("pool must remain live for this regression")
	}
	w := httptest.NewRecorder()
	app.routes().ServeHTTP(w, httptest.NewRequest("GET", "/api/health", nil))
	if w.Code != 503 {
		t.Fatal("lost owner reported healthy because pool was live")
	}
	w = httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/rooms", nil)
	r.Header.Set("Origin", app.runtime.PublicOrigin)
	app.routes().ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("lost owner still served world mutation")
	}
	select {
	case <-app.tickerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("world ticker did not stop after writer loss")
	}
	successorStore := openWriterTestStore(t, dsn)
	successor := publicWriterTestApp(t, successorStore)
	record.State.Coins = 6789
	if err := successorStore.saveRoom(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := s.saveRoom(context.Background(), record); err == nil {
		t.Fatal("failed owner reacquired authority")
	}
	app.shutdown()
	saved, err := successorStore.loadRooms(context.Background())
	if err != nil || len(saved) != 1 || saved[0].State.Coins != 6789 {
		t.Fatal("old owner shutdown overwrote successor world")
	}
	_ = successor
}

func TestPublicWorldWriterLostLockAndCanceledProbe(t *testing.T) {
	s, _ := worldWriterTestStore(t)
	app := publicWriterTestApp(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.worldWriterHealthy(ctx); err == nil {
		t.Fatal("canceled probe succeeded")
	}
	if !s.worldWriterActive() {
		t.Fatal("canceled probe revoked a still-live owner")
	}
	s.worldWriter.mu.Lock()
	_, err := s.worldWriter.conn.Exec(context.Background(), "SELECT pg_advisory_unlock_all()")
	s.worldWriter.mu.Unlock()
	if err != nil {
		t.Fatal("test lock release failed")
	}
	if err = s.worldWriterHealthy(context.Background()); err == nil {
		t.Fatal("lost advisory lock reported healthy")
	}
	if err = s.saveRoom(context.Background(), writerTestRecord()); err == nil {
		t.Fatal("connection without ownership wrote world")
	}
	app.shutdown()
}
