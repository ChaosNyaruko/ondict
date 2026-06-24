package sources

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/util"
)

func Test_QueryByURL(t *testing.T) {
	get := QueryByURL("doctor")
	t.Logf("get doctor from ldoceonline: %q", get)
}

func TestStore_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")
	historyFile = "" // reset singleton

	// Store writes the (empty) history map to disk.
	Store()

	_, err := os.Stat(util.HistoryFile())
	require.NoError(t, err)
}

func TestRestore_FromFile(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")
	historyFile = ""

	// Write some JSON to the history file.
	require.NoError(t, os.WriteFile(util.HistoryFile(),
		[]byte(`{"doctor":"<div>definition</div>"}`), 0o644))

	Restore()
	// The history map should now contain the key.
	assert.NotEmpty(t, history)
	assert.Equal(t, "<div>definition</div>", history["doctor"])

	// Reset history for other tests.
	history = make(map[string]string)
	historyFile = ""
}

func TestRestore_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")
	historyFile = ""

	// No history file → Restore silently returns without error.
	assert.NotPanics(t, func() { Restore() })
	historyFile = ""
}

func TestGetFromLDOCE_CacheHit(t *testing.T) {
	// Pre-seed the cache.
	history["testword"] = "cached-definition"
	defer delete(history, "testword")

	result := GetFromLDOCE("testword")
	assert.Equal(t, "cached-definition", result)
}
