package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ChaosNyaruko/ondict/sources"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/require"
)

func prepareSearchDB(t *testing.T) {
	t.Helper()

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
CREATE TABLE vocab(word TEXT NOT NULL COLLATE NOCASE, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "", def_text TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word, src, def, def_text) VALUES
	('doctor', 'dict-a', '<div>someone who treats kidney problems</div>', 'someone who treats kidney problems'),
	('nurse', 'dict-a', '<div>someone who helps patients recover</div>', 'someone who helps patients recover');
`)
	require.NoError(t, err)
	require.NoError(t, sources.BuildDefinitionSearchIndex(db, sources.DefinitionTokenizerUnicode61))
}

func TestSearchHandlerHTML(t *testing.T) {
	prepareSearchDB(t)
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/search?query=kidney&mode=definition&format=html&record=0", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "Definition matches for")
	require.Contains(t, body, "doctor")
	require.Contains(t, body, "result-card")
}

func TestSearchHandlerHeadwordRedirect(t *testing.T) {
	prepareSearchDB(t)
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/search?query=doctor&mode=headword&format=html&record=0", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Contains(t, rec.Header().Get("Location"), "/dict?query=doctor")
}
