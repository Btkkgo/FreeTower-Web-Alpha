package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var errWorldWriterUnavailable = errors.New("FreeTower world writer unavailable")

// A public world has one physical PostgreSQL session for both its advisory lock
// and every durable world mutation. A lost session can never fall back to pool.
type worldWriterState struct {
	mu          sync.Mutex
	required    bool
	failed      bool
	conn        *pgx.Conn
	databaseKey int32
	schemaKey   int32
	lost        chan struct{}
}

func (s *Store) acquireWorldWriter(ctx context.Context) error {
	w := &s.worldWriter
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.required {
		return errWorldWriterUnavailable
	}
	w.required = true
	w.lost = make(chan struct{})
	if s.db == nil {
		s.failWorldWriterLocked()
		return errWorldWriterUnavailable
	}
	pooled, err := s.db.Acquire(ctx)
	if err != nil {
		s.failWorldWriterLocked()
		return errWorldWriterUnavailable
	}
	// Hijack permanently removes this session from the pool. It must be closed,
	// never released back to the pool with a session advisory lock still held.
	w.conn = pooled.Hijack()
	acquired := false
	err = w.conn.QueryRow(ctx, "SELECT hashtext(current_database()), hashtext(current_schema() || ':freetower-world')").Scan(&w.databaseKey, &w.schemaKey)
	if err == nil {
		err = w.conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1::integer,$2::integer)", w.databaseKey, w.schemaKey).Scan(&acquired)
	}
	if err != nil || !acquired {
		s.failWorldWriterLocked()
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.conn.Close(closeCtx)
		return errWorldWriterUnavailable
	}
	return nil
}

// Called only with worldWriter.mu held. It signals lifecycle code without
// calling App or room functions while holding the store mutex.
func (s *Store) failWorldWriterLocked() {
	w := &s.worldWriter
	if !w.failed {
		w.failed = true
		if w.lost != nil {
			close(w.lost)
		}
	}
}
func (s *Store) checkWorldWriterLocked(ctx context.Context) error {
	w := &s.worldWriter
	if !w.required {
		return nil
	}
	if w.failed || w.conn == nil || w.conn.IsClosed() {
		s.failWorldWriterLocked()
		return errWorldWriterUnavailable
	}
	held := false
	err := w.conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND pid=pg_backend_pid() AND classid=$1::oid AND objid=$2::oid AND objsubid=2 AND granted)`, int64(uint32(w.databaseKey)), int64(uint32(w.schemaKey))).Scan(&held)
	if err != nil || !held {
		if err != nil && ctx.Err() != nil && !w.conn.IsClosed() {
			return errWorldWriterUnavailable
		}
		s.failWorldWriterLocked()
		return errWorldWriterUnavailable
	}
	return nil
}
func (s *Store) worldWriterHealthy(ctx context.Context) error {
	s.worldWriter.mu.Lock()
	defer s.worldWriter.mu.Unlock()
	return s.checkWorldWriterLocked(ctx)
}
func (s *Store) worldWriterClaimed() bool {
	s.worldWriter.mu.Lock()
	defer s.worldWriter.mu.Unlock()
	return s.worldWriter.required
}
func (s *Store) worldWriterActive() bool {
	s.worldWriter.mu.Lock()
	defer s.worldWriter.mu.Unlock()
	return s.worldWriter.required && !s.worldWriter.failed
}
func (s *Store) worldWriterFailure() <-chan struct{} {
	s.worldWriter.mu.Lock()
	defer s.worldWriter.mu.Unlock()
	return s.worldWriter.lost
}
func (s *Store) closeWorldWriter() {
	w := &s.worldWriter
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.required {
		return
	}
	s.failWorldWriterLocked()
	if w.conn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.conn.Close(ctx)
	}
}
func (s *Store) worldExec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	w := &s.worldWriter
	w.mu.Lock()
	if !w.required {
		w.mu.Unlock()
		return s.db.Exec(ctx, query, args...)
	}
	defer w.mu.Unlock()
	if err := s.checkWorldWriterLocked(ctx); err != nil {
		return pgconn.CommandTag{}, err
	}
	result, err := w.conn.Exec(ctx, query, args...)
	if err != nil && w.conn.IsClosed() {
		s.failWorldWriterLocked()
		return result, errWorldWriterUnavailable
	}
	return result, err
}
func (s *Store) loadDatabaseRooms(ctx context.Context) ([]RoomRecord, error) {
	w := &s.worldWriter
	w.mu.Lock()
	var rows pgx.Rows
	var err error
	if w.required {
		defer func() {
			if w.conn != nil && w.conn.IsClosed() {
				s.failWorldWriterLocked()
			}
			w.mu.Unlock()
		}()
		if err = s.checkWorldWriterLocked(ctx); err != nil {
			return nil, err
		}
		rows, err = w.conn.Query(ctx, "SELECT data FROM sheep_rooms ORDER BY updated_at DESC")
	} else {
		w.mu.Unlock()
		rows, err = s.db.Query(ctx, "SELECT data FROM sheep_rooms ORDER BY updated_at DESC")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoomRecord{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var r RoomRecord
		if err = json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (a *App) watchWorldWriter() {
	defer close(a.worldWriterDone)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-a.worldWriterLost:
			log.Print("FreeTower world writer lost; world access disabled")
			a.identity.mu.Lock()
			a.closing = true
			a.identity.mu.Unlock()
			a.mu.RLock()
			rooms := make([]*RuntimeRoom, 0, len(a.rooms))
			for _, rr := range a.rooms {
				rooms = append(rooms, rr)
			}
			a.mu.RUnlock()
			for _, rr := range rooms {
				rr.mu.Lock()
				for c := range rr.clients {
					c.close()
				}
				rr.mu.Unlock()
			}
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = a.store.worldWriterHealthy(ctx)
			cancel()
		}
	}
}
