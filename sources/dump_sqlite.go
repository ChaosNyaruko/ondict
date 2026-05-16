package sources

import (
	"database/sql"
	"fmt"

	"github.com/ChaosNyaruko/ondict/decoder"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

const dumpStatusKey = "dump_status"

// IsDumpComplete returns true only when vocab.db exists and was fully written.
func IsDumpComplete(dbPath string) bool {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return false
	}
	defer db.Close()
	var val string
	err = db.QueryRow(`SELECT value FROM search_meta WHERE key = ?`, dumpStatusKey).Scan(&val)
	return err == nil && val == "done"
}

func DumpMDXFilesToSQLite(dbPath string, mdxPaths []string, limit int, tokenizer string) error {
	db, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		return fmt.Errorf("open db err: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}
	if err := resetVocabTable(db); err != nil {
		return err
	}
	// Mark in-progress so a crash leaves the db in a detectable incomplete state.
	if err := setDumpStatus(db, "in_progress"); err != nil {
		return err
	}
	for _, mdxPath := range mdxPaths {
		if err := dumpSingleMDXToSQLite(db, mdxPath, limit); err != nil {
			return err
		}
	}
	if err := BuildDefinitionSearchIndex(db, tokenizer); err != nil {
		return fmt.Errorf("build definition search index: %v", err)
	}
	// Only mark done after everything (including FTS index) succeeded.
	if err := setDumpStatus(db, "done"); err != nil {
		return err
	}
	return nil
}

func setDumpStatus(db *sql.DB, status string) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS search_meta(
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("ensure search_meta: %w", err)
	}
	_, err = db.Exec(`INSERT INTO search_meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, dumpStatusKey, status)
	return err
}

func resetVocabTable(db *sql.DB) error {
	_, err := db.Exec(`DROP TABLE IF EXISTS vocab;
CREATE TABLE IF NOT EXISTS vocab(
    word TEXT NOT NULL COLLATE NOCASE,
    src TEXT NOT NULL DEFAULT "",
    def TEXT NOT NULL DEFAULT "",
    def_text TEXT NOT NULL DEFAULT ""
)`)
	if err != nil {
		return fmt.Errorf("create table error: %v", err)
	}
	return nil
}

func dumpSingleMDXToSQLite(db *sql.DB, mdxPath string, limit int) error {
	m := &decoder.MDict{}
	err := m.Decode(mdxPath, false)
	if err != nil {
		return fmt.Errorf("failed to decode mdx file[%v], err: %v", mdxPath, err)
	}
	defer m.Close()

	log.Infof("Decoding dict %q......", mdxPath)
	words, err := m.DumpDict(limit)
	if err != nil {
		return fmt.Errorf("DumpDict %v err: %v", mdxPath, err)
	}

	log.Infof("Inserting dict to database %q.....", mdxPath)
	bar := progressbar.Default(int64(len(words)), fmt.Sprintf("Inserting dict to database: %v", mdxPath))

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT INTO vocab (word, src, def, def_text) VALUES (?, ?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for k, vs := range words {
		for _, v := range vs {
			if _, err := stmt.Exec(k, mdxPath, v, extractVisibleText(v)); err != nil {
				log.Errorf("insert word %v, err: %v", k, err)
				continue
			}
		}
		_ = bar.Add(1)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Infof("Dump %q success!", mdxPath)
	return nil
}
