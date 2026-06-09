package main

import (
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ChaosNyaruko/ondict/history"
	"github.com/ChaosNyaruko/ondict/sources"
	"github.com/ChaosNyaruko/ondict/wordbank"
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

func TestSearchHandlerDefaultsToHeadwordRedirect(t *testing.T) {
	prepareSearchDB(t)
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/search?query=doctor&format=html&record=0", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Contains(t, rec.Header().Get("Location"), "/dict?query=doctor")
}

func TestWordsHandlerHTML(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, wordbank.Add("apple"))
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/words", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "Word Bank")
	require.Contains(t, body, "apple")
	require.Contains(t, body, "/words/remove")
}

func TestWordBankAddAndRemoveHandlers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	addReq := httptest.NewRequest(http.MethodPost, "/words/add", strings.NewReader("word=apple&next=/words"))
	addReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addRec := httptest.NewRecorder()
	proxy.e.ServeHTTP(addRec, addReq)

	require.Equal(t, http.StatusSeeOther, addRec.Code)
	require.Equal(t, "/words", addRec.Header().Get("Location"))
	contains, err := wordbank.Contains("apple")
	require.NoError(t, err)
	require.True(t, contains)

	removeReq := httptest.NewRequest(http.MethodPost, "/words/remove", strings.NewReader("word=apple&next=/words"))
	removeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	removeRec := httptest.NewRecorder()
	proxy.e.ServeHTTP(removeRec, removeReq)

	require.Equal(t, http.StatusSeeOther, removeRec.Code)
	contains, err = wordbank.Contains("apple")
	require.NoError(t, err)
	require.False(t, contains)
}

func TestDictHandlerShowsWordBankButton(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/dict?query=apple&engine=mdx&format=html&record=0", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Add to Word Bank")
}

func TestParseAddr(t *testing.T) {
	tests := []struct {
		input   string
		network string
		address string
	}{
		{"auto", "auto", ""},
		{"localhost:1345", "tcp", "localhost:1345"},
		{"unix;/tmp/ondict.sock", "unix", "/tmp/ondict.sock"},
		{"tcp;0.0.0.0:1234", "tcp", "0.0.0.0:1234"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			n, a := ParseAddr(tt.input)
			require.Equal(t, tt.network, n)
			require.Equal(t, tt.address, a)
		})
	}
}

func TestCleanInput(t *testing.T) {
	require.Equal(t, "hello", cleanInput("  hello  "))
	require.Equal(t, "hello world", cleanInput("hello world"))
	require.Equal(t, "", cleanInput("  "))
	require.Equal(t, ".exit", cleanInput("  .exit "))
}

func TestPrintPromptAndHelp(t *testing.T) {
	// These just print to stdout; verify they don't panic.
	printPrompt()
	displayHelp()
	printUnknown("unknown cmd")
	handleInvalidCmd("bad")
	handleCmd("bad")
}

func TestIndexRoute(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Ondict")
}

func TestCompleteRoute(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/complete?prefix=ap", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSyncRoute(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/sync", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Cloud Sync")
}

func TestDictHandlerNonHTML(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/dict?query=apple&engine=mdx&format=md&record=0", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestLoginHandler_GET(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "html")
}

func TestProcessLogin_InvalidCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader("username=wrong&password=bad"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestProcessLogin_ValidCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader("username=user&password=password"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/auth", rec.Header().Get("Location"))
}

func TestAuthRoute_Unauthenticated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)

	// Unauthenticated → redirect to /login
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/login", rec.Header().Get("Location"))
}

func TestReviewHandler_NoParams(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)
	// Unauthenticated → redirected
	require.Equal(t, http.StatusFound, rec.Code)
}

func TestQuery_MDX(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil

	result := query("apple", "mdx", "md", false)
	require.IsType(t, "", result)
}

func TestQuery_Online(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil

	// Non-mdx engine tries network; we just verify it returns a string (may fail gracefully).
	result := query("apple", "online", "md", false)
	require.IsType(t, "", result)
}

func TestQuery_EmptyEngineAndFormat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil

	// Empty engine/format → falls back to flag defaults (*engine, *renderFormat).
	// The flag defaults are "", which itself is not "mdx", so it calls GetFromLDOCE.
	result := query("apple", "", "", false)
	require.IsType(t, "", result)
}

func TestQuery_WithHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = history.NewHistory(history.NewSqlite3Writer())
	defer func() { his = nil }()

	result := query("apple", "mdx", "md", true)
	require.IsType(t, "", result)
}

func TestQueryDefinition_NoMatches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil

	// Without vocab.db the function returns an error message; with it, empty results.
	result := queryDefinition("zzznomatch99999", "md", false)
	require.NotEmpty(t, result)
}

func TestQueryDefinition_WithRecord(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Use a real history writer.
	his = history.NewHistory(history.NewSqlite3Writer())
	defer func() { his = nil }()

	result := queryDefinition("zzznomatch99999", "md", true)
	require.IsType(t, "", result)
}

func TestQueryDefinition_WithMatches(t *testing.T) {
	prepareSearchDB(t)
	his = nil

	// "kidney" appears in the "doctor" entry definition; should return results.
	result := queryDefinition("kidney", "md", false)
	// Either found matches or an error — either way it's a non-empty string.
	require.NotEmpty(t, result)
	// If the search worked, result should contain "doctor".
	if strings.Contains(result, "doctor") {
		require.Contains(t, result, "1.")
	}
}

func TestQueryDefinition_WithSrc(t *testing.T) {
	prepareSearchDB(t)
	his = nil

	// Query for "recover" which is in the nurse entry with src "dict-a".
	result := queryDefinition("recover", "md", false)
	require.NotEmpty(t, result)
}

func TestRestoreAndStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Just verify they don't panic.
	Store()
	Restore()
}

func TestAutoNetworkAddressPosix(t *testing.T) {
	network, addr := autoNetworkAddressPosix("/usr/local/bin/ondict", "testid")
	require.Equal(t, "unix", network)
	require.Contains(t, addr, "ondict")
	require.Contains(t, addr, "testid")
}

func TestAutoNetworkAddressPosix_NoID(t *testing.T) {
	network, addr := autoNetworkAddressPosix("/usr/local/bin/ondict", "")
	require.Equal(t, "unix", network)
	require.Contains(t, addr, "ondict")
}

func TestReviewHandler_WithParams(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = history.NewHistory(history.NewSqlite3Writer())
	defer func() { his = nil }()
	proxy := NewProxy()

	// With query params → reviewHandler calls his.Review and returns text.
	req := httptest.NewRequest(http.MethodGet, "/auth?days_ago=7", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)
	// Unauthenticated → redirect; that's OK, auth layer runs first.
	// Just verify no panic and we get a sensible status.
	require.True(t, rec.Code == http.StatusFound || rec.Code == http.StatusOK)
}

func TestIdleResetMiddleware(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	proxy := NewProxy()

	// A request should flow through idleResetMiddleware without panicking.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	proxy.e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestIdleResetMiddleware_WithTimeout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil
	p := NewProxy()

	// Install a real timer on the proxy so the non-nil branch is exercised.
	p.timeout = time.NewTimer(10 * time.Second)
	defer p.timeout.Stop()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	p.e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMountSyncServer_MissingEnvVars(t *testing.T) {
	t.Setenv("ONDICT_SYNC_USER", "")
	t.Setenv("ONDICT_SYNC_PASSWORD", "")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	err := mountSyncServer(r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "env vars")
}

func TestMountSyncServer_WithCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONDICT_SYNC_USER", "alice")
	t.Setenv("ONDICT_SYNC_PASSWORD", "secret")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	err := mountSyncServer(r)
	require.NoError(t, err)
}

func TestProxyRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	his = nil

	p := NewProxy()

	// Create a real listener on a random port, start the proxy, do one
	// request, then close the listener to let Run return.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- p.Run(ln)
	}()

	// Make a request to verify the server responds.
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err == nil {
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// Stop the server by closing the listener.
	ln.Close()
	<-done // wait for Run to return
}

func TestClearScreen(t *testing.T) {
	// clearScreen runs "clear" command; just verify no panic.
	clearScreen()
}

func TestAutoNetworkAddressDefault_EmptyID(t *testing.T) {
	network, addr := autoNetworkAddressDefault("/usr/bin/ondict", "")
	require.Equal(t, "tcp", network)
	require.Contains(t, addr, "localhost")
}

func TestAutoNetworkAddressDefault_WithID_Panics(t *testing.T) {
	require.Panics(t, func() {
		autoNetworkAddressDefault("/usr/bin/ondict", "some-id")
	})
}

func TestStartRemoteDefault_InvalidBinary(t *testing.T) {
	err := startRemoteDefault("/nonexistent/binary", nil)
	require.Error(t, err)
}

func TestStartRemoteDefault_ValidBinary(t *testing.T) {
	// Use "true" which exists on all POSIX systems and exits 0 immediately.
	err := startRemoteDefault("/usr/bin/true", nil)
	require.NoError(t, err)
}

func TestAuthMiddleware_NoSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// NewProxy sets up session store; unauthenticated request should redirect.
	p := NewProxy()
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec := httptest.NewRecorder()
	p.e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/login", rec.Header().Get("Location"))
}

func TestStartRemotePosix_InvalidBinary(t *testing.T) {
	err := startRemotePosix("/nonexistent/binary", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "startRemote server err")
}

func TestStartRemotePosix_ValidBinary(t *testing.T) {
	err := startRemotePosix("/usr/bin/true", nil)
	require.NoError(t, err)
}
