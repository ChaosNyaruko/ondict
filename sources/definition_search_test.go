package sources

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

func TestBuildDefinitionSearchIndexAndQuery(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	require.NoError(t, os.Setenv("HOME", tmpHome))
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})

	dbPath := filepath.Join(tmpHome, ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, err = db.Exec(`
CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "", def_text TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word, src, def) VALUES
	('doctor', 'dict-a', '<div>someone who treats kidney problems</div>'),
	('nurse', 'dict-a', '<div>someone who helps patients recover</div>');
`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE vocab SET def_text = CASE word
		WHEN 'doctor' THEN 'someone who treats kidney problems'
		WHEN 'nurse' THEN 'someone who helps patients recover'
		ELSE ''
	END`)
	require.NoError(t, err)
	require.NoError(t, BuildDefinitionSearchIndex(db, DefinitionTokenizerUnicode61))

	matches, err := SearchDefinitions("kidney", 10)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "doctor", matches[0].Word)
	require.Contains(t, matches[0].Snippet, "kidney")
}

func TestExtractVisibleText(t *testing.T) {
	got := extractVisibleText(`<div>Hello <strong>world</strong><script>alert(1)</script><br>again</div>`)
	require.Equal(t, "Hello world again", got)
}

func TestEnableFastIndexBuildModeRestoresPragmas(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	require.NoError(t, os.Setenv("HOME", tmpHome))
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})

	dbPath := filepath.Join(tmpHome, ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	var beforeSync, beforeTemp, beforeCache int
	var beforeJournal string
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA synchronous`, &beforeSync))
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA temp_store`, &beforeTemp))
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA cache_size`, &beforeCache))
	require.NoError(t, queryStringContext(ctx, conn, `PRAGMA journal_mode`, &beforeJournal))

	restore, err := enableFastIndexBuildMode(ctx, conn)
	require.NoError(t, err)

	var fastSync, fastTemp, fastCache int
	var fastJournal string
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA synchronous`, &fastSync))
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA temp_store`, &fastTemp))
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA cache_size`, &fastCache))
	require.NoError(t, queryStringContext(ctx, conn, `PRAGMA journal_mode`, &fastJournal))

	require.Equal(t, 0, fastSync)
	require.Equal(t, 2, fastTemp)
	require.Equal(t, -65536, fastCache)
	require.Equal(t, "memory", fastJournal)

	restore()

	var afterSync, afterTemp, afterCache int
	var afterJournal string
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA synchronous`, &afterSync))
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA temp_store`, &afterTemp))
	require.NoError(t, queryIntContext(ctx, conn, `PRAGMA cache_size`, &afterCache))
	require.NoError(t, queryStringContext(ctx, conn, `PRAGMA journal_mode`, &afterJournal))

	require.Equal(t, beforeSync, afterSync)
	require.Equal(t, beforeTemp, afterTemp)
	require.Equal(t, beforeCache, afterCache)
	require.Equal(t, beforeJournal, afterJournal)
}
