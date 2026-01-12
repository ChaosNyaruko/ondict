package util

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
