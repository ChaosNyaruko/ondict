package sources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ChaosNyaruko/ondict/util"
)

type MockResult struct {
	match string
	def   string
}

func (m MockResult) GetMatch() string      { return m.match }
func (m MockResult) GetDefinition() string { return m.def }
func (m MockResult) GetSrc() string        { return "" }

type MockSearcher struct {
	results []MockResult
}

func (ms *MockSearcher) GetRawOutputs(word string) []RawOutput {
	var ret []RawOutput
	for _, r := range ms.results {
		// Simple mock: return all results if word is "all", else exact match
		if word == "all" || r.match == word {
			ret = append(ret, r)
		}
	}
	return ret
}

func TestQueryMDX(t *testing.T) {
	// Setup G
	originalG := G
	defer func() { G = originalG }()

	mockSearcher := &MockSearcher{
		results: []MockResult{
			{match: "test", def: "definition of test"},
		},
	}

	dict := &MdxDict{
		Type:     "TestType",
		MdxFile:  "test.mdx",
		searcher: mockSearcher,
	}

	d := Dicts{dict}
	G = &d

	// Test html format
	res := QueryMDX("test", "html")
	assert.Contains(t, res, "definition of test")
	assert.Contains(t, res, "div")

	// Test text format
	res = QueryMDX("test", "text")
	assert.Contains(t, res, "definition of test")
	assert.Contains(t, res, "----")
}

func TestAllCss_ReturnsString(t *testing.T) {
	// allCss starts as ""; AllCss() should not panic.
	result := AllCss()
	assert.IsType(t, "", result)
}

func TestInitAllCss_NoDictsPath(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	// No dicts directory → initAllCss silently returns.
	assert.NotPanics(t, func() { initAllCss() })
}

func TestInitAllCss_WithCSS(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	// Create a dicts sub-dir and a CSS file.
	dictsDir := util.DictsPath()
	assert.NoError(t, os.MkdirAll(dictsDir, 0o755))
	assert.NoError(t, os.WriteFile(
		filepath.Join(dictsDir, "my.css"),
		[]byte("body { color: red }"),
		0o644,
	))

	initAllCss()
	assert.Contains(t, allCss, "body { color: red }")
	// Reset to avoid leaking state.
	allCss = ""
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	assert.NoError(t, os.WriteFile(src, []byte("hello"), 0o644))
	assert.NoError(t, copyFile(src, dst))

	got, err := os.ReadFile(dst)
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), got)
}

func TestCopyFile_SrcMissing(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "noexist.txt"), filepath.Join(dir, "dst.txt"))
	assert.Error(t, err)
}

func TestGetMDDFile_Empty(t *testing.T) {
	// mddFiles is empty; GetMDDFile should return nil.
	result := GetMDDFile("any.mp3")
	assert.Nil(t, result)
}

func TestLoadDecodedMdx_JSON(t *testing.T) {
	dir := t.TempDir()
	// Create a JSON file that loadDecodedMdx reads.
	jsonPath := filepath.Join(dir, "test.json")
	assert.NoError(t, os.WriteFile(jsonPath, []byte(`{"apple":"red fruit","banana":"yellow fruit"}`), 0o644))

	// loadDecodedMdx looks for filePath+".json" where filePath = dir/test.
	result := loadDecodedMdx(filepath.Join(dir, "test"), false, false, false)
	assert.NotNil(t, result)

	// Verify the returned Dict has the expected keys.
	keys := result.Keys()
	assert.Contains(t, keys, "apple")
	assert.Contains(t, keys, "banana")
	assert.Equal(t, "red fruit", result.Get("apple"))
}

func TestMdxDict_Get(t *testing.T) {
	mockSearcher := &MockSearcher{
		results: []MockResult{
			{match: "short", def: "def1"},
			{match: "longer", def: "def2"},
			{match: "longest", def: "def3"},
		},
	}

	dict := &MdxDict{
		Type:     "TestType",
		MdxFile:  "test.mdx",
		searcher: mockSearcher,
	}

	// MockSearcher returns all for "all"
	// Get logic picks longest match
	defs := dict.Get("all")
	assert.Len(t, defs, 1)
	assert.Equal(t, "def3", defs[0])
}

func TestRegisterDictDB(t *testing.T) {
	// Save and restore G.
	orig := G
	d := Dicts{}
	G = &d
	defer func() { G = orig }()

	dict := &MdxDict{
		Type:    "TestType",
		MdxFile: "test",
	}
	err := dict.registerDictDB()
	assert.NoError(t, err)
	assert.NotNil(t, dict.MdxDict)
	assert.Len(t, *G, 1)
}

func TestMdxDict_Get_Empty(t *testing.T) {
	mockSearcher := &MockSearcher{results: nil}
	dict := &MdxDict{
		Type:     "TestType",
		MdxFile:  "test.mdx",
		searcher: mockSearcher,
	}
	// No results → empty slice.
	defs := dict.Get("nonexistent")
	assert.Empty(t, defs)
}

func TestMdxDict_Get_SingleResult(t *testing.T) {
	mockSearcher := &MockSearcher{
		results: []MockResult{
			{match: "apple", def: "a fruit"},
		},
	}
	dict := &MdxDict{
		Type:     "TestType",
		MdxFile:  "test.mdx",
		searcher: mockSearcher,
	}
	defs := dict.Get("apple")
	assert.Len(t, defs, 1)
}
