package dbutil

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEnsureSchema_AppliesAndIsIdempotent(t *testing.T) {
	db := openTempDB(t)

	var applied []int
	migrations := []Migration{
		{Version: 1, Name: "create_t", Apply: func(tx *sql.Tx) error {
			applied = append(applied, 1)
			_, err := tx.Exec(`CREATE TABLE t (x INTEGER)`)
			return err
		}},
		{Version: 2, Name: "add_y", Apply: func(tx *sql.Tx) error {
			applied = append(applied, 2)
			_, err := tx.Exec(`ALTER TABLE t ADD COLUMN y TEXT`)
			return err
		}},
	}

	require.NoError(t, EnsureSchema(db, migrations))
	require.Equal(t, []int{1, 2}, applied)

	v, err := CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 2, v)

	// Second run is a no-op.
	require.NoError(t, EnsureSchema(db, migrations))
	require.Equal(t, []int{1, 2}, applied, "migrations re-applied unexpectedly")
}

func TestEnsureSchema_RejectsNonContiguousVersions(t *testing.T) {
	db := openTempDB(t)
	migrations := []Migration{
		{Version: 1, Name: "a", Apply: func(tx *sql.Tx) error { return nil }},
		{Version: 3, Name: "c", Apply: func(tx *sql.Tx) error { return nil }},
	}
	require.Error(t, EnsureSchema(db, migrations))
}

func TestEnsureSchema_RollsBackOnFailure(t *testing.T) {
	db := openTempDB(t)
	migrations := []Migration{
		{Version: 1, Name: "create", Apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE t (x INTEGER)`)
			return err
		}},
		{Version: 2, Name: "boom", Apply: func(tx *sql.Tx) error {
			_, _ = tx.Exec(`CREATE TABLE u (z INTEGER)`)
			return errSimulated
		}},
	}
	require.Error(t, EnsureSchema(db, migrations))

	v, err := CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 1, v, "schema_version must not advance past failed migration")

	// Table from the failed migration must not exist.
	var exists int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='u'`).Scan(&exists)
	require.NoError(t, err)
	require.Equal(t, 0, exists)
}

func TestColumnExists(t *testing.T) {
	db := openTempDB(t)
	_, err := db.Exec(`CREATE TABLE t (a INTEGER, b TEXT)`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	got, err := ColumnExists(tx, "t", "a")
	require.NoError(t, err)
	require.True(t, got)

	got, err = ColumnExists(tx, "t", "missing")
	require.NoError(t, err)
	require.False(t, got)
}

var errSimulated = simulatedError("boom")

type simulatedError string

func (e simulatedError) Error() string { return string(e) }
