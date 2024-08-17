package render_test

import (
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"

	"github.com/ChaosNyaruko/ondict/render"
	"github.com/ChaosNyaruko/ondict/sources"
)

func Test_GetWords(t *testing.T) {
	var dfs func(*html.Node, int, *html.Node)
	words := make([]string, 0, 10000)
	dfs = func(n *html.Node, level int, parent *html.Node) {
		// t.Logf("LEVEL[%d %p <- %p] Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", level, n, parent, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
		if level == 0 {
			// t.Logf("ROOT: type: %v atom[%s]", n.Type, n.DataAtom)
		}
		if n.Type == html.TextNode && n.Parent != nil && render.IsElement(n.Parent, "body", "") {
			t.Logf("LEVEL[%d %p <- %p] Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", level, n, parent, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
			// words = append(words, n.Data)
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			dfs(c, level+1, n)
		}
	}
	fd, err := os.Open("../testdata/ldoce5.html")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()
	doc, err := html.ParseWithOptions(fd, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	// log.Debugf("result: %v", readText(doc))
	dfs(doc, 0, nil)
	t.Logf("len: %d, %v", len(words), words)
}

func Test_MDXParser(t *testing.T) {
	// fd, err := os.Open("../testdata/test.html")
	fd, err := os.Open("../testdata/doctor_mdx.html")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()
	// log.Debugf("result: %v", readText(doc))
	t.Logf("res: %v", render.ParseHTML(fd))
}

func Test_MultiMatch(t *testing.T) {
	d := sources.MdxDict{
		MdxFile: "../testdata/test_dict",
	}
	d.Register()
	assert.Equal(t, 1, len(d.Get("doctor")), "doctor")
	assert.Equal(t, 1, len(d.Get("jesus")), "jesus")
	assert.Equal(t, 1, len(d.Get("Doctor")), "Doctor")
	assert.Equal(t, 1, len(d.Get("Jesus")), "Jesus")
	assert.Equal(t, 2, len(d.Get("August")), "August")
	assert.Equal(t, 2, len(d.Get("august")), "august")
	t.Logf("%v", d.Get("from a to b"))
	assert.Equal(t, 0, len(d.Get("b")), "b")
}

func Test_play(t *testing.T) {
	var g *sources.MdxDict
	if os.Getenv("FULLTEST") == "1" {
		sources.LoadConfig()
		g = (*sources.G)[0]
	} else {
		d := sources.MdxDict{
			MdxFile: "../testdata/test_dict",
		}
		g = &d
	}
	g.Register()
	dict := g.MdxDict
	input := make([]string, 0, len(dict.Keys()))
	// lowercase
	lowDict := make(map[string][]string, len(dict.Keys()))
	for _, k := range dict.Keys() {
		lk := strings.ToLower(k)
		lowDict[lk] = append(lowDict[lk], k)
	}

	for k, _ := range lowDict {
		input = append(input, k)
	}
	t.Logf("raw dict %d items, "+
		"lowercase dict %d items, "+
		"because different item in the raw dictionary "+
		"like 'August' and 'august' will be "+
		"combined into a string slice\n",
		len(dict.Keys()), len(lowDict))

	word := "want"
	t.Logf("%q output: %v", word, lowDict[word])
}

func TestMain(m *testing.M) {
	// log.SetOutput(io.Discard)
	m.Run()
}
