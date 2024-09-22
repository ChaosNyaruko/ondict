package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/render"
	"github.com/ChaosNyaruko/ondict/util"
)

var Gbold = "**"
var Gitalic = "*"

type Dicts []*MdxDict

var G = &Dicts{}
var once sync.Once

func (g *Dicts) Load(fzf bool, mdd bool) error {
	once.Do(func() {
		for _, d := range *g {
			d.Register(fzf, mdd)
		}
		log.Debugf("loading g")
	})
	return nil
}

func QueryMDX(word string, f string) string {
	type mdxResult struct {
		defs []string
		css  string
		t    string // SourceType
	}
	var defs []mdxResult
	for _, dict := range *G {
		defs = append(defs, mdxResult{dict.Get(word), dict.CSS(), dict.Type})
		log.Debugf("def of %q, %v: %q", dict.MdxFile, defs, word)
	}
	// TODO: put the render abstraction here?
	if f == "html" { // f for format
		var res []string
		for _, dict := range defs {
			for _, def := range dict.defs {
				h := render.HTMLRender{Raw: def, SourceType: dict.t}
				// m1 := regexp.MustCompile(`<img src="(.*?)\.png" style`)
				// replaceImg := m1.ReplaceAllString(def, `<img src="`+"data/"+`${1}.png" style`)
				// log.Debugf("try to replace %v", replaceImg)
				// TODO: it might be overriden
				rs := fmt.Sprintf("<div>%s<style>%s</style></div> ", h.Render(), dict.css)
				if strings.Contains(dict.t, "Online") {
					rs = fmt.Sprintf("<script>%s</script>%v", util.CommonJS, rs)
				}
				// rs := fmt.Sprintf("%s", h.Render())
				res = append(res, rs)
			}
		}
		return strings.Join(res, "<br><br>")
	}

	log.Debugf("query: %v, format: %v", word, f)
	var res string
	for i, dict := range defs {
		for _, def := range dict.defs {
			if dict.t == render.LongmanEasy {
				fd := strings.NewReader(def)
				res += "\n---\n" + render.ParseMDX(fd, f)
			} else if dict.t == render.Longman5Online {
				fd := strings.NewReader(def)
				res += "\n--\n" + render.ParseHTML(fd)
			} else {
				log.Debugf("undefined markdown render for %dth dict, whose type is %v", i, dict.t)
			}
		}
	}
	return res
}

func loadDecodedMdx(filePath string, fzf bool, mdd bool) Dict {
	jsonData, err := os.ReadFile(filePath + ".json")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("Failed to read JSON file: %v, %v", filePath, err)
	} else if errors.Is(err, os.ErrNotExist) {
		log.Debugf("JSON file not exist: %v", filePath+".json")
		m := &decoder.MDict{}
		err := m.Decode(filePath+".mdx", fzf)
		if !fzf && mdd {
			// FIXME: dump on demand
			go func() {
				mdd := decoder.MDict{}
				if err := mdd.Decode(filePath+".mdd", false); err != nil {
					log.Debugf("[WARN] parse %v.mdd err: %v", filePath, err)
				} else {
					log.Debugf("[INFO] successfully decode %v.mdd", filePath)
					if err := mdd.DumpData(); err != nil {
						log.Fatalf("dump mdd err: %v", err)
					}
				}
			}()
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
	MdxFile string
	// Only match the mdx with the same mdxFile name
	MdxCss   string
	MdxDict  Dict
	searcher Searcher
}

func (d *MdxDict) CSS() string {
	return d.MdxCss
}

func (d *MdxDict) Get(word string) []string {
	results := d.searcher.GetRawOutputs(strings.ToLower(word))
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
