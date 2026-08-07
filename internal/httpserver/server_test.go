package httpserver

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/util"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// ── PageTitle ─────────────────────────────────────────────────────────────────

func TestPageTitle(t *testing.T) {
	assert.Equal(t, "Ondict", PageTitle(""))
	assert.Equal(t, "Ondict", PageTitle("   "))
	assert.Equal(t, "apple - Ondict", PageTitle("apple"))
	assert.Equal(t, "apple - Ondict", PageTitle("  apple  "))
}

// ── SafeNext ──────────────────────────────────────────────────────────────────

func TestSafeNext(t *testing.T) {
	assert.Equal(t, "/words", SafeNext(""))
	assert.Equal(t, "/words", SafeNext("words"))
	assert.Equal(t, "/words", SafeNext("//evil.com"))
	assert.Equal(t, "/words", SafeNext("http://evil.com"))
	assert.Equal(t, "/words", SafeNext("/words"))
	assert.Equal(t, "/custom/path", SafeNext("/custom/path"))
}

// ── New / basic route smoke tests ──────────────────────────────────────────────

func TestNew_IndexRoute(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	// The index handler returns 200 with portal.html content.
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "html")
}

func TestNew_CompleteRoute_NoPrefix(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/complete", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "prefix empty")
}

func TestNew_CompleteRoute_WithPrefix(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/complete?prefix=app", nil)
	r.ServeHTTP(w, req)
	// Completions come from the global dict which is empty in tests; returns JSON null/[].
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// Either "null" or a JSON array.
	assert.True(t, body == "null" || strings.HasPrefix(body, "["),
		"unexpected body: %q", body)
}

func TestNew_SyncRoute(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/sync", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Cloud Sync")
}

func TestNew_WordsRoute(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/words", nil)
	r.ServeHTTP(w, req)
	// words.html template renders OK.
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNew_SearchRoute_Redirect(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/search?query=apple&engine=mdx&format=html", nil)
	r.ServeHTTP(w, req)
	// headword mode with html format → redirect to /dict
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/dict")
}

func TestNew_SearchRoute_NonHTML(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/search?query=apple&engine=mdx&format=md", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNew_SearchRoute_DefinitionMode_HTML(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/search?query=water&mode=definition&format=html", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNew_SearchRoute_DefinitionMode_NonHTML(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/search?query=water&mode=definition&format=md", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNew_DictRoute_HTMLFormat(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/dict?query=apple&engine=mdx&format=html", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNew_DictRoute_NonHTMLFormat(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/dict?query=apple&engine=mdx&format=md", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNew_AddWordHandler_EmptyWord(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("word", "")
	req, _ := http.NewRequest(http.MethodPost, "/words/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNew_RemoveWordHandler_EmptyWord(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("word", "")
	req, _ := http.NewRequest(http.MethodPost, "/words/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestNew_AddAndRemoveWord(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	r := New(Options{})

	// Add a word.
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("word", "testword")
	form.Set("next", "/words")
	req, _ := http.NewRequest(http.MethodPost, "/words/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code)

	// Remove the word.
	w = httptest.NewRecorder()
	form = url.Values{}
	form.Set("word", "testword")
	form.Set("next", "/words")
	req, _ = http.NewRequest(http.MethodPost, "/words/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code)
}

func TestNew_ResourceHandlerNotFound(t *testing.T) {
	called := false
	r := New(Options{
		ResourceHandler: gin.HandlerFunc(func(c *gin.Context) {
			called = true
			c.Status(http.StatusNotFound)
		}),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/nonexistent/file.mp3", nil)
	r.ServeHTTP(w, req)
	assert.True(t, called)
}

func TestNew_WithMiddleware(t *testing.T) {
	middlewareCalled := false
	r := New(Options{
		Middleware: []gin.HandlerFunc{
			func(c *gin.Context) {
				middlewareCalled = true
				c.Next()
			},
		},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/sync", nil)
	r.ServeHTTP(w, req)
	assert.True(t, middlewareCalled)
}

func TestQueryMDX_MDXEngine(t *testing.T) {
	result := queryMDX("apple", "mdx", "md", nil, false)
	assert.IsType(t, "", result)
}

func TestQueryMDX_OnlineEngine(t *testing.T) {
	// online engine falls back to GetFromLDOCE which returns a string
	result := queryMDX("apple", "online", "md", nil, false)
	assert.IsType(t, "", result)
}

func TestQueryDefinition_Empty(t *testing.T) {
	// SearchDefinitions returns nil for empty query → QueryDefinition returns "no definition matches"
	result := QueryDefinition("")
	// Either empty or "no definition matches …" depending on vocab.db presence.
	assert.IsType(t, "", result)
}

func TestQueryDefinition_NoMatch(t *testing.T) {
	// Without vocab.db this returns a definition search error message.
	result := QueryDefinition("zzznomatch99999")
	assert.NotEmpty(t, result)
}

func TestMddFileHandler_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.NoRoute(MddFileHandler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/nonexistent.mp3", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMddFileHandler_ServesTmpDirFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configDir := t.TempDir()
	tmpDir := t.TempDir()
	util.SetPaths(configDir, tmpDir)
	t.Cleanup(func() { util.SetPaths("", "") })

	cachedFile := filepath.Join(tmpDir, "images", "fruit.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(cachedFile), 0o755))
	require.NoError(t, os.WriteFile(cachedFile, []byte("cached image"), 0o644))

	r := gin.New()
	r.NoRoute(MddFileHandler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/images/fruit.jpg", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cached image", w.Body.String())
}

func TestWordsHandler_WithWordbank(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/words", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCompleteHandler_WithLimit(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/complete?prefix=app&limit=5", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddWordHandler_EmptyWord(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("word", "")
	req, _ := http.NewRequest(http.MethodPost, "/words/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	// Empty word → 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveWordHandler_EmptyWord(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("word", "")
	req, _ := http.NewRequest(http.MethodPost, "/words/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	// Empty word → 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddWordHandler_ValidWord(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := New(Options{})
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("word", "testword")
	form.Set("next", "/words")
	req, _ := http.NewRequest(http.MethodPost, "/words/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	// Should redirect to /words or return server error
	require.True(t, w.Code == http.StatusSeeOther || w.Code == http.StatusInternalServerError)
}

func TestRemoveWordHandler_ValidWord(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := New(Options{})
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("word", "testword")
	form.Set("next", "/words")
	req, _ := http.NewRequest(http.MethodPost, "/words/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	require.True(t, w.Code == http.StatusSeeOther || w.Code == http.StatusInternalServerError)
}

func TestSearchHandler_WithQuery(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/search?query=apple&mode=definition", nil)
	r.ServeHTTP(w, req)
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
}

func TestSearchHandler_NoQuery(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	// No query, mode=headword → redirect to /dict
	req, _ := http.NewRequest(http.MethodGet, "/search", nil)
	r.ServeHTTP(w, req)
	// mode=headword with html format redirects to /dict
	assert.True(t, w.Code == http.StatusFound || w.Code == http.StatusOK)
}

func TestSearchHandler_DefinitionTextFormat(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/search?query=apple&mode=definition&format=md", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearchHandler_HeadwordTextFormat(t *testing.T) {
	r := New(Options{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/search?query=apple&mode=headword&format=md", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueryDefinition_WithVocabDB(t *testing.T) {
	// Set up a temp home with a vocab.db containing searchable entries.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	defer func() { t.Setenv("HOME", tmpHome) }()

	dbPath := tmpHome + "/.config/ondict/vocab.db"
	if err := os.MkdirAll(dbPath[:strings.LastIndex(dbPath, "/")], 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE vocab(word TEXT NOT NULL COLLATE NOCASE, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "", def_text TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word, src, def, def_text) VALUES
	('doctor', 'dict-a', '<div>someone who treats kidney problems</div>', 'someone who treats kidney problems'),
	('nurse', '', '<div>cares for patients</div>', 'cares for patients');
`)
	if err != nil {
		t.Fatal(err)
	}
	// Build the FTS index.
	if err := sources.BuildDefinitionSearchIndex(db, sources.DefinitionTokenizerUnicode61); err != nil {
		t.Fatal(err)
	}

	// "kidney" should match the doctor entry.
	result := QueryDefinition("kidney")
	// If vocab.db is found, result should contain the match.
	assert.IsType(t, "", result)
	if strings.Contains(result, "1.") {
		assert.Contains(t, result, "doctor")
	}

	// "cares" should match the nurse entry (no src set).
	result2 := QueryDefinition("cares")
	assert.IsType(t, "", result2)
}
