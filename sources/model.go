package sources

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

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

func loadAllCss() (string, error) {
	var a []string
	filepath.WalkDir(util.DictsPath(), func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ".css" {
			log.Infof("name: %v\n", filepath.Join(s, d.Name()))
			if content, err := os.ReadFile(s); err == nil {
				a = append(a, string(content))
			} else {
				return err
			}
			return nil
		}
		return nil
	})
	return strings.Join(a, "\n"), nil
}

func (d *MdxDict) Register(fzf bool, mdd bool, lazy bool) error {
	d.MdxDict = loadDecodedMdx(d.MdxFile, fzf, mdd, lazy)
	if contents, err := os.ReadFile(d.MdxCss); err == nil {
		d.MdxCss = string(contents)
	} else {
		if css, err := loadAllCss(); err != nil {
			log.Debugf("load dicts[%v] css err: %v", d.MdxFile, err)
		} else {
			d.MdxCss = string(css)
		}
	}
	if !fzf {
		d.searcher = NewAho(d.MdxDict)
	} else {
		d.searcher = NewExact(d.MdxDict)
	}
	return nil
}
