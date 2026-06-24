package sources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/util"
)

func Test_loadCssFiles(t *testing.T) {
	res, err := loadAllCss()
	assert.Nil(t, err)
	t.Logf("css file concatenation: \n%v", res)
}

func Test_loadAllCss_WithFiles(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	// Create dicts dir with some CSS files.
	dictsDir := filepath.Join(dir, "dicts")
	require.NoError(t, os.MkdirAll(dictsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dictsDir, "vanilla.css"), []byte("body{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dictsDir, "custom.css"), []byte(".foo{color:red}"), 0o644))

	res, err := loadAllCss()
	require.NoError(t, err)
	assert.Contains(t, res, "body{}")
	assert.Contains(t, res, ".foo{color:red}")
	// vanilla CSS must come before custom.
	assert.Less(t, indexOf(res, "body{}"), indexOf(res, ".foo{color:red}"))
	// Sentinel must be appended.
	assert.Contains(t, res, ".pagetitle")
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestExact_GetRawOutputs tests the case-sensitive Exact searcher.
func TestExact_GetRawOutputs(t *testing.T) {
	dict := &mockDict{
		keys: []string{"apple", "Apple"},
		data: map[string]string{
			"apple": "a fruit",
			"Apple": "tech company",
		},
	}
	s := NewExact(dict)
	got := s.GetRawOutputs("apple")
	require.Len(t, got, 1)
	assert.Equal(t, "apple", got[0].GetMatch())
	assert.Equal(t, "a fruit", got[0].GetDefinition())
}

// TestIExact_GetRawOutputs tests the case-insensitive IExact searcher.
func TestIExact_GetRawOutputs(t *testing.T) {
	dict := &mockDict{
		keys: []string{"Apple", "apple"},
		data: map[string]string{
			"Apple": "tech company",
			"apple": "a fruit",
		},
	}
	s := NewIExact(dict)
	got := s.GetRawOutputs("APPLE")
	// Both "Apple" and "apple" match case-insensitively.
	require.GreaterOrEqual(t, len(got), 1)
}

// mockDict is a minimal Dict implementation for tests.
type mockDict struct {
	keys []string
	data map[string]string
}

func (m *mockDict) Keys() []string { return m.keys }

func (m *mockDict) Get(s string) string { return m.data[s] }

// TestGetSrc verifies the output.GetSrc helper.
func TestOutputGetSrc(t *testing.T) {
	o := output{rawWord: "word", src: "dict-a", def: "meaning"}
	assert.Equal(t, "dict-a", o.GetSrc())
	assert.Equal(t, "word", o.GetMatch())
	assert.Equal(t, "meaning", o.GetDefinition())
}

// TestGetHistoryFile exercises the lazy initialisation path.
func TestGetHistoryFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	historyFile = "" // reset singleton
	f := getHistoryFile()
	assert.NotEmpty(t, f)
	assert.Contains(t, f, "history.json")
	historyFile = "" // leave clean
}
