package wordbank

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
ON CONFLICT(word) DO UPDATE SET update_time=datetime('now', 'localtime');
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
	create_time DATETIME NOT NULL DEFAULT (datetime('now', 'localtime')),
	update_time DATETIME NOT NULL DEFAULT (datetime('now', 'localtime'))
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
