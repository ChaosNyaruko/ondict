package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/render"
	"github.com/ChaosNyaruko/ondict/util"
)

var Gbold = "**"
var Gitalic = "*"

// OnMDDDumped is an optional callback invoked after MDD resources are
// successfully extracted. Set this before calling G.Load() to write a
// marker file on mobile so the dump is skipped on subsequent launches.
var OnMDDDumped func()

// mddFiles holds loaded MDD decoders for on-demand file extraction.
// Populated during G.Load() when mdd=false (lazy/mobile mode).
var mddFiles []*decoder.MDict
var mddMu sync.Mutex

// GetMDDFile looks up a resource file (e.g. "GB_hello0205.mp3") across all
// loaded MDD dictionaries and returns its raw bytes. Returns nil if not found.
func GetMDDFile(name string) []byte {
	mddMu.Lock()
	defer mddMu.Unlock()
	for _, mdd := range mddFiles {
		if data := mdd.GetFile(name); data != nil {
			return data
		}
	}
	return nil
}

type Dicts []*MdxDict

var G = &Dicts{}
var once sync.Once

func (g *Dicts) Load(fzf bool, mdd bool, lazy bool) error {
	once.Do(func() {
		t0 := time.Now()
		// Use the SQLite vocab.db when it was fully written on a previous run.
		dbPath := util.VocabDB()
		if IsDumpComplete(dbPath) {
			d := &MdxDict{
				Type:     render.LongmanEasy, // TODO: may need some other abstractions
				MdxFile:  "vocab.db",
				MdxDict:  nil,
				searcher: nil,
			}
			if d.registerDictDB() == nil {
				log.Infof("[timing] vocab.db loaded in %v — skipping MDX decode", time.Since(t0))
				return
			}
			log.Warnf("vocab.db exists but registerDictDB failed; falling back to MDX")
		} else if _, err := os.Stat(dbPath); err == nil {
			// Partial/corrupt db from a previous crashed dump — remove it so we
			// start fresh and re-trigger the background dump below.
			log.Warnf("vocab.db is incomplete, removing and rebuilding")
			_ = os.Remove(dbPath)
		}

		if err := LoadConfig(); err != nil {
			log.Fatalf("load config err: %v", err)
		}
		log.Infof("stuck at Register")
		for _, d := range *g {
			d.Register(fzf, mdd, lazy)
		}
		log.Infof("[timing] MDX Register took %v", time.Since(t0))

		// Background: dump all MDX files to vocab.db so the next launch is faster.
		go func() {
			c, err := ReadConfig()
			if err != nil {
				log.Warnf("auto-dump: could not read config: %v", err)
				return
			}
			var mdxPaths []string
			for _, dc := range c.Dicts {
				if dc.Enabled != nil && !*dc.Enabled {
					continue
				}
				mdxPaths = append(mdxPaths, filepath.Join(util.DictsPath(), dc.Name+".mdx"))
			}
			if len(mdxPaths) == 0 {
				return
			}
			log.Infof("auto-dump: starting background MDX→SQLite dump for %d dict(s)", len(mdxPaths))
			tDump := time.Now()
			if err := DumpMDXFilesToSQLite(dbPath, mdxPaths, 0, c.Search.DefinitionIndex.Tokenizer); err != nil {
				log.Errorf("auto-dump: failed: %v", err)
				_ = os.Remove(dbPath) // don't leave a broken db
				return
			}
			log.Infof("auto-dump: vocab.db ready in %v — next launch will use SQLite", time.Since(tDump))
		}()
	})
	log.Infof("stuck at Load")
	return nil
}

func QueryMDX(word string, f string) string {
	type mdxResult struct {
		defs []string
		t    string // SourceType
	}
	var defs []mdxResult
	for _, dict := range *G {
		defs = append(defs, mdxResult{dict.Get(word), dict.Type})
		log.Debugf("def of %q, %v: %q", dict.MdxFile, defs, word)
	}
	// TODO: put the render abstraction here?
	if f == "html" { // f for format
		var res []string
		for _, dict := range defs {
			for _, def := range dict.defs {
				h := render.HTMLRender{Raw: def, SourceType: dict.t}
				rs := fmt.Sprintf("<div>%s</div> ", h.Render())
				res = append(res, rs)
			}
		}
		return strings.Join(res, "<br><br>")
	}

	log.Debugf("query: %v, format: %v", word, f)
	var res string
	for _, dict := range defs {
		for _, def := range dict.defs {
			ren := &render.MarkdownRender{
				Raw:        def,
				SourceType: dict.t,
			}
			res += "\n----\n" + ren.Render()
		}
	}
	return res
}

func loadDecodedMdx(filePath string, fzf bool, mdd bool, lazy bool) Dict {
	jsonData, err := os.ReadFile(filePath + ".json")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("Failed to read JSON file: %v, %v", filePath, err)
	} else if errors.Is(err, os.ErrNotExist) {
		log.Debugf("JSON file not exist: %v, fzf: %v, mdd: %v", filePath+".json", fzf, mdd)
		m := &decoder.MDict{}
		err := m.Decode(filePath+".mdx", fzf)
		if !lazy {
			go func() {
				m.DumpKeys()
			}()
		}
		if mdd {
			log.Infof("The server will dump mdd resources for [%v]!", filePath+".mdd")
			go func() {
				mdd := decoder.MDict{}
				if err := mdd.Decode(filePath+".mdd", false); err != nil {
					log.Errorf("parse %v.mdd err: %v", filePath, err)
				} else {
					log.Infof("[INFO] successfully decode %v.mdd", filePath)
					if err := mdd.DumpData(); err != nil {
						log.Errorf("dump mdd err: %v", err)
					} else if OnMDDDumped != nil {
						OnMDDDumped()
					}
				}
			}()
		} else {
			// Lazy mode: load MDD for on-demand file serving without dumping.
			mddPath := filePath + ".mdd"
			if _, err := os.Stat(mddPath); err == nil {
				go func() {
					mdd := &decoder.MDict{}
					if err := mdd.Decode(mddPath, true); err != nil {
						log.Errorf("lazy load %v.mdd err: %v", filePath, err)
						return
					}
					mddMu.Lock()
					mddFiles = append(mddFiles, mdd)
					mddMu.Unlock()
					log.Infof("lazy MDD loaded: %v", mddPath)
				}()
			}
		}
		if err != nil {
			log.Fatalf("Failed to load mdx file[%v], err: %v", filePath, err)
		}
		return m
	}

	// Define a map to hold the unmarshaled data
	data := Map(make(map[string]string))

	// Unmarshal the JSON data into the map
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	return data
}

type MdxDict struct {
	// SourceType
	Type string
	// For personal usage example, "oald9.json", or "Longman Dictionary of Contemporary English"
	MdxFile  string
	MdxDict  Dict // TODO: it's "embedded" in the searcher, maybe we can remove it to reduce mem usage when apply non-plain search algorithms.
	searcher Searcher
}

func (d *MdxDict) Get(word string) []string {
	results := d.searcher.GetRawOutputs(word)
	if len(results) == 0 {
		return []string{}
	}
	// TODO: Give user the options.
	// Naive solution: Give user the longest match.
	// What about same length? Show all of them.
	var maxes, defs []string
	for _, res := range results {
		m := res.GetMatch()
		if len(maxes) == 0 || len(m) > len(maxes[0]) {
			maxes = []string{m}
			defs = []string{res.GetDefinition()}
		} else if len(m) == len(maxes[0]) {
			maxes = append(maxes, m)
			defs = append(defs, res.GetDefinition())
		}
	}
	return defs
}
