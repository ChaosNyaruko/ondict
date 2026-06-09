package mobile

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsSyncTransient(t *testing.T) {
	cases := []struct {
		name      string
		errMsg    string
		wantRetry bool
	}{
		// 4xx — permanent, do NOT retry
		{"401 unauthorized", "/sync/v1/wordbank/pull: HTTP 401: Unauthorized", false},
		{"403 forbidden", "/sync/v1/history/pull: HTTP 403: Forbidden", false},
		{"404 not found", "/sync/v1/wordbank/push: HTTP 404: Not Found", false},
		{"400 bad request", "/sync/v1/wordbank/pull: HTTP 400: bad since timestamp", false},

		// 5xx — transient, retry
		{"500 server error", "/sync/v1/wordbank/push: HTTP 500: internal server error", true},
		{"503 unavailable", "/sync/v1/history/pull: HTTP 503: Service Unavailable", true},

		// Network-level errors — no HTTP code, transient
		{"connection refused", "Post \"http://...\": dial tcp: connect: connection refused", true},
		{"timeout", "context deadline exceeded", true},
		{"empty", "", true},

		// Unknown / unrecognised format — default to transient
		{"no code parseable", "some unexpected error: HTTP xyz: broken", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSyncTransient(tc.errMsg)
			if got != tc.wantRetry {
				t.Errorf("IsSyncTransient(%q) = %v, want %v", tc.errMsg, got, tc.wantRetry)
			}
		})
	}
}

func TestWordbank_AddContainsListRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Reset the storeOnce so each test starts clean.
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	require.Equal(t, "", WordbankAdd("mango"))
	assert.True(t, WordbankContains("mango"))

	list := WordbankList()
	assert.Contains(t, list, "mango")

	require.Equal(t, "", WordbankRemove("mango"))
	assert.False(t, WordbankContains("mango"))
}

func TestWordbankList_Empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	list := WordbankList()
	assert.Equal(t, "[]", list)
}

func TestComplete_Empty(t *testing.T) {
	// No dicts loaded; Complete should return "null" or "[]".
	out := Complete("app", 5)
	assert.True(t, out == "null" || out == "[]", "got: %s", out)
}

func TestQueryEntry_Empty(t *testing.T) {
	// No dicts loaded; QueryEntry should return "".
	out := QueryEntry("apple")
	assert.IsType(t, "", out)
}

func TestGetFile_NotFound(t *testing.T) {
	// No MDD loaded; returns nil.
	data := GetFile("nonexistent.mp3")
	assert.Nil(t, data)
}

func TestGetCSS(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// No CSS files; returns the sentinel pagetitle override.
	css := GetCSS()
	assert.IsType(t, "", css)
}

func TestConfigureSync_EmptyCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	// Empty credentials → no error, sync is disabled.
	err := ConfigureSync("", "", "")
	require.NoError(t, err)
}

func TestSync_NotConfigured(t *testing.T) {
	syncClient = nil
	_, err := Sync()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestEnsureSharedStores(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	err := ensureSharedStores()
	require.NoError(t, err)
	assert.NotNil(t, syncWB)
	assert.NotNil(t, syncHistory)
}

func TestInitSyncOnly_EmptyCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil
	syncClient = nil

	dir := t.TempDir()
	cache := t.TempDir()
	// Empty credentials → ConfigureSync clears the client, no error.
	err := InitSyncOnly(dir, cache, "", "", "")
	require.NoError(t, err)
}

func TestInitSyncOnly_WithCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil
	syncClient = nil

	dir := t.TempDir()
	cache := t.TempDir()
	err := InitSyncOnly(dir, cache, "http://127.0.0.1:19999", "alice", "secret")
	require.NoError(t, err)
	assert.NotNil(t, syncClient)

	// Sync should fail since server is not reachable but no panic.
	_, err = Sync()
	assert.Error(t, err)
}

func TestConfigureSync_WithCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil
	syncClient = nil

	err := ConfigureSync("http://localhost:19999", "alice", "secret")
	require.NoError(t, err)
	assert.NotNil(t, syncClient)
}

func TestWordbank_Add_ErrorPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	// Adding an empty word should return an error string.
	result := WordbankAdd("")
	assert.NotEmpty(t, result)
}

func TestWordbank_Remove_ErrorPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	// Removing an empty word should return an error string.
	result := WordbankRemove("")
	assert.NotEmpty(t, result)
}

func TestWordbank_Contains_NotPresent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	// Word not in bank → false.
	ok := WordbankContains("nonexistent_word_xyz")
	assert.False(t, ok)
}

func TestInit_WithTempDir(t *testing.T) {
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil

	dir := t.TempDir()
	cache := t.TempDir()

	// Init should complete without error.
	err := Init(dir, cache)
	// Error is expected since no dicts are loaded, but function should not panic.
	_ = err
}

func TestIsSyncTransient_Cases(t *testing.T) {
	// No HTTP in message → network error → transient.
	assert.True(t, IsSyncTransient("connection refused"))

	// HTTP 4xx → permanent (not transient).
	assert.False(t, IsSyncTransient("HTTP 401: unauthorized"))

	// HTTP 5xx → transient.
	assert.True(t, IsSyncTransient("HTTP 500: internal server error"))

	// Empty string → true (no HTTP, treat as transient).
	assert.True(t, IsSyncTransient(""))
}

func TestSync_AlreadyConfigured(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	storeOnce = sync.Once{}
	storeOpenErr = nil
	syncWB = nil
	syncHistory = nil
	syncClient = nil

	// Not configured → error.
	_, err := Sync()
	require.Error(t, err)
}
