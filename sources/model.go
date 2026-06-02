package sources

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/util"
)

var mu sync.Mutex // owns history
var history map[string]string = make(map[string]string)
var historyFile string

func getHistoryFile() string {
	if historyFile == "" {
		historyFile = util.HistoryFile()
	}
	return historyFile
}

type Dict interface {
	Keys() []string
	Get(string) string
}

type RawOutput interface {
	GetMatch() string
	GetDefinition() string
	GetSrc() string
}

type Searcher interface {
	GetRawOutputs(string) []RawOutput
}

type Source interface {
	Register() error
	Get(string) []string
}

type output struct {
	rawWord string `db:"word"`
	src     string `db:"src"`
	def     string `db:"def"`
}

func (o output) GetMatch() string {
	return o.rawWord
}

func (o output) GetDefinition() string {
	return o.def
}

func (o output) GetSrc() string {
	return o.src
}

func loadAllCss() (string, error) {
	var files []string
	dictsPath := util.DictsPath()
	if _, err := os.Stat(dictsPath); os.IsNotExist(err) {
		return "", nil
	}
	err := filepath.WalkDir(dictsPath, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".css" {
			files = append(files, s)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Load vanilla/base CSS files first so that more specific non-vanilla
	// rules (e.g. showing .HWD) come later and take precedence in the cascade.
	sort.SliceStable(files, func(i, j int) bool {
		iVanilla := strings.Contains(strings.ToLower(filepath.Base(files[i])), "vanilla")
		jVanilla := strings.Contains(strings.ToLower(filepath.Base(files[j])), "vanilla")
		if iVanilla != jVanilla {
			return iVanilla // vanilla files sort first
		}
		return files[i] < files[j]
	})

	var a []string
	for _, s := range files {
		log.Infof("loading css: %v", s)
		content, err := os.ReadFile(s)
		if err != nil {
			return "", err
		}
		a = append(a, string(content))
	}
	// LDOCE5++ LM5Switch.js uses .pagetitle { border-top-style } as a sentinel
	// to detect whether it is running inside a real MDD viewer. If the style is
	// not "double" it calls lm5pp_removePictureAndSound() which removes all
	// .fa-volume-up elements — hiding every speaker icon. Override it here so
	// the JS sees a "real" viewer and leaves the audio elements intact.
	a = append(a, ".pagetitle { border-top-style: double; }")
	return strings.Join(a, "\n"), nil
}

func (d *MdxDict) registerDictDB() error {
	d.MdxDict = &DBDict{}
	d.searcher = NewDBIExact()
	*G = append(*G, d)
	return nil
}

// TODO: make the option easier to maintain.
func (d *MdxDict) Register(fzf bool, mdd bool, lazy bool) error {
	d.MdxDict = loadDecodedMdx(d.MdxFile, fzf, mdd, lazy)
	if !fzf {
		log.Infof("stuck NewAho ok")
		d.searcher = NewAho(d.MdxDict)
	} else {
		log.Infof("stuck IExact ok")
		d.searcher = NewIExact(d.MdxDict)
	}
	log.Infof("Register ok")
	return nil
}
