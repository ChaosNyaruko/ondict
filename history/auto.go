package history

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/util"
)

type Writer interface { // size=16 (0x10)
	io.Closer
	Append(word string) error
	// Write(log RecyclableLog) error
	// Flush() error
}

type History struct {
	writers []Writer
}

func NewHistory(ws ...Writer) *History {
	return &History{
		writers: ws,
	}
}

func (h *History) Append(word string) error {
	for _, w := range h.writers {
		if err := w.Append(word); err != nil {
			log.Errorf("append word %q error: %v", word, err)
		}
	}
	return nil
}

var _ Writer = &TxtWriter{}

type TxtWriter struct {
	fd *os.File
}

func NewTxtWriter() *TxtWriter {
	// TODO: a singleton fd
	return &TxtWriter{}
}

func (w *TxtWriter) Close() error {
	if w == nil {
		panic("nil TxtWriter!")
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
	if _, err := table.WriteString(fmt.Sprintf("%s | %s\n", time.Now(), word)); err != nil {
		return fmt.Errorf("write a record error: %v", err)
	}
	defer table.Close()

	return nil
}

type Sqlite3Writer struct {
	db *sql.DB
}

func NewSqlite3Writer() *Sqlite3Writer {
	// TODO: a singleton fd
	return &Sqlite3Writer{}
}

func (w *Sqlite3Writer) Close() error {
	if w == nil {
		panic("nil Sqlite3Writer!")
	}
	return w.db.Close()
}

func (w *Sqlite3Writer) Append(word string) error {
	dbName := util.HistoryDB()
	log.Debugf("Connected to %v!", dbName)
	db, err := sql.Open("sqlite3", "file:"+dbName)
	if err != nil {
		log.Errorf("open db err: %v", err)
		return err
	}
	defer db.Close()

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	log.Infof("Connected!")

	res, err := db.Exec(`CREATE TABLE IF NOT EXISTS history (
    word TEXT NOT NULL UNIQUE,
	` +
		"`count`" + ` INTEGER NOT NULL DEFAULT 0,
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
	if err != nil {
		return err
	}
	res, err = db.Exec(`INSERT INTO history (word, count) VALUES (?, 1) ON CONFLICT(word) DO UPDATE SET count=count+1, update_time=CURRENT_TIMESTAMP;`, word)
	id, err := res.LastInsertId()
	if err != nil {
		log.Errorf("INSERT error: %v, %v", id, err)
	}

	return nil
}
