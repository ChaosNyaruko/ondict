package sources

import (
	"log"
	"os"
	"path/filepath"
	"sync"
)

var mu sync.Mutex // owns history
var history map[string]string = make(map[string]string)
var dataPath string
var historyFile string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	configPath := filepath.Join(home, ".config")
	dataPath = filepath.Join(configPath, "ondict")
	historyFile = filepath.Join(dataPath, "history.json")
	if dataPath == "" || historyFile == "" {
		log.Fatalf("empty datapath/historyfile: %v||%v", dataPath, historyFile)
	}
}

type RawOutput interface {
	GetMatch() string
	GetDefinition() string
}

type Searcher interface {
	GetRawOutputs(string) []RawOutput
}

type Source interface {
	Register() error
	Get(string) []string
}

type output struct {
	rawWord string
	def     string
}

func (o output) GetMatch() string {
	return o.rawWord
}

func (o output) GetDefinition() string {
	return o.def
}

func (d *MdxDict) Load() error {
	d.mdxDict = loadDecodedMdx(filepath.Join(dataPath, "dicts", d.mdxFile))
	if contents, err := os.ReadFile((filepath.Join(dataPath, "dicts", d.mdxCss))); err == nil {
		d.mdxCss = string(contents)
	} else {
		d.mdxCss = ""
		log.Printf("load dicts[%v] css err: %v", d.mdxFile, err)
	}
	d.searcher = New(d.mdxDict)
	return nil
}
