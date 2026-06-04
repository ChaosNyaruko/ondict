package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ChaosNyaruko/ondict/dbutil"
)

// SQLiteCursorStore persists sync cursors (last-pull / last-push timestamps)
// in the meta table of any ondict-managed SQLite database. Typically
// callers point this at the wordbank.db so the cursor lives next to the
// data it tracks; using a dedicated cursor.db is fine too.
type SQLiteCursorStore struct {
	db *sql.DB
}

// OpenSQLiteCursorStore wraps an existing SQLiteWordbank or SQLiteHistory's
// underlying *sql.DB. Callers are expected to share the same *sql.DB to
// avoid SQLite write lock contention.
func NewCursorStore(db *sql.DB) *SQLiteCursorStore {
	return &SQLiteCursorStore{db: db}
}

// CursorDB returns the underlying *sql.DB so adjacent components can share it.
func (s *SQLiteCursorStore) CursorDB() *sql.DB { return s.db }

// GetCursor implements syncclient.CursorStore.
func (s *SQLiteCursorStore) GetCursor(_ context.Context, key string) (string, error) {
	v, err := dbutil.GetMeta(s.db, key)
	if err != nil {
		return "", fmt.Errorf("read cursor %q: %w", key, err)
	}
	return v, nil
}

// SetCursor implements syncclient.CursorStore.
func (s *SQLiteCursorStore) SetCursor(_ context.Context, key, value string) error {
	if err := dbutil.SetMeta(s.db, key, value); err != nil {
		return fmt.Errorf("write cursor %q: %w", key, err)
	}
	return nil
}

// DB exposes the underlying *sql.DB on a SQLiteWordbank, used by the sync
// client to share the connection with a cursor store.
func (w *SQLiteWordbank) DB() *sql.DB { return w.db }

// DB exposes the underlying *sql.DB on a SQLiteHistory.
func (h *SQLiteHistory) DB() *sql.DB { return h.db }
