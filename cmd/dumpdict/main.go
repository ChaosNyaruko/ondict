// This program dumps a "MDX" format dictionary file into a sqlite database file.
package main // go install github.com/ChaosNyaruko/ondict/cmd/dumpdict@latest

import (
	"database/sql"
	"flag"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/util"
)

var help = flag.Bool("h", false, "Show this help doc")
var file = flag.String("f", "", "Specify the mdx file that you want to dump")

// var dir= flag.String("q", "", "Specify the word that you want to query")

func main() {
	flag.Parse()
	// log.Infof("%v, %v", flag.NFlag(), flag.Args())
	if *help || flag.NFlag() == 0 || len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}
	if *file == "" {
		log.Fatalf("no file or directory specified")
	}
	// flag: mdx files location
	// -f specific file name
	// -d all mdx files in the directory
	m := &decoder.MDict{}
	name := *file
	err := m.Decode(name, false)
	if err != nil {
		log.Fatalf("Failed to decode mdx file[%v], err: %v", name, err)
	}
	defer m.Close()
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
	log.Infof("Decoding dict......")
	words, err := m.DumpDict()
	if err != nil {
		log.Fatalf("DumpDict %v err: %v", name, err)
	}

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
	log.Infof("INSERT success: %v, %v", id, err)
	log.Infof("Dumping dict......")
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
}

func dump(db *sql.DB, name string) {
}
