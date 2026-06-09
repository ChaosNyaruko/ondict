package sources

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/util"
)

func TestRankCompletionsPrefixDedupsAndSorts(t *testing.T) {
	words := []string{"application", "Apple", "apple", "app", "append"}

	got := rankCompletions(words, "app", CompletionPrefix, 10)

	assert.Equal(t, []string{"app", "Apple", "append", "application"}, got)
}

func TestRankCompletionsFuzzy(t *testing.T) {
	words := []string{"dictionary", "doctor", "dark", "dock", "deal"}

	got := rankCompletions(words, "dct", CompletionFuzzy, 3)

	require.GreaterOrEqual(t, len(got), 2)
	assert.Equal(t, []string{"doctor", "dictionary"}, got[:2])
}

func TestParseCompletionMode(t *testing.T) {
	assert.Equal(t, CompletionFuzzy, ParseCompletionMode("fzf"))
	assert.Equal(t, CompletionFuzzy, ParseCompletionMode("fuzzy"))
	assert.Equal(t, CompletionFuzzy, ParseCompletionMode("FZF"))
	assert.Equal(t, CompletionFuzzy, ParseCompletionMode("FUZZY"))
	assert.Equal(t, CompletionPrefix, ParseCompletionMode("prefix"))
	assert.Equal(t, CompletionPrefix, ParseCompletionMode(""))
	assert.Equal(t, CompletionPrefix, ParseCompletionMode("unknown"))
}

func TestRankCompletions_EmptyQuery(t *testing.T) {
	words := []string{"apple", "banana"}
	got := rankCompletions(words, "   ", CompletionPrefix, 10)
	assert.Nil(t, got)
}

func TestRankCompletions_LimitRespected(t *testing.T) {
	words := []string{"apple", "application", "apply", "appoint", "approach", "appreciate"}
	got := rankCompletions(words, "app", CompletionPrefix, 3)
	assert.Len(t, got, 3)
}

func TestReadConfig_NotExist(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	util.SetPaths("", "")
	defer util.SetPaths("", "")

	// config.json doesn't exist → DefaultConfig + ErrNotExist returned.
	cfg, err := ReadConfig()
	assert.Error(t, err)
	assert.Equal(t, "unicode61", cfg.Search.DefinitionIndex.Tokenizer)
}

func TestReadConfig_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	data, _ := json.Marshal(Config{
		Dicts: []DictConfig{{Name: "MyDict", Type: "Online"}},
		Search: SearchConfig{DefinitionIndex: DefinitionIndexConfig{Tokenizer: "ascii"}},
	})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644))

	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.Len(t, cfg.Dicts, 1)
	assert.Equal(t, "MyDict", cfg.Dicts[0].Name)
	assert.Equal(t, "ascii", cfg.Search.DefinitionIndex.Tokenizer)
}

func TestReadConfig_EmptyTokenizer_DefaultsToUnicode61(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	// Write config with empty tokenizer to trigger normalizeConfig.
	data := []byte(`{"dicts":[],"search":{"definition_index":{"tokenizer":""}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), data, 0o644))

	cfg, err := ReadConfig()
	require.NoError(t, err)
	assert.Equal(t, "unicode61", cfg.Search.DefinitionIndex.Tokenizer)
}

func TestReadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`not json`), 0o644))

	_, err := ReadConfig()
	assert.Error(t, err)
}

func TestLoadConfig_Empty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	util.SetPaths("", "")
	defer util.SetPaths("", "")

	origG := G
	newG := Dicts{}
	G = &newG
	defer func() { G = origG }()

	// No config.json → LoadConfig ignores ErrNotExist and returns nil.
	assert.NoError(t, LoadConfig())
	assert.Empty(t, *G)
}

func TestLoadConfig_WithDicts(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	origG := G
	newG := Dicts{}
	G = &newG
	defer func() { G = origG }()

	cfg := `{"dicts":[{"name":"TestDict","type":"Online"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o644))

	assert.NoError(t, LoadConfig())
	assert.Len(t, *G, 1)
	assert.Contains(t, (*G)[0].MdxFile, "TestDict")
}

func TestLoadConfig_DisabledDict(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")

	origG := G
	newG := Dicts{}
	G = &newG
	defer func() { G = origG }()

	cfg := `{"dicts":[{"name":"TestDict","type":"Online","enabled":false}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o644))

	assert.NoError(t, LoadConfig())
	assert.Empty(t, *G, "disabled dict should not be added to G")
}

// setupG replaces the global G with a single MapDict entry and returns a
// cleanup function that restores the original value.
func setupG(t *testing.T, words map[string]string) func() {
	t.Helper()
	orig := G
	d := &MdxDict{
		MdxFile: "test.mdx",
		MdxDict: Map(words),
	}
	newG := Dicts{d}
	G = &newG
	return func() { G = orig }
}

func TestComplete_EmptyQuery(t *testing.T) {
	defer setupG(t, map[string]string{"apple": "def"})()
	result := Complete("", CompletionPrefix, 10)
	assert.Nil(t, result)
}

func TestComplete_PrefixMode(t *testing.T) {
	defer setupG(t, map[string]string{
		"apple": "a", "application": "b", "banana": "c",
	})()
	result := Complete("app", CompletionPrefix, 10)
	require.NotEmpty(t, result)
	for _, w := range result {
		assert.True(t, strings.HasPrefix(strings.ToLower(w), "app"),
			"expected prefix 'app', got %q", w)
	}
}

func TestComplete_FuzzyMode(t *testing.T) {
	defer setupG(t, map[string]string{
		"apple": "a", "application": "b", "banana": "c",
	})()
	result := Complete("apl", CompletionFuzzy, 5)
	// At minimum "apple" or "application" should match.
	assert.NotEmpty(t, result)
}

func TestComplete_DefaultLimit(t *testing.T) {
	words := make(map[string]string)
	for i := 0; i < 20; i++ {
		w := fmt.Sprintf("apple%d", i)
		words[w] = "def"
	}
	defer setupG(t, words)()
	result := Complete("apple", CompletionPrefix, 0) // 0 ⇒ default 10
	assert.LessOrEqual(t, len(result), 10)
}

func TestAllWords(t *testing.T) {
	defer setupG(t, map[string]string{"dog": "d", "cat": "c"})()
	words := allWords()
	assert.ElementsMatch(t, []string{"dog", "cat"}, words)
}

func TestDBDict_Complete_Prefix(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")
	// setup creates a minimal vocab.db under dir
	setupVocabDB(t)

	d := &DBDict{}
	words := d.Complete("a", CompletionPrefix, 5)
	assert.LessOrEqual(t, len(words), 5)
}

func TestDBDict_Complete_Fuzzy(t *testing.T) {
	dir := t.TempDir()
	util.SetPaths(dir, "")
	defer util.SetPaths("", "")
	setupVocabDB(t)

	d := &DBDict{}
	words := d.Complete("apl", CompletionFuzzy, 5)
	assert.LessOrEqual(t, len(words), 5)
}

func TestComplete_ZeroLimit(t *testing.T) {
	// limit=0 → defaults to 10, but no words → nil/empty result.
	result := Complete("test", CompletionFuzzy, 0)
	_ = result
}

func TestComplete_WithVocabDB(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "ondict", "vocab.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))

	db, err := sql.Open("sqlite3", "file:"+dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE vocab(word TEXT NOT NULL, src TEXT NOT NULL DEFAULT "", def TEXT NOT NULL DEFAULT "");
INSERT INTO vocab(word) VALUES ('apple'), ('apply'), ('application');`)
	require.NoError(t, err)

	// Set G to a single vocab.db dict.
	orig := G
	t.Cleanup(func() { G = orig })
	dict := &DBDict{}
	newDicts := Dicts{&MdxDict{MdxFile: "vocab.db", MdxDict: dict}}
	G = &newDicts

	result := Complete("app", CompletionPrefix, 5)
	require.NotEmpty(t, result)
}
