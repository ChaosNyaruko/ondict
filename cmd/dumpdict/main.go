// This program dumps a "MDX" format dictionary file into a sqlite database file.
// Use "dumpdict -h" for more.
package main // go install github.com/ChaosNyaruko/ondict/cmd/dumpdict@latest

import (
	"flag"
	"io/fs"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/sources"
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
var definitionTokenizer = flag.String("fts-tokenizer", "unicode61", "Tokenizer used by the SQLite definition search index: unicode61 or trigram")

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
	dbName := util.VocabDB()
	var mdxPaths []string
	for _, name := range files {
		mdxPaths = append(mdxPaths, name)
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
				mdxPaths = append(mdxPaths, s)
			}
			return nil
		})
	}
	if err := sources.DumpMDXFilesToSQLite(dbName, mdxPaths, 0, *definitionTokenizer); err != nil {
		log.Fatalf("dump sqlite err: %v", err)
	}
}
