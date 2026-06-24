package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

func TestRebuildWordsTableV4_WithData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wb.db")
	db, err := sql.Open("sqlite3", "file:"+path)
	require.NoError(t, err)
	defer db.Close()

	// Create a minimal "words" table matching the schema before v4 rebuild.
	_, err = db.Exec(`CREATE TABLE words (
		word           TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
		create_time    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		update_time    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at     DATETIME,
		server_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO words(word) VALUES ('mango'), ('banana')`)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, rebuildWordsTableV4(tx))
	require.NoError(t, tx.Commit())

	// Verify rows were migrated.
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&count))
	require.Equal(t, 2, count)
}

func TestRebuildHistoryTableV4_WithData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hist.db")
	db, err := sql.Open("sqlite3", "file:"+path)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE history (" +
		"word TEXT NOT NULL UNIQUE, " +
		"`count` INTEGER NOT NULL DEFAULT 0, " +
		"create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, " +
		"update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, " +
		"deleted_at DATETIME, " +
		"server_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP" +
		")")
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO history(word, `count`) VALUES ('apple', 1), ('cherry', 2)")
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, rebuildHistoryTableV4(tx))
	require.NoError(t, tx.Commit())

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM history`).Scan(&count))
	require.Equal(t, 2, count)
}
