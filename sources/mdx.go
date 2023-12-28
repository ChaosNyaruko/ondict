package sources

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/render"
)

var Gbold = "**"
var Gitalic = "*"

type Dicts []*MdxDict

var G = &Dicts{}

func (g *Dicts) Load() error {
	for _, d := range *g {
		d.Register()
	}
	log.Printf("loading g")
	return nil
}

func QueryMDX(word string, f string) string {
	var defs []string
	for _, dict := range *G {
		defs = append(defs, dict.Get(word)...)
		log.Printf("def of %q, %v: %q", dict.MdxFile, defs, word)
	}
	// TODO: put the render abstraction here?
	if f == "html" { // f for format
		var res []string
		for _, def := range defs {
			h := render.HTMLRender{Raw: def}
			// m1 := regexp.MustCompile(`<img src="(.*?)\.png" style`)
			// replaceImg := m1.ReplaceAllString(def, `<img src="`+"data/"+`${1}.png" style`)
			// log.Printf("try to replace %v", replaceImg)
			res = append(res, h.Render())
		}
		return strings.Join(res, "<br><br>")
	}

	var res string
	for _, def := range defs {
		fd := strings.NewReader(def) // TODO: find a "close" one when missing?
		res += "\n---\n" + render.ParseMDX(fd, f)
	}
	return res
}

func loadDecodedMdx(filePath string) Dict {
	jsonData, err := os.ReadFile(filePath + ".json")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("Failed to read JSON file: %v, %v", filePath, err)
	} else if errors.Is(err, os.ErrNotExist) {
		log.Printf("JSON file not exist: %v", filePath+".json")
		m := &decoder.MDict{}
		err := m.Decode(filePath + ".mdx")
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
