package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockResult struct {
	match string
	def   string
}

func (m MockResult) GetMatch() string      { return m.match }
func (m MockResult) GetDefinition() string { return m.def }

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
