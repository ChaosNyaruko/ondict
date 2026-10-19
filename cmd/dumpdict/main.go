// This program dumps a "MDX" format dictionary file into a sqlite database file.
// Use "dumpdict -h" for more.
package main // go install github.com/ChaosNyaruko/ondict/cmd/dumpdict@latest

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/decoder"
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
	flag.Var(&files, "f", "Specify the mdx or mdd files that you want to dump. It can be used multiple times for more than one dicts")
	flag.Var(&srcDirs, "d", "Specify the directory in which you want to dump all the mdx and mdd files contained.")
	flag.Parse()
	// log.Infof("%v, %v", flag.NFlag(), flag.Args())
	if *help || flag.NFlag() == 0 || len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}
	if len(files) == 0 && len(srcDirs) == 0 {
		log.Fatalf("no file or directory specified")
	}
	paths, err := collectDictPaths([]string(files), []string(srcDirs))
	if err != nil {
		log.Fatalf("collect dictionary paths err: %v", err)
	}
	if len(paths.mdx) == 0 && len(paths.mdd) == 0 {
		log.Fatalf("no mdx or mdd files found")
	}
	if len(paths.mdx) > 0 {
		dbName := util.VocabDB()
		if err := sources.DumpMDXFilesToSQLite(dbName, paths.mdx, 0, *definitionTokenizer); err != nil {
			log.Fatalf("dump sqlite err: %v", err)
		}
	} else {
		log.Infof("no mdx files found, skipping sqlite dump")
	}
	if len(paths.mdd) > 0 {
		if err := dumpMDDFiles(paths.mdd); err != nil {
			log.Fatalf("dump mdd err: %v", err)
		}
	}
}

type dictPaths struct {
	mdx []string
	mdd []string
}

func collectDictPaths(files []string, srcDirs []string) (dictPaths, error) {
	var paths dictPaths
	seenMDX := make(map[string]struct{})
	seenMDD := make(map[string]struct{})

	addPath := func(name string) bool {
		switch strings.ToLower(filepath.Ext(name)) {
		case ".mdx":
			if _, ok := seenMDX[name]; !ok {
				seenMDX[name] = struct{}{}
				paths.mdx = append(paths.mdx, name)
			}
			return true
		case ".mdd":
			if _, ok := seenMDD[name]; !ok {
				seenMDD[name] = struct{}{}
				paths.mdd = append(paths.mdd, name)
			}
			return true
		default:
			return false
		}
	}

	for _, name := range files {
		if !addPath(name) {
			return paths, fmt.Errorf("unsupported dictionary extension %q for %s", filepath.Ext(name), name)
		}
		if strings.EqualFold(filepath.Ext(name), ".mdx") {
			mddPath := strings.TrimSuffix(name, filepath.Ext(name)) + ".mdd"
			if _, err := os.Stat(mddPath); err == nil {
				addPath(mddPath)
			} else if err != nil && !os.IsNotExist(err) {
				return paths, fmt.Errorf("stat paired mdd file %s: %w", mddPath, err)
			}
		}
	}

	for _, dir := range srcDirs {
		root, err := filepath.Abs(dir)
		if err != nil {
			log.Warnf("skip the bad directory: %q", dir)
			continue
		}
		if err := filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			addPath(s)
			return nil
		}); err != nil {
			log.Warnf("skip the bad directory: %q, err: %v", dir, err)
		}
	}

	return paths, nil
}

func dumpMDDFiles(mddPaths []string) error {
	for _, mddPath := range mddPaths {
		if err := dumpMDDFile(mddPath); err != nil {
			return err
		}
	}
	return nil
}

func dumpMDDFile(mddPath string) error {
	log.Infof("Dumping MDD resources from %s to %s...", mddPath, util.TmpDir())
	m := &decoder.MDict{}
	if err := m.Decode(mddPath, false); err != nil {
		return fmt.Errorf("failed to decode mdd file[%v], err: %w", mddPath, err)
	}
	defer m.Close()

	if err := m.DumpData(); err != nil {
		return fmt.Errorf("failed to dump mdd data[%v], err: %w", mddPath, err)
	}
	log.Infof("MDD resources dumped successfully: %s", mddPath)
	return nil
}
