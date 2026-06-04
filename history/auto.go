// Package history records every word the user queries.
//
// The package presents the legacy [Writer] interface and [History] aggregator
// so existing call sites stay untouched. Internally the SQLite writer
// delegates to a process-singleton [store.HistoryStore] keyed by the path
// returned from util.HistoryDB(); see ADR 0001 / D7 for why we picked the
// "interface + singleton shim" shape over a full call-site migration.
package history

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/store"
	"github.com/ChaosNyaruko/ondict/util"
)

// Writer is the fan-out append target. Implementations must be safe for
// concurrent use from a single History.
type Writer interface {
	io.Closer
	Append(word string) error
}

// History fans every Append out to N writers.
type History struct {
	writers []Writer
}

func NewHistory(ws ...Writer) *History {
	return &History{writers: ws}
}

func (h *History) Append(word string) error {
	for _, w := range h.writers {
		if err := w.Append(word); err != nil {
			log.Errorf("append word %q error: %v", word, err)
		}
	}
	return nil
}

// Word is the legacy text-formatted view of a history entry.
type Word struct {
	Name       string
	Count      int
	CreateTime string
	UpdateTime string
	DeletedAt  string
}

func (w *Word) String() string {
	return fmt.Sprintf("%-20v|%v|%v ", w.Name, w.UpdateTime, w.Count)
}

// Review returns the most-recent words queried within the last `days` days
// with at least `count` occurrences. Timestamps are compared in UTC (post
// schema v1; see ADR 0001).
func (h *History) Review(days string, count string) (string, error) {
	d, err := strconv.Atoi(days)
	if err != nil {
		return "", err
	}
	cnt, err := strconv.Atoi(count)
	if err != nil {
		return "", err
	}
	s, err := historyStore()
	if err != nil {
		return "", err
	}
	rows, err := s.Review(context.Background(), d, cnt)
	if err != nil {
		return "", err
	}
	res := make([]string, 0, len(rows))
	for _, r := range rows {
		w := Word{Name: r.Word, Count: r.Count, CreateTime: r.CreateTime, UpdateTime: r.UpdateTime}
		res = append(res, w.String())
	}
	return strings.Join(res, "\n"), nil
}

// ---------------------------------------------------------------------------
// TxtWriter — flat append-only log; unchanged by the sync work.
// ---------------------------------------------------------------------------

var _ Writer = &TxtWriter{}

type TxtWriter struct {
	loc *time.Location
	fd  *os.File
}

func NewTxtWriter() *TxtWriter {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		log.Warnf("LoadLocation err for History: %v", err)
	}
	return &TxtWriter{
		loc: loc,
		fd:  nil, // TODO: a singleton fd
	}
}

func (w *TxtWriter) Close() error {
	if w == nil || w.fd == nil {
		return nil
	}
	return w.fd.Close()
}

func (w *TxtWriter) Append(word string) error {
	// TODO: log rotation to avoid too-big files
	t := util.HistoryTable()
	table, err := os.OpenFile(t, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open %s err: %v", t, err)
	}
	defer table.Close()
	if _, err := table.WriteString(fmt.Sprintf("%s | %s\n", time.Now().In(w.loc), word)); err != nil {
		return fmt.Errorf("write a record error: %v", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sqlite3Writer — delegates to the process-singleton store.HistoryStore.
// ---------------------------------------------------------------------------

var _ Writer = &Sqlite3Writer{}

// Sqlite3Writer is preserved for API compatibility. Every Append delegates
// to the singleton store.HistoryStore.
type Sqlite3Writer struct{}

func NewSqlite3Writer() *Sqlite3Writer { return &Sqlite3Writer{} }

func (w *Sqlite3Writer) Close() error { return nil }

func (w *Sqlite3Writer) Append(word string) error {
	s, err := historyStore()
	if err != nil {
		log.Errorf("history store: %v", err)
		return err
	}
	if err := s.Append(context.Background(), word); err != nil {
		log.Errorf("INSERT word %q error: %v", word, err)
		return nil // legacy behaviour: swallow per-write errors
	}
	return nil
}

// ---------------------------------------------------------------------------
// Process-singleton store.HistoryStore.
// ---------------------------------------------------------------------------

var (
	singletonMu       sync.Mutex
	singletonImpl     store.HistoryStore
	singletonPath     string
	singletonInjected bool // true when the impl was supplied via SetStore
)

// SetStore replaces the process-singleton HistoryStore. Symmetric with
// wordbank.SetStore; useful for tests and for sync wrappers. An injected
// store is owned by the caller and is never replaced by storeImpl until
// SetStore(nil) is called.
func SetStore(s store.HistoryStore) {
	singletonMu.Lock()
	defer singletonMu.Unlock()
	singletonImpl = s
	singletonPath = ""
	singletonInjected = s != nil
}

func historyStore() (store.HistoryStore, error) {
	singletonMu.Lock()
	defer singletonMu.Unlock()
	if singletonInjected {
		return singletonImpl, nil
	}
	want := util.HistoryDB()
	if singletonImpl != nil && singletonPath == want {
		return singletonImpl, nil
	}
	if singletonImpl != nil && singletonPath != want {
		_ = singletonImpl.Close()
		singletonImpl = nil
	}
	h, err := store.OpenSQLiteHistory(want)
	if err != nil {
		return nil, err
	}
	singletonImpl = h
	singletonPath = want
	return singletonImpl, nil
}
