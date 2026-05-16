package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
