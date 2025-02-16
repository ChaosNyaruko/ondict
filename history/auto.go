package history

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"
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

type Word struct {
	Name       string
	Count      int
	CreateTime string
	UpdateTime string
}

func (w *Word) NormTime(loc *time.Location) error {
	// TODO: ''bad review request: parsing time "2025-02-15T18:00:27Z" as "2006-01-02 15:04:05": cannot parse "T18:00:27Z" as " "'
	// > SQLite does not have a storage class set aside for storing dates and/or times. Instead, the built-in Date And Time Functions of SQLite are capable of storing dates and times as TEXT, REAL, or INTEGER values:
	return nil
	ct, err := time.ParseInLocation("2006-01-02 15:04:05", w.CreateTime, loc)
	if err != nil {
		return err
	}
	ut, err := time.ParseInLocation("2006-01-02 15:04:05", w.UpdateTime, loc)
	if err != nil {
		return err
	}
	w.CreateTime = ct.String()
	w.UpdateTime = ut.String()
	return nil
}

func (w *Word) String() string {
	return fmt.Sprintf("%-20v|%v|%v ", w.Name, w.UpdateTime, w.Count)
}

func (h *History) Review() (string, error) {
	// TODO: refactor
	dbName := util.HistoryDB()
	log.Debugf("Connected to %v!", dbName)
	db, err := sql.Open("sqlite3", "file:"+dbName)
	if err != nil {
		log.Errorf("open db err: %v", err)
		return "", err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT * FROM history
		WHERE update_time > datetime('now', 'localtime', '-7 days')
		ORDER BY update_time DESC;
		`)
	if err != nil {
		log.Errorf("query most frequently queried words error: %v", err)
		return "", err
	}
	defer rows.Close()
	var res []string
	// Loop through rows, using Scan to assign column data to struct fields.
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.Name, &w.Count, &w.CreateTime, &w.UpdateTime); err != nil {
			return "", fmt.Errorf("Review words Scan: %v", err)
		}
		if err := w.NormTime(loc); err != nil {
			return "", err
		}
		res = append(res, w.String())
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("Review words: %v", err)
	}
	return strings.Join(res, "\n"), nil
}

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
		// TODO: a singleton fd
		fd: nil,
	}
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
	if _, err := table.WriteString(fmt.Sprintf("%s | %s\n", time.Now().In(w.loc), word)); err != nil {
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
    create_time DATETIME NOT NULL DEFAULT (datetime('now', 'localtime')),
    update_time DATETIME NOT NULL DEFAULT (datetime('now', 'localtime'))
);
`)
	if err != nil {
		return err
	}
	res, err = db.Exec(`INSERT INTO history (word, count) VALUES (?, 1) ON CONFLICT(word) DO UPDATE SET count=count+1, update_time=datetime('now','localtime');`, word)
	id, err := res.LastInsertId()
	if err != nil {
		log.Errorf("INSERT error: %v, %v", id, err)
	}

	return nil
}
