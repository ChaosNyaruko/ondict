package dbutil

import (
	"database/sql"
	"fmt"
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

func TestGetMetaAndSetMeta(t *testing.T) {
	db := openTempDB(t)
	// GetMeta on a missing key returns ("", nil)
	v, err := GetMeta(db, "cursor")
	require.NoError(t, err)
	require.Equal(t, "", v)

	// SetMeta persists a value
	require.NoError(t, SetMeta(db, "cursor", "2024-01-01T00:00:00Z"))
	v, err = GetMeta(db, "cursor")
	require.NoError(t, err)
	require.Equal(t, "2024-01-01T00:00:00Z", v)

	// SetMeta upserts (overwrites)
	require.NoError(t, SetMeta(db, "cursor", "2025-06-01T12:00:00Z"))
	v, err = GetMeta(db, "cursor")
	require.NoError(t, err)
	require.Equal(t, "2025-06-01T12:00:00Z", v)

	// Multiple distinct keys coexist
	require.NoError(t, SetMeta(db, "other", "hello"))
	v, err = GetMeta(db, "cursor")
	require.NoError(t, err)
	require.Equal(t, "2025-06-01T12:00:00Z", v)
}

var errSimulated = simulatedError("boom")

type simulatedError string

func (e simulatedError) Error() string { return string(e) }

func TestColumnExists_Present(t *testing.T) {
	db := openTempDB(t)
	// The meta table has a "key" column.
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// Create a simple test table.
	_, err = tx.Exec(`CREATE TABLE test_col (id INTEGER, name TEXT)`)
	require.NoError(t, err)

	ok, err := ColumnExists(tx, "test_col", "name")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestColumnExists_Absent(t *testing.T) {
	db := openTempDB(t)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	_, err = tx.Exec(`CREATE TABLE test_col2 (id INTEGER)`)
	require.NoError(t, err)

	ok, err := ColumnExists(tx, "test_col2", "nonexistent_column")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestApplyOne_Success(t *testing.T) {
	db := openTempDB(t)
	require.NoError(t, EnsureMetaTable(db))
	m := Migration{
		Version: 99,
		Name:    "test migration",
		Apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS test_apply (id INTEGER)`)
			return err
		},
	}
	require.NoError(t, applyOne(db, m))

	// The meta table should now show version 99.
	v, err := GetMeta(db, SchemaKey)
	require.NoError(t, err)
	require.Equal(t, "99", v)
}

func TestApplyOne_ApplyError(t *testing.T) {
	db := openTempDB(t)
	m := Migration{
		Version: 100,
		Name:    "failing migration",
		Apply: func(tx *sql.Tx) error {
			return errSimulated
		},
	}
	err := applyOne(db, m)
	require.Error(t, err)
}

func TestCurrentVersion_WithVersion(t *testing.T) {
	db := openTempDB(t)
	require.NoError(t, EnsureMetaTable(db))

	// Set a version manually.
	_, err := db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)`, SchemaKey, "5")
	require.NoError(t, err)

	v, err := CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 5, v)
}

func TestCurrentVersion_NoRows(t *testing.T) {
	db := openTempDB(t)
	// Fresh DB with meta table but no schema_version row.
	v, err := CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 0, v)
}

func TestEnsureSchema_AppliesMigrations(t *testing.T) {
	db := openTempDB(t)
	migrations := []Migration{
		{Version: 1, Name: "create t1", Apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE t1 (id INTEGER)`)
			return err
		}},
		{Version: 2, Name: "create t2", Apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE t2 (id INTEGER)`)
			return err
		}},
	}

	require.NoError(t, EnsureSchema(db, migrations))

	v, err := CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 2, v)

	// Run again: idempotent.
	require.NoError(t, EnsureSchema(db, migrations))
	v, err = CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 2, v)
}

func TestApplyOne_CommitPath(t *testing.T) {
	db := openTempDB(t)
	require.NoError(t, EnsureMetaTable(db))

	m := Migration{
		Version: 1,
		Name:    "create foo",
		Apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE foo (id INTEGER)`)
			return err
		},
	}
	require.NoError(t, applyOne(db, m))

	v, err := CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 1, v)
}

func TestApplyOne_RollbackOnApplyError(t *testing.T) {
	db := openTempDB(t)
	require.NoError(t, EnsureMetaTable(db))

	m := Migration{
		Version: 1,
		Name:    "bad migration",
		Apply: func(tx *sql.Tx) error {
			return fmt.Errorf("intentional error")
		},
	}
	err := applyOne(db, m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "intentional error")

	// Version should still be 0.
	v, err := CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 0, v)
}

func TestCurrentVersion_InvalidFormat(t *testing.T) {
	db := openTempDB(t)
	require.NoError(t, EnsureMetaTable(db))

	// Store a non-numeric value → Sscanf should fail.
	_, err := db.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)`, SchemaKey, "not-a-number")
	require.NoError(t, err)

	_, err = CurrentVersion(db)
	require.Error(t, err)
}

func TestGetMeta_ExistingKey(t *testing.T) {
	db := openTempDB(t)
	require.NoError(t, SetMeta(db, "mykey", "myvalue"))

	v, err := GetMeta(db, "mykey")
	require.NoError(t, err)
	require.Equal(t, "myvalue", v)
}

func TestSetMeta_Upsert(t *testing.T) {
	db := openTempDB(t)
	require.NoError(t, SetMeta(db, "k", "v1"))
	require.NoError(t, SetMeta(db, "k", "v2")) // upsert
	v, err := GetMeta(db, "k")
	require.NoError(t, err)
	require.Equal(t, "v2", v)
}
