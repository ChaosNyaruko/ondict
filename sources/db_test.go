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
