package wordbank

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/store"
)

// recordingStore is a no-op store.WordbankStore that counts which methods
// the package shim forwarded to it. Used by TestSetStore_InjectsAndIsObserved.
type recordingStore struct {
	adds    int
	removes int
	lastAdd string
}

func (r *recordingStore) Close() error { return nil }
func (r *recordingStore) Add(_ context.Context, w string) error {
	r.adds++
	r.lastAdd = w
	return nil
}
func (r *recordingStore) Remove(_ context.Context, _ string) error {
	r.removes++
	return nil
}
func (r *recordingStore) Contains(context.Context, string) (bool, error) { return false, nil }
func (r *recordingStore) List(context.Context) ([]store.WordbankRow, error) {
	return nil, nil
}
func (r *recordingStore) ListSince(context.Context, time.Time) ([]store.WordbankRow, error) {
	return nil, nil
}
func (r *recordingStore) ListChanged(context.Context, time.Time, time.Time) ([]store.WordbankRow, error) {
	return nil, nil
}
func (r *recordingStore) Upsert(context.Context, store.WordbankRow) error      { return nil }
func (r *recordingStore) GCTombstones(context.Context, time.Time) (int, error) { return 0, nil }

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

func TestWordBankRemoveLeavesTombstone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, Add("apple"))
	require.NoError(t, Remove("apple"))

	// List() hides tombstones.
	live, err := List()
	require.NoError(t, err)
	require.Empty(t, live)

	contains, err := Contains("apple")
	require.NoError(t, err)
	require.False(t, contains)

	// ListWithDeleted exposes the tombstone for sync/merge.
	all, err := ListWithDeleted()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "apple", all[0].Name)
	require.NotEmpty(t, all[0].DeletedAt, "expected deleted_at to be set")
}

func TestWordBankReAddClearsTombstone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, Add("apple"))
	require.NoError(t, Remove("apple"))
	require.NoError(t, Add("apple"))

	contains, err := Contains("apple")
	require.NoError(t, err)
	require.True(t, contains)

	all, err := ListWithDeleted()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Empty(t, all[0].DeletedAt, "re-add must clear the tombstone")
}

// Regression for code-review P3: SetStore must actually replace the
// process-singleton store. The previous implementation reset
// singletonPath="" and then storeImpl() observed the mismatch with
// util.WordBankDB() and silently closed and replaced the injected store.
func TestSetStore_InjectsAndIsObserved(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Reset any leftover singleton from previous tests in this binary.
	SetStore(nil)

	fake := &recordingStore{}
	SetStore(fake)
	t.Cleanup(func() { SetStore(nil) })

	require.NoError(t, Add("hello"))
	require.Equal(t, 1, fake.adds, "Add must reach the injected store")
	require.Equal(t, "hello", fake.lastAdd)

	require.NoError(t, Remove("hello"))
	require.Equal(t, 1, fake.removes, "Remove must reach the injected store")

	got, err := Contains("hello")
	require.NoError(t, err)
	require.False(t, got, "Contains must reach the injected store")
}
