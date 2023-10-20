package main

import (
	"log"
	"os"
	"testing"

	"golang.org/x/net/html"
)

func Test_MDXParser(t *testing.T) {
	fd, err := os.Open("./tmp/doctor_mdx.html")
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()
	doc, err := html.ParseWithOptions(fd, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	// Type      NodeType
	// DataAtom  atom.Atom
	// Data      string
	// Namespace string
	// Attr      []Attribute
	var s string
	var f func(*html.Node, int) string
	f = func(n *html.Node, level int) string {
		// t.Logf("LEVEL: %v Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", level, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
		if n.Type == html.TextNode {
			return n.Data
		}
		if n.Type == html.ElementNode && n.DataAtom.String() == "br" {
			return "\n"
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			s += f(c, level+1)
		}
		return s
	}
	// log.Printf("result: %v", readText(doc))
	t.Logf("res: %v", f(doc, 0))
}
