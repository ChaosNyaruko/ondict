package sources

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, dbPath
}

func TestIsDumpComplete_NotExist(t *testing.T) {
	assert.False(t, IsDumpComplete("/nonexistent/path/vocab.db"))
}

func TestIsDumpComplete_InProgress(t *testing.T) {
	db, dbPath := openTestDB(t)
	require.NoError(t, setDumpStatus(db, "in_progress"))
	_ = db.Close()
	assert.False(t, IsDumpComplete(dbPath))
}

func TestIsDumpComplete_Done(t *testing.T) {
	db, dbPath := openTestDB(t)
	require.NoError(t, setDumpStatus(db, "done"))
	_ = db.Close()
	assert.True(t, IsDumpComplete(dbPath))
}

func TestSetDumpStatus_IdempotentUpsert(t *testing.T) {
	db, _ := openTestDB(t)
	require.NoError(t, setDumpStatus(db, "in_progress"))
	require.NoError(t, setDumpStatus(db, "done"))

	var val string
	require.NoError(t, db.QueryRow(`SELECT value FROM search_meta WHERE key=?`, dumpStatusKey).Scan(&val))
	assert.Equal(t, "done", val)
}

func TestResetVocabTable_CreatesTable(t *testing.T) {
	db, _ := openTestDB(t)
	require.NoError(t, resetVocabTable(db))

	// Insert a row to verify the table exists and has the right schema.
	_, err := db.Exec(`INSERT INTO vocab(word, src, def) VALUES('test', 'src', 'def')`)
	assert.NoError(t, err)
}

func TestResetVocabTable_DropsExisting(t *testing.T) {
	db, _ := openTestDB(t)
	require.NoError(t, resetVocabTable(db))
	_, err := db.Exec(`INSERT INTO vocab(word, src, def) VALUES('old', 'src', 'old def')`)
	require.NoError(t, err)

	// A second call should drop and re-create.
	require.NoError(t, resetVocabTable(db))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM vocab`).Scan(&count))
	assert.Equal(t, 0, count, "table should be empty after reset")
}

func TestDumpMDXFilesToSQLite_WithTestdata(t *testing.T) {
	mdxPath := "../testdata/Longman Dictionary of Contemporary English.mdx"
	if _, err := os.Stat(mdxPath); os.IsNotExist(err) {
		t.Skip("testdata MDX not found")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vocab.db")

	err := DumpMDXFilesToSQLite(dbPath, []string{mdxPath}, 50, DefinitionTokenizerUnicode61)
	require.NoError(t, err)

	assert.True(t, IsDumpComplete(dbPath))

	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	require.NoError(t, err)
	defer db.Close()

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM vocab`).Scan(&count))
	assert.Greater(t, count, 0)
}

func TestDumpSingleMDXToSQLite_WithTestdata(t *testing.T) {
	mdxPath := "../testdata/Longman Dictionary of Contemporary English.mdx"
	if _, err := os.Stat(mdxPath); os.IsNotExist(err) {
		t.Skip("testdata MDX not found")
	}

	db, _ := openTestDB(t)
	require.NoError(t, resetVocabTable(db))

	err := dumpSingleMDXToSQLite(db, mdxPath, 10)
	require.NoError(t, err)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM vocab`).Scan(&count))
	assert.Greater(t, count, 0)
	assert.LessOrEqual(t, count, 10)
}
