package sources

import (
	"log"
	"os"
	"sync"

	"github.com/ChaosNyaruko/ondict/util"
)

var mu sync.Mutex // owns history
var history map[string]string = make(map[string]string)
var historyFile string

func init() {
	historyFile = util.HistoryFile()
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
	d.MdxDict = loadDecodedMdx(d.MdxFile)
	if contents, err := os.ReadFile(d.MdxCss); err == nil {
		d.MdxCss = string(contents)
	} else {
		d.MdxCss = ""
		log.Printf("load dicts[%v] css err: %v", d.MdxFile, err)
	}
	d.searcher = New(d.MdxDict)
	return nil
}
