package sources

import (
	"database/sql"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"

	"github.com/ChaosNyaruko/ondict/util"
)

const (
	DefinitionTokenizerUnicode61 = "unicode61"
	DefinitionTokenizerTrigram   = "trigram"
)

type DefinitionMatch struct {
	Word    string
	Src     string
	Snippet string
	Score   float64
}

type DefinitionSearchError struct {
	Reason string
}

func (e *DefinitionSearchError) Error() string {
	return e.Reason
}

func normalizeDefinitionTokenizer(tokenizer string) string {
	switch strings.ToLower(strings.TrimSpace(tokenizer)) {
	case "", DefinitionTokenizerUnicode61:
		return DefinitionTokenizerUnicode61
	case DefinitionTokenizerTrigram:
		return DefinitionTokenizerTrigram
	default:
		return DefinitionTokenizerUnicode61
	}
}

func ActiveDefinitionTokenizer() string {
	cfg, err := ReadConfig()
	if err != nil {
		return DefinitionTokenizerUnicode61
	}
	return normalizeDefinitionTokenizer(cfg.Search.DefinitionIndex.Tokenizer)
}

func vocabDBPath() string {
	return filepath.Join(util.ConfigPath(), "vocab.db")
}

func openVocabDB() (*sql.DB, error) {
	return sql.Open("sqlite3", "file:"+vocabDBPath())
}

func BuildDefinitionSearchIndex(db *sql.DB, tokenizer string) error {
	tokenizer = normalizeDefinitionTokenizer(tokenizer)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS search_meta(
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create search_meta: %w", err)
	}
	if _, err := db.Exec(`DROP TABLE IF EXISTS vocab_fts`); err != nil {
		return fmt.Errorf("drop vocab_fts: %w", err)
	}
	stmt := fmt.Sprintf(`CREATE VIRTUAL TABLE vocab_fts USING fts5(
		word,
		src UNINDEXED,
		def_text,
		tokenize = '%s'
	)`, tokenizer)
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("create vocab_fts: %w", err)
	}

	hasDefText, err := vocabHasColumn(db, "def_text")
	if err != nil {
		return fmt.Errorf("inspect vocab schema: %w", err)
	}

	selectSQL := `SELECT word, src, def FROM vocab`
	if hasDefText {
		selectSQL = `SELECT word, src, def_text FROM vocab`
	}

	rows, err := db.Query(selectSQL)
	if err != nil {
		return fmt.Errorf("select vocab rows: %w", err)
	}
	defer rows.Close()

	totalRows := 0
	if err := db.QueryRow(`SELECT COUNT(*) FROM vocab`).Scan(&totalRows); err != nil {
		return fmt.Errorf("count vocab rows: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin fts transaction: %w", err)
	}
	stmtInsert, err := tx.Prepare(`INSERT INTO vocab_fts(word, src, def_text) VALUES(?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare fts insert: %w", err)
	}
	defer stmtInsert.Close()

	bar := progressbar.NewOptions(
		totalRows,
		progressbar.OptionSetWriter(progressWriter()),
		progressbar.OptionSetDescription(fmt.Sprintf("building definition index (%s)", tokenizer)),
		progressbar.OptionSetWidth(24),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("rows"),
		progressbar.OptionThrottle(65),
	)

	for rows.Next() {
		var word, src, defText string
		if err := rows.Scan(&word, &src, &defText); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("scan vocab row: %w", err)
		}
		if !hasDefText {
			defText = extractVisibleText(defText)
		}
		if _, err := stmtInsert.Exec(word, src, defText); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert fts row for %q: %w", word, err)
		}
		_ = bar.Add(1)
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("iterate vocab rows: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO search_meta(key, value) VALUES('definition_tokenizer', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, tokenizer); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store tokenizer metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit fts transaction: %w", err)
	}
	_ = bar.Finish()
	return nil
}

func vocabHasColumn(db *sql.DB, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(vocab)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func progressWriter() io.Writer {
	info, err := os.Stderr.Stat()
	if err != nil {
		return io.Discard
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return io.Discard
	}
	return os.Stderr
}

func DefinitionSearchReady() error {
	db, err := openVocabDB()
	if err != nil {
		return &DefinitionSearchError{Reason: fmt.Sprintf("open vocab db: %v", err)}
	}
	defer db.Close()
	return checkDefinitionSearchReady(db, ActiveDefinitionTokenizer())
}

func checkDefinitionSearchReady(db *sql.DB, expectedTokenizer string) error {
	expectedTokenizer = normalizeDefinitionTokenizer(expectedTokenizer)

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='vocab'`).Scan(&count); err != nil {
		return &DefinitionSearchError{Reason: fmt.Sprintf("inspect vocab table: %v", err)}
	}
	if count == 0 {
		return &DefinitionSearchError{Reason: "definition search requires vocab.db; build it first with ondict -init or dumpdict"}
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='vocab_fts'`).Scan(&count); err != nil {
		return &DefinitionSearchError{Reason: fmt.Sprintf("inspect vocab_fts: %v", err)}
	}
	if count == 0 {
		return &DefinitionSearchError{Reason: "definition search index is missing; rebuild vocab.db or run dumpdict with FTS enabled"}
	}

	var tokenizer string
	err := db.QueryRow(`SELECT value FROM search_meta WHERE key='definition_tokenizer'`).Scan(&tokenizer)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &DefinitionSearchError{Reason: "definition search index metadata is missing; rebuild vocab.db to create the configured tokenizer index"}
		}
		return &DefinitionSearchError{Reason: fmt.Sprintf("read definition search metadata: %v", err)}
	}
	tokenizer = normalizeDefinitionTokenizer(tokenizer)
	if tokenizer != expectedTokenizer {
		return &DefinitionSearchError{Reason: fmt.Sprintf("definition search index tokenizer mismatch: db=%s config=%s; rebuild vocab.db to switch tokenizer", tokenizer, expectedTokenizer)}
	}
	return nil
}

func SearchDefinitions(query string, limit int) ([]DefinitionMatch, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	db, err := openVocabDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	tokenizer := ActiveDefinitionTokenizer()
	if err := checkDefinitionSearchReady(db, tokenizer); err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT word, src, snippet(vocab_fts, 2, '<mark>', '</mark>', ' ... ', 18), bm25(vocab_fts)
		FROM vocab_fts
		WHERE vocab_fts MATCH ?
		ORDER BY bm25(vocab_fts), word
		LIMIT ?`, query, limit)
	if err != nil {
		log.Debugf("definition search err for %q: %v", query, err)
		return nil, err
	}
	defer rows.Close()

	var matches []DefinitionMatch
	for rows.Next() {
		var m DefinitionMatch
		if err := rows.Scan(&m.Word, &m.Src, &m.Snippet, &m.Score); err != nil {
			return matches, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func extractVisibleText(raw string) string {
	raw = util.ReplaceLINK(raw)
	tokenizer := html.NewTokenizer(strings.NewReader(raw))
	var b strings.Builder
	skipDepth := 0
	appendSpace := func() {
		if b.Len() == 0 {
			return
		}
		if s := b.String(); len(s) > 0 && !strings.HasSuffix(s, " ") {
			b.WriteByte(' ')
		}
	}

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			res := strings.Join(strings.Fields(b.String()), " ")
			return strings.TrimSpace(res)
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "script" || string(name) == "style" {
				skipDepth++
			} else if string(name) == "br" || string(name) == "p" || string(name) == "div" || string(name) == "li" {
				appendSpace()
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "script" || string(name) == "style" {
				if skipDepth > 0 {
					skipDepth--
				}
			} else if string(name) == "p" || string(name) == "div" || string(name) == "li" {
				appendSpace()
			}
		case html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == "br" {
				appendSpace()
			}
		case html.TextToken:
			if skipDepth > 0 {
				continue
			}
			text := strings.TrimSpace(stdhtml.UnescapeString(string(tokenizer.Text())))
			if text == "" {
				continue
			}
			appendSpace()
			b.WriteString(text)
		}
	}
}
