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
