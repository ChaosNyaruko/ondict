package sources

import (
	"log"
	"os"
	"path/filepath"
	"sync"
)

var mu sync.Mutex // owns history
var history map[string]string = make(map[string]string)
var DataPath string
var historyFile string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	configPath := filepath.Join(home, ".config")
	DataPath = filepath.Join(configPath, "ondict")
	historyFile = filepath.Join(DataPath, "history.json")
	if DataPath == "" || historyFile == "" {
		log.Fatalf("empty datapath/historyfile: %v||%v", DataPath, historyFile)
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

func (d *MdxDict) Register() error {
	d.MdxDict = loadDecodedMdx(filepath.Join(DataPath, "dicts", d.MdxFile))
	if contents, err := os.ReadFile((filepath.Join(DataPath, "dicts", d.MdxCss))); err == nil {
		d.MdxCss = string(contents)
	} else {
		d.MdxCss = ""
		log.Printf("load dicts[%v] css err: %v", d.MdxFile, err)
	}
	d.searcher = New(d.MdxDict)
	return nil
}
