// This program dumps a "MDX" format dictionary file into a sqlite database file.
// Use "dumpdict -h" for more.
package main // go install github.com/ChaosNyaruko/ondict/cmd/dumpdict@latest

import (
	"database/sql"
	"flag"
	"io/fs"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/util"
)

var help = flag.Bool("h", false, "Show this help doc")

type List []string

func (i *List) String() string {
	return "my string representation"
}

func (i *List) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var files List
var srcDirs List

// var dir= flag.String("q", "", "Specify the word that you want to query")

func main() {
	flag.Var(&files, "f", "Specify the mdx files that you want to dump. It can be used multiple times for more than one dicts")
	flag.Var(&srcDirs, "d", "Specify the directory in which you want to dump all the mdx files contained.")
	flag.Parse()
	// log.Infof("%v, %v", flag.NFlag(), flag.Args())
	if *help || flag.NFlag() == 0 || len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}
	if len(files) == 0 && len(srcDirs) == 0 {
		log.Fatalf("no file or directory specified")
	}
	dbName := filepath.Join(util.ConfigPath(), "vocab.db")
	db, err := sql.Open("sqlite3", "file:"+dbName)
	if err != nil {
		log.Errorf("open db err: %v", err)
		return
	}
	defer db.Close()

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
	log.Infof("Connected!")

	res, err := db.Exec(`DROP TABLE IF EXISTS vocab;
CREATE TABLE IF NOT EXISTS vocab(
    word TEXT NOT NULL,
    src TEXT NOT NULL DEFAULT "",
    def TEXT NOT NULL DEFAULT ""
)`)
	id, err := res.LastInsertId()
	if err != nil {
		log.Errorf("INSERT error: %v, %v", id, err)
	}
	for _, name := range files {
		dump(db, name)
	}
	for _, dir := range srcDirs {
		root, err := filepath.Abs(dir)
		if err != nil {
			log.Warnf("Skip the bad directory: %q", dir)
		}
		filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if filepath.Ext(d.Name()) == ".mdx" {
				dump(db, s)
			}
			return nil
		})
	}
}

func dump(db *sql.DB, name string) {
	m := &decoder.MDict{}
	err := m.Decode(name, false)
	if err != nil {
		log.Fatalf("Failed to decode mdx file[%v], err: %v", name, err)
	}
	defer m.Close()
	log.Infof("Decoding dict %q......", name)
	words, err := m.DumpDict()
	if err != nil {
		log.Fatalf("DumpDict %v err: %v", name, err)
	}
	log.Infof("Dumping dict %q.....", name)
	for k, v := range words {
		result, err := db.Exec("INSERT INTO vocab (word, src, def) VALUES (?, ?, ?)", k, name, v)
		if err != nil {
			log.Errorf("insert word %v, err: %v", k, err)
			continue
		}
		id, err := result.LastInsertId()
		if err != nil {
			log.Errorf("LastInsertId err word %v, err: %v", k, err)
			continue
		} else {
			log.Debugf("LastInsertId word %v: %v", k, id)
		}
	}
	log.Infof("Dump %q success!", name)
}
