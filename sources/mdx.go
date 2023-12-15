package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/net/html"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/render"
)

var Gbold = "**"
var Gitalic = "*"

var GlobalDict MdxDict

func QueryMDX(word string, f string) string {
	defs := GlobalDict.Get(word)
	log.Printf("def of %v: %q", defs, word)
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
		res += "\n---\n" + parseMDX(fd, f)
	}
	return res
}

func parseMDX(info io.Reader, ft string) string {
	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	return format([]string{f(doc, 0, nil, ft)})
}

// f -> readText
func f(n *html.Node, level int, parent *html.Node, ft string) string {
	var bold, italic string
	if ft == "md" {
		bold, italic = "**", "*"
	}
	// log.Printf("LEVEL[%d %p <- %p] Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", level, n, parent, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
	if n.Type == html.TextNode {
		// t.Logf("text: [%s] level %d", n.Data, level)
		return compressEmptyLine(n.Data)
	}
	if isElement(n, "div", "") {
		// return "\n" + readS(n) + "\n"
		return "\n"
	}
	// italic
	if isElement(n, "i", "") {
		return renderMD(readS(n, ft), italic)
	}
	// bold
	if isElement(n, "b", "") {
		return renderMD(readS(n, ft), bold)
	}
	if isElement(n, "table", "") {
		return "\n" + readS(n, ft) + "\n"
	}
	if isElement(n, "ex", "") {
		return fmt.Sprintf("> %s <", readS(n, ft))
	}
	if n.Type == html.ElementNode && n.DataAtom.String() == "br" {
		return "\n"
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += f(c, level+1, n, ft)
	}
	return s
}

func readS(n *html.Node, ft string) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return compressEmptyLine(n.Data)
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += f(c, 0, n, ft)
	}
	return s
}

func renderMD(s string, id string) string {
	return id + s + id
}

func loadDecodedMdx(filePath string) Dict {
	jsonData, err := os.ReadFile(filePath + ".json")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("Failed to read JSON file: %v, %v", filePath, err)
	} else if errors.Is(err, os.ErrNotExist) {
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
	mdxFile string
	// Only match the mdx with the same mdxFile name
	mdxCss   string
	mdxDict  Dict
	searcher Searcher
}

func (d *MdxDict) CSS() string {
	return d.mdxCss
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
