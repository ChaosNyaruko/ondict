// Package wordbank exposes the user's saved word bank via a stable
// package-level API (Add / Remove / Contains / List / ...).
//
// Internally every call delegates to a process-singleton [store.WordbankStore]
// keyed by the path returned from util.WordBankDB(). This is the
// "interface + singleton shim" approach captured in
// docs/adr/0001-wordbank-history-sync.md (D7): existing call sites stay
// untouched while the sync layer can swap the underlying store impl.
package wordbank

import (
	"context"
	"sync"
	"time"

	"github.com/ChaosNyaruko/ondict/store"
	"github.com/ChaosNyaruko/ondict/util"
)

// ErrEmptyWord is returned by mutator functions when the word is empty after
// trimming. Re-exported from package store so call sites that import only
// wordbank don't need a second import for this sentinel.
var ErrEmptyWord = store.ErrEmptyWord

// Word is the legacy shape returned by List. DeletedAt is populated only by
// ListWithDeleted; for live rows returned from List it is always empty.
type Word struct {
	Name       string
	CreateTime string
	UpdateTime string
	DeletedAt  string
}

var (
	singletonMu       sync.Mutex
	singletonImpl     store.WordbankStore
	singletonPath     string
	singletonInjected bool // true when the impl was supplied via SetStore
)

// SetStore replaces the process-singleton WordbankStore. Useful for tests
// and for the sync layer (which may want a wrapping store that broadcasts
// writes). Passing nil resets the singleton; the next package-level call
// will lazily reopen the default sqlite store.
//
// Closing the previous singleton is the caller's responsibility.
func SetStore(s store.WordbankStore) {
	singletonMu.Lock()
	defer singletonMu.Unlock()
	singletonImpl = s
	singletonPath = ""
	// An injected store is "owned" by the caller and must NOT be replaced
	// just because util.WordBankDB() resolves to a different path (the
	// previous implementation incorrectly closed and replaced it). We
	// keep the injection in effect until the caller calls SetStore(nil).
	singletonInjected = s != nil
}

// storeImpl returns the active singleton, lazily opening the default
// SQLite-backed store at util.WordBankDB() if none has been set.
//
// The path is captured per-call so that tests using t.Setenv("HOME", ...)
// re-resolve to a fresh DB on the next call. An externally injected store
// (via SetStore) bypasses path tracking entirely.
func storeImpl() (store.WordbankStore, error) {
	singletonMu.Lock()
	defer singletonMu.Unlock()
	if singletonInjected {
		return singletonImpl, nil
	}
	want := util.WordBankDB()
	if singletonImpl != nil && singletonPath == want {
		return singletonImpl, nil
	}
	// Path changed (e.g. test harness flipping HOME) or first use.
	if singletonImpl != nil && singletonPath != want {
		_ = singletonImpl.Close()
		singletonImpl = nil
	}
	wb, err := store.OpenSQLiteWordbank(want)
	if err != nil {
		return nil, err
	}
	singletonImpl = wb
	singletonPath = want
	return singletonImpl, nil
}

func Add(word string) error {
	s, err := storeImpl()
	if err != nil {
		return err
	}
	return s.Add(context.Background(), word)
}

func Remove(word string) error {
	s, err := storeImpl()
	if err != nil {
		return err
	}
	return s.Remove(context.Background(), word)
}

func Contains(word string) (bool, error) {
	s, err := storeImpl()
	if err != nil {
		return false, err
	}
	return s.Contains(context.Background(), word)
}

func List() ([]Word, error) {
	s, err := storeImpl()
	if err != nil {
		return nil, err
	}
	rows, err := s.List(context.Background())
	if err != nil {
		return nil, err
	}
	return toLegacy(rows), nil
}

// ListWithDeleted exposes tombstoned rows for the merge / sync layer.
func ListWithDeleted() ([]Word, error) {
	s, err := storeImpl()
	if err != nil {
		return nil, err
	}
	rows, err := s.ListSince(context.Background(), time.Time{})
	if err != nil {
		return nil, err
	}
	return toLegacy(rows), nil
}

func toLegacy(rows []store.WordbankRow) []Word {
	out := make([]Word, 0, len(rows))
	for _, r := range rows {
		out = append(out, Word{
			Name:       r.Word,
			CreateTime: r.CreateTime,
			UpdateTime: r.UpdateTime,
			DeletedAt:  r.DeletedAt,
		})
	}
	return out
}
