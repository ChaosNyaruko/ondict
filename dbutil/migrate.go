// Package dbutil contains shared SQLite helpers used by the wordbank and
// history stores. The most important piece is the migration framework, which
// applies idempotent, forward-only schema changes keyed off a per-database
// `meta` table.
//
// Design notes are in docs/adr/0001-wordbank-history-sync.md.
package dbutil

import (
	"database/sql"
	"errors"
	"fmt"
)

// SchemaKey is the key under which the schema version is recorded in the
// per-database `meta` table.
const SchemaKey = "schema_version"

// Migration is a single forward-only schema delta. Each migration is applied
// inside a transaction; if Apply returns an error the transaction is rolled
// back and the schema version is not bumped.
//
// Version numbers must be strictly increasing across the slice passed to
// EnsureSchema and must start at 1.
type Migration struct {
	Version int
	Name    string
	Apply   func(tx *sql.Tx) error
}

// EnsureMetaTable creates the bookkeeping `meta(key TEXT PK, value TEXT)`
// table used to record the current schema version. Safe to call repeatedly.
func EnsureMetaTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`)
	return err
}

// CurrentVersion returns the schema_version recorded in the meta table, or 0
// if no row exists yet.
func CurrentVersion(db *sql.DB) (int, error) {
	if err := EnsureMetaTable(db); err != nil {
		return 0, err
	}
	var raw string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = ?`, SchemaKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	var v int
	if _, err := fmt.Sscanf(raw, "%d", &v); err != nil {
		return 0, fmt.Errorf("parse schema version %q: %w", raw, err)
	}
	return v, nil
}

// EnsureSchema brings the database forward through every Migration whose
// Version is strictly greater than the currently recorded version. Each
// migration runs inside its own transaction; the schema_version is bumped
// inside the same transaction, so partial applies cannot leave a database
// in an in-between state.
//
// Migrations must:
//   - have strictly increasing Version numbers starting at 1;
//   - be idempotent OR guarded by their own existence checks (e.g.
//     `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE` wrapped in a
//     PRAGMA-table check); since the version gate already short-circuits
//     re-runs, simple non-idempotent SQL is also fine.
func EnsureSchema(db *sql.DB, migrations []Migration) error {
	if err := EnsureMetaTable(db); err != nil {
		return err
	}
	current, err := CurrentVersion(db)
	if err != nil {
		return err
	}

	// Sanity-check ordering and starting point.
	expected := 1
	for _, m := range migrations {
		if m.Version != expected {
			return fmt.Errorf("migration version sequence broken: got %d, expected %d (%q)", m.Version, expected, m.Name)
		}
		expected++
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}
		if err := applyOne(db, m); err != nil {
			return fmt.Errorf("apply migration v%d %q: %w", m.Version, m.Name, err)
		}
	}
	return nil
}

func applyOne(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := m.Apply(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		SchemaKey, fmt.Sprintf("%d", m.Version),
	); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ColumnExists reports whether a column is present on a table. Useful for
// guard logic inside ALTER-style migrations that may run against either a
// fresh or pre-existing database.
func ColumnExists(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query(fmt.Sprintf(`PRAGMA table_info(%q)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// GetMeta reads a value from the meta table. Returns ("", nil) when the key
// is absent. The meta table is expected to exist already (created by
// EnsureMetaTable / EnsureSchema).
func GetMeta(db *sql.DB, key string) (string, error) {
	if err := EnsureMetaTable(db); err != nil {
		return "", err
	}
	var v string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

// SetMeta writes a value into the meta table (UPSERT).
func SetMeta(db *sql.DB, key, value string) error {
	if err := EnsureMetaTable(db); err != nil {
		return err
	}
	_, err := db.Exec(
		`INSERT INTO meta (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
