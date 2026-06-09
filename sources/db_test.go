package sources

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

func TestDBDictWordsWithPrefixDedups(t *testing.T) {
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
CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word, src, def) VALUES
	('apple', 'dict-a', 'a'),
	('apple', 'dict-b', 'b'),
	('Application', 'dict-a', 'c'),
	('app', 'dict-a', 'd');
`)
	require.NoError(t, err)

	got := (&DBDict{}).WordsWithPrefix("app")
	require.Equal(t, []string{"Application", "app", "apple"}, got)
}

func TestDBIExactGetRawOutputsWithDefinitionIndexColumns(t *testing.T) {
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
INSERT INTO vocab(word, src, def, def_text) VALUES
	('hello', 'dict-a', '<div>greeting</div>', 'greeting');
`)
	require.NoError(t, err)

	got := (&DBIExact{}).GetRawOutputs("hello")
	require.Len(t, got, 1)
	require.Equal(t, "hello", got[0].GetMatch())
	require.Contains(t, got[0].GetDefinition(), "greeting")
}

func TestDBDict_Keys_WithData(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word) VALUES ('mango'), ('banana');`)
	require.NoError(t, err)

	keys := (&DBDict{}).Keys()
	require.NotEmpty(t, keys)
	require.Contains(t, keys, "mango")
	require.Contains(t, keys, "banana")
}

func TestDBIExact_GetRawOutputs_NoMatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "");`)
	require.NoError(t, err)

	got := (&DBIExact{}).GetRawOutputs("nonexistent")
	require.Empty(t, got)
}

func TestDBDict_Keys_NoVocabDB(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No vocab.db → Keys should return nil.
	keys := (&DBDict{}).Keys()
	require.Nil(t, keys)
}

func TestDBDict_Complete_PrefixMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word) VALUES ('apple'), ('application'), ('apex');`)
	require.NoError(t, err)

	// Prefix mode, limit >= results.
	got := (&DBDict{}).Complete("ap", CompletionPrefix, 10)
	require.NotEmpty(t, got)

	// Prefix mode, limit < results.
	got2 := (&DBDict{}).Complete("ap", CompletionPrefix, 1)
	require.Len(t, got2, 1)

	// Fuzzy mode.
	got3 := (&DBDict{}).Complete("appl", CompletionFuzzy, 5)
	require.NotEmpty(t, got3)
}

func TestDBDict_Complete_EmptyLimit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word) VALUES ('mango'), ('manta'), ('manual');`)
	require.NoError(t, err)

	// limit=0 should default to 10.
	got := (&DBDict{}).Complete("man", CompletionPrefix, 0)
	require.NotEmpty(t, got)
}
