package wordbank

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWordBankAddListContainsAndRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, Add(" apple "))
	require.NoError(t, Add("banana"))

	contains, err := Contains("apple")
	require.NoError(t, err)
	require.True(t, contains)

	words, err := List()
	require.NoError(t, err)
	require.Len(t, words, 2)
	require.ElementsMatch(t, []string{"apple", "banana"}, []string{words[0].Name, words[1].Name})
	require.NotEmpty(t, words[0].CreateTime)
	require.NotEmpty(t, words[0].UpdateTime)

	require.NoError(t, Remove("apple"))
	contains, err = Contains("apple")
	require.NoError(t, err)
	require.False(t, contains)
}

func TestWordBankAddIsCaseInsensitive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, Add("Apple"))
	require.NoError(t, Add("apple"))

	words, err := List()
	require.NoError(t, err)
	require.Len(t, words, 1)
	require.Equal(t, "Apple", words[0].Name)
}

func TestWordBankRejectsEmptyWord(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.True(t, errors.Is(Add(" "), ErrEmptyWord))
	require.True(t, errors.Is(Remove(""), ErrEmptyWord))

	contains, err := Contains("")
	require.NoError(t, err)
	require.False(t, contains)
}
