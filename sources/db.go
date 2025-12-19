package sources

import (
	"database/sql"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/util"
)

// IExact is a case in-sensitive search source, no other fuzzy searching algos.
type DBIExact struct {
}

var _ Searcher = &DBIExact{}

func NewDBIExact() Searcher {
	return &DBIExact{}
}

func (e *DBIExact) GetRawOutputs(input string) []RawOutput {
	dbName := filepath.Join(util.ConfigPath(), "vocab.db")
	db, err := sql.Open("sqlite3", "file:"+dbName)
	if err != nil {
		log.Errorf("open db err: %v", err)
		return nil
	}
	defer db.Close()
	res := make([]RawOutput, 0, 1)

	// NOTE: we don't very care about the security problem here.
	// And based on this doc https://go.dev/doc/database/sql-injection, there will not be sql injection problem.
	rows, err := db.Query("SELECT * FROM vocab WHERE word = ?", input)
	if err != nil {
		log.Errorf("select from vocab error: %v", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var ro output
		if err := rows.Scan(&ro.rawWord, &ro.src, &ro.def); err != nil {
			log.Errorf("scan row for %q err: %v", input, err)
			return res
		}
		res = append(res, ro)
	}
	return res
}

type DBDict struct {
}

func (d *DBDict) Keys() []string {
	return nil
}

func (d *DBDict) Get(s string) string {
	panic("We don't call DBDict.Get directly, use searcher instead")
}

func (d *DBDict) WordsWithPrefix(prefix string) []string {
	dbName := filepath.Join(util.ConfigPath(), "vocab.db")
	db, err := sql.Open("sqlite3", "file:"+dbName)
	if err != nil {
		log.Errorf("open db err: %v", err)
		return nil
	}
	defer db.Close()

	pattern := prefix + "%"

	// NOTE: we don't very care about the security problem here.
	// And based on this doc https://go.dev/doc/database/sql-injection, there will not be sql injection problem.
	rows, err := db.Query("SELECT word FROM vocab WHERE word LIKE ?", pattern)
	if err != nil {
		log.Errorf("select %q from vocab error: %v", prefix, err)
		return nil
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var ro output
		if err := rows.Scan(&ro.rawWord); err != nil {
			log.Errorf("scan row for %q err: %v", prefix, err)
			return res
		}
		res = append(res, ro.rawWord)
	}
	return res
}
