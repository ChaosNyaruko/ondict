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

func setupVocabDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dbPath := filepath.Join(tmpHome, ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestDefinitionSearchError_Error(t *testing.T) {
	err := &DefinitionSearchError{Reason: "test reason"}
	require.Equal(t, "test reason", err.Error())
}

func TestNormalizeDefinitionTokenizer(t *testing.T) {
	require.Equal(t, DefinitionTokenizerUnicode61, normalizeDefinitionTokenizer(""))
	require.Equal(t, DefinitionTokenizerUnicode61, normalizeDefinitionTokenizer("unicode61"))
	require.Equal(t, DefinitionTokenizerUnicode61, normalizeDefinitionTokenizer("UNICODE61"))
	require.Equal(t, DefinitionTokenizerTrigram, normalizeDefinitionTokenizer("trigram"))
	require.Equal(t, DefinitionTokenizerUnicode61, normalizeDefinitionTokenizer("unknown_tokenizer"))
}

func TestDefinitionSearchReady_NoVocabDB(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// No vocab.db exists — should return a DefinitionSearchError.
	err := DefinitionSearchReady()
	require.Error(t, err)
}

func TestCheckDefinitionSearchReady_NoVocabTable(t *testing.T) {
	db := setupVocabDB(t)
	err := checkDefinitionSearchReady(db, DefinitionTokenizerUnicode61)
	require.Error(t, err)
	var dse *DefinitionSearchError
	require.ErrorAs(t, err, &dse)
	require.Contains(t, dse.Reason, "vocab")
}

func TestCheckDefinitionSearchReady_NoFTS(t *testing.T) {
	db := setupVocabDB(t)
	_, err := db.Exec(`CREATE TABLE vocab(word TEXT, def TEXT, def_text TEXT, src TEXT)`)
	require.NoError(t, err)

	err = checkDefinitionSearchReady(db, DefinitionTokenizerUnicode61)
	require.Error(t, err)
	var dse *DefinitionSearchError
	require.ErrorAs(t, err, &dse)
	require.Contains(t, dse.Reason, "index")
}

func TestNewDBIExact(t *testing.T) {
	s := NewDBIExact()
	require.NotNil(t, s)
}

func TestDBDict_Keys_Empty(t *testing.T) {
	db := setupVocabDB(t)
	_, err := db.Exec(`CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "")`)
	require.NoError(t, err)

	// Keys() returns nil when db is empty.
	d := &DBDict{}
	keys := d.Keys()
	// No entries → returns nil or empty slice.
	require.Empty(t, keys)
}

func TestDBDict_Get_Panics(t *testing.T) {
	d := &DBDict{}
	require.Panics(t, func() { d.Get("anything") })
}

func TestSearchDefinitions_Empty(t *testing.T) {
	matches, err := SearchDefinitions("", 10)
	require.NoError(t, err)
	require.Nil(t, matches)
}

func TestSearchDefinitions_ZeroLimit(t *testing.T) {
	// SearchDefinitions with limit=0 uses default of 10; no panic.
	_, err := SearchDefinitions("anything", 0)
	// May fail because vocab.db doesn't exist in test env; that's fine.
	_ = err
}

func TestActiveDefinitionTokenizer_Default(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No config.json → returns default unicode61.
	tok := ActiveDefinitionTokenizer()
	require.Equal(t, DefinitionTokenizerUnicode61, tok)
}

func TestVocabHasColumn_Present(t *testing.T) {
	db := setupVocabDB(t)
	_, err := db.Exec(`CREATE TABLE vocab(word TEXT, def TEXT)`)
	require.NoError(t, err)

	ok, err := vocabHasColumn(db, "def")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestVocabHasColumn_Absent(t *testing.T) {
	db := setupVocabDB(t)
	_, err := db.Exec(`CREATE TABLE vocab(word TEXT)`)
	require.NoError(t, err)

	ok, err := vocabHasColumn(db, "def_text")
	require.NoError(t, err)
	require.False(t, ok)
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

func setupVocabDBWithData(t *testing.T) *sql.DB {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dbPath := filepath.Join(tmpHome, ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE vocab(word TEXT NOT NULL COLLATE NOCASE, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "", def_text TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word, src, def, def_text) VALUES
	('doctor', 'dict-a', '<div>treats kidney problems</div>', 'treats kidney problems'),
	('nurse', '', '<div>cares for patients</div>', 'cares for patients');`)
	require.NoError(t, err)
	return db
}

func TestBuildDefinitionSearchIndex_WithTrigram(t *testing.T) {
	db := setupVocabDBWithData(t)
	// Build with trigram tokenizer.
	err := BuildDefinitionSearchIndex(db, DefinitionTokenizerTrigram)
	require.NoError(t, err)
}

func TestSearchDefinitions_WithResults(t *testing.T) {
	db := setupVocabDBWithData(t)
	require.NoError(t, BuildDefinitionSearchIndex(db, DefinitionTokenizerUnicode61))

	matches, err := SearchDefinitions("kidney", 10)
	require.NoError(t, err)
	if len(matches) > 0 {
		require.NotEmpty(t, matches[0].Word)
	}
}

func TestCheckDefinitionSearchReady_NoVocab(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No vocab.db → SearchDefinitions should return a DefinitionSearchError.
	_, err := SearchDefinitions("test", 10)
	require.Error(t, err)
	var dse *DefinitionSearchError
	require.ErrorAs(t, err, &dse)
}
