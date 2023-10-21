package main

import (
	"log"
	"os"
	"testing"

	"golang.org/x/net/html"
)

func Test_GetWords(t *testing.T) {
	var dfs func(*html.Node, int, *html.Node)
	words := make([]string, 0, 10000)
	dfs = func(n *html.Node, level int, parent *html.Node) {
		// t.Logf("LEVEL[%d %p <- %p] Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", level, n, parent, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
		if level == 0 {
			// t.Logf("ROOT: type: %v atom[%s]", n.Type, n.DataAtom)
		}
		if n.Type == html.TextNode && n.Parent != nil && isElement(n.Parent, "body", "") {
			t.Logf("LEVEL[%d %p <- %p] Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", level, n, parent, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
			// words = append(words, n.Data)
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			dfs(c, level+1, n)
		}
	}
	fd, err := os.Open("./tmp/ldoce5.html")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()
	doc, err := html.ParseWithOptions(fd, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	// log.Printf("result: %v", readText(doc))
	dfs(doc, 0, nil)
	t.Logf("len: %d, %v", len(words), words)
}

func Test_MDXParser(t *testing.T) {
	// fd, err := os.Open("./tmp/test.html")
	fd, err := os.Open("./tmp/doctor_mdx.html")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()
	doc, err := html.ParseWithOptions(fd, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	// log.Printf("result: %v", readText(doc))
	t.Logf("res: %v", format([]string{f(doc, 0, nil)}))
}
