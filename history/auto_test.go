package history

import (
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/ChaosNyaruko/ondict/dbutil"
	"github.com/ChaosNyaruko/ondict/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTxtWriter_Append(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	w := NewTxtWriter()
	err := w.Append("testword")
	assert.NoError(t, err)

	content, err := os.ReadFile(util.HistoryTable())
	assert.NoError(t, err)
	assert.Contains(t, string(content), "testword")
}

func TestSqlite3Writer_Append(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	w := NewSqlite3Writer()
	err := w.Append("testword")
	assert.NoError(t, err)

	// Check if db exists
	_, err = os.Stat(util.HistoryDB())
	assert.NoError(t, err)
}

func TestHistory_Append(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	txtW := NewTxtWriter()
	sqlW := NewSqlite3Writer()
	h := NewHistory(txtW, sqlW)

	err := h.Append("bothword")
	assert.NoError(t, err)

	// Verify txt
	content, err := os.ReadFile(util.HistoryTable())
	assert.NoError(t, err)
	assert.Contains(t, string(content), "bothword")
}

func TestHistory_Review(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// We need to write some data first
	sqlW := NewSqlite3Writer()
	h := NewHistory(sqlW)

	err := h.Append("reviewword")
	assert.NoError(t, err)
	time.Sleep(100 * time.Millisecond) // Ensure DB write

	// Review checks for words updated > -days and count >= count
	// Append adds with count 1 and current time.
	// So -1 days and count 1 should match.
	res, err := h.Review("1", "1")
	assert.NoError(t, err)
	assert.Contains(t, res, "reviewword")
}

// TestSqlite3Writer_AppendIncrementsCount makes sure the upsert path bumps
// `count` on subsequent queries (regression guard for the v2 migration).
func TestSqlite3Writer_AppendIncrementsCount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	w := NewSqlite3Writer()
	require.NoError(t, w.Append("doctor"))
	require.NoError(t, w.Append("doctor"))
	require.NoError(t, w.Append("doctor"))

	db, err := sql.Open("sqlite3", "file:"+util.HistoryDB())
	require.NoError(t, err)
	defer db.Close()

	var count int
	require.NoError(t, db.QueryRow(`SELECT `+"`count`"+` FROM history WHERE word = ?`, "doctor").Scan(&count))
	require.Equal(t, 3, count)
}

// TestHistoryMigration_LegacyLocaltimeRowsRewrittenToUTC seeds a v0-shaped DB
// (the original create-table-on-each-insert flow), then triggers EnsureSchema
// and verifies the legacy "YYYY-MM-DD HH:MM:SS" localtime strings get
// rewritten to RFC3339 UTC.
func TestHistoryMigration_LegacyLocaltimeRowsRewrittenToUTC(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dbPath := util.HistoryDB()
	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)

	// Seed a v0 database manually – the original pre-migration shape.
	_, err = db.Exec(`CREATE TABLE history (
		word TEXT NOT NULL UNIQUE,
		` + "`count`" + ` INTEGER NOT NULL DEFAULT 0,
		create_time DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
		update_time DATETIME NOT NULL DEFAULT (datetime('now','localtime'))
	)`)
	require.NoError(t, err)

	// Insert a row with a hand-crafted localtime string we can predict.
	const legacy = "2025-02-15 18:00:27"
	_, err = db.Exec(`INSERT INTO history (word, `+"`count`"+`, create_time, update_time) VALUES (?, ?, ?, ?)`,
		"legacyword", 4, legacy, legacy)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	// Trigger migration via the public Append path (which calls EnsureSchema).
	w := NewSqlite3Writer()
	require.NoError(t, w.Append("anotherword"))

	db, err = sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	defer db.Close()

	v, err := dbutil.CurrentVersion(db)
	require.NoError(t, err)
	require.GreaterOrEqual(t, v, 2, "schema must be at v2 after Append")

	var ct, ut string
	var deleted sql.NullString
	require.NoError(t, db.QueryRow(
		`SELECT create_time, update_time, deleted_at FROM history WHERE word = ?`, "legacyword",
	).Scan(&ct, &ut, &deleted))

	wantUTC := time.Date(2025, 2, 15, 18, 0, 27, 0, time.Local).UTC().Format(time.RFC3339)
	require.Equal(t, wantUTC, ct, "create_time should be rewritten to UTC RFC3339")
	require.Equal(t, wantUTC, ut, "update_time should be rewritten to UTC RFC3339")
	require.False(t, deleted.Valid, "legacy rows must not be tombstoned by migration")
}

// TestHistoryMigration_IsIdempotent makes sure running EnsureSchema twice
// (which happens on every Append) doesn't blow up or rewrite UTC rows.
func TestHistoryMigration_IsIdempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	w := NewSqlite3Writer()
	require.NoError(t, w.Append("apple"))
	require.NoError(t, w.Append("apple"))
	require.NoError(t, w.Append("apple"))

	db, err := sql.Open("sqlite3", "file:"+util.HistoryDB())
	require.NoError(t, err)
	defer db.Close()

	v, err := dbutil.CurrentVersion(db)
	require.NoError(t, err)
	require.Equal(t, 4, v)
}
