package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaths(t *testing.T) {
	// Setup temp home
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// UserCacheDir on macOS usually defaults to $HOME/Library/Caches if not set,
	// or might fallback.
	// For testing, we can just check if the returned paths are non-empty and contain the expected suffix.

	cp := ConfigPath()
	assert.NotEmpty(t, cp)
	assert.True(t, strings.Contains(cp, "ondict"))

	dp := DictsPath()
	assert.NotEmpty(t, dp)
	assert.True(t, strings.HasSuffix(dp, "dicts"))

	hf := HistoryFile()
	assert.NotEmpty(t, hf)
	assert.True(t, strings.HasSuffix(hf, "history.json"))

	ht := HistoryTable()
	assert.NotEmpty(t, ht)
	assert.True(t, strings.HasSuffix(ht, "history.table"))

	hdb := HistoryDB()
	assert.NotEmpty(t, hdb)
	assert.True(t, strings.HasSuffix(hdb, "history.db"))

	wdb := WordBankDB()
	assert.NotEmpty(t, wdb)
	assert.True(t, strings.HasSuffix(wdb, "wordbank.db"))

	// TmpDir usually uses UserCacheDir
	// On some systems it might fail if HOME is messed up, but with Setenv HOME it should work.
	// However, UserCacheDir behaviour depends on OS.
	// On Darwin: $HOME/Library/Caches
	// On Linux: $XDG_CACHE_HOME or $HOME/.cache

	// We can just verify it doesn't crash.
	tmp := TmpDir()
	assert.NotEmpty(t, tmp)
	assert.True(t, strings.Contains(tmp, "ondict"))

	// Check if directories were created
	_, err := os.Stat(cp)
	assert.NoError(t, err)

	_, err = os.Stat(tmp)
	assert.NoError(t, err)
}

func TestSetPaths_OverridesConfigAndTmp(t *testing.T) {
	// Save and restore overrides so other tests are not affected.
	defer SetPaths("", "")

	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "mycfg")
	tmpDir := filepath.Join(dir, "mytmp")

	SetPaths(cfgDir, tmpDir)

	cp := ConfigPath()
	require.Equal(t, cfgDir, cp)
	_, err := os.Stat(cfgDir)
	require.NoError(t, err)

	tp := TmpDir()
	require.Equal(t, tmpDir, tp)
	_, err = os.Stat(tmpDir)
	require.NoError(t, err)
}

func TestVocabDB_UsesConfigPathByDefault(t *testing.T) {
	defer SetPaths("", "")
	SetPaths("", "")

	t.Setenv("HOME", t.TempDir())
	v := VocabDB()
	require.True(t, strings.HasSuffix(v, "vocab.db"))
	require.Contains(t, v, "ondict")
}

func TestVocabDB_UsesTmpPathWhenSet(t *testing.T) {
	defer SetPaths("", "")

	dir := t.TempDir()
	SetPaths("", filepath.Join(dir, "cache"))

	v := VocabDB()
	require.True(t, strings.HasSuffix(v, "vocab.db"))
	require.Contains(t, v, "cache")
}

func TestConfigPath_DefaultPath(t *testing.T) {
	defer SetPaths("", "")
	SetPaths("", "")
	t.Setenv("HOME", t.TempDir())

	// No override → uses HOME-based default.
	cp := ConfigPath()
	require.NotEmpty(t, cp)
	require.Contains(t, cp, "ondict")
}

func TestTmpDir_DefaultPath(t *testing.T) {
	defer SetPaths("", "")
	SetPaths("", "")
	t.Setenv("HOME", t.TempDir())

	// No override → uses user cache dir.
	td := TmpDir()
	require.NotEmpty(t, td)
	require.Contains(t, td, "ondict")
}

func TestConfigPath_WithOverride(t *testing.T) {
	defer SetPaths("", "")
	dir := t.TempDir()
	SetPaths(dir, "")

	cp := ConfigPath()
	require.Equal(t, dir, cp)
}

func TestTmpDir_WithOverride(t *testing.T) {
	defer SetPaths("", "")
	dir := t.TempDir()
	SetPaths("", dir)

	td := TmpDir()
	require.Equal(t, dir, td)
}
