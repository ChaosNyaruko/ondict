package wordbank

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/ChaosNyaruko/ondict/util"
)

var ErrEmptyWord = errors.New("word is empty")

type Word struct {
	Name       string
	CreateTime string
	UpdateTime string
}

// toUTCString parses a timestamp string from the DB and returns it as an
// RFC 3339 UTC string (e.g. "2026-05-09T15:07:34Z").
//
// SQLite timestamps may be stored in one of two formats:
//   - "YYYY-MM-DD HH:MM:SS"   – legacy rows written with datetime('now','localtime')
//     that we treat as already-local; we can only re-emit them as-is since we
//     have no timezone metadata, so we fall back to the raw string.
//   - "YYYY-MM-DDTHH:MM:SSZ"  – UTC rows written by CURRENT_TIMESTAMP (new default).
//
// For truly correct handling the DB should store UTC, which is enforced by the
// updated schema default below.
func toUTCString(s string) string {
	// Try ISO 8601 UTC format first (new rows).
	if t, err := time.Parse("2006-01-02T15:04:05Z", s); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	// Try the space-separated format (old rows from datetime('now','localtime')).
	// We cannot know the original timezone, so return as-is.
	if _, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return s
	}
	// Already RFC3339 or unknown format – return unchanged.
	return s
}

func Add(word string) error {
	word, err := normalize(word)
	if err != nil {
		return err
	}
	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`
INSERT INTO words (word) VALUES (?)
ON CONFLICT(word) DO UPDATE SET update_time=CURRENT_TIMESTAMP;
`, word)
	return err
}

func Remove(word string) error {
	word, err := normalize(word)
	if err != nil {
		return err
	}
	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(`DELETE FROM words WHERE word = ?`, word)
	return err
}

func List() ([]Word, error) {
	db, err := open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT word, create_time, update_time FROM words ORDER BY update_time DESC, word ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var words []Word
	for rows.Next() {
		var word Word
		if err := rows.Scan(&word.Name, &word.CreateTime, &word.UpdateTime); err != nil {
			return nil, fmt.Errorf("scan word bank row: %w", err)
		}
		word.CreateTime = toUTCString(word.CreateTime)
		word.UpdateTime = toUTCString(word.UpdateTime)
		words = append(words, word)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list word bank rows: %w", err)
	}
	return words, nil
}

func Contains(word string) (bool, error) {
	word, err := normalize(word)
	if err != nil {
		if errors.Is(err, ErrEmptyWord) {
			return false, nil
		}
		return false, err
	}
	db, err := open()
	if err != nil {
		return false, err
	}
	defer db.Close()

	var found int
	err = db.QueryRow(`SELECT 1 FROM words WHERE word = ? LIMIT 1`, word).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func open() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "file:"+util.WordBankDB())
	if err != nil {
		return nil, err
	}
	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS words (
	word TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
	create_time DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
	update_time DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
);`)
	return err
}

func normalize(word string) (string, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return "", ErrEmptyWord
	}
	return word, nil
}
