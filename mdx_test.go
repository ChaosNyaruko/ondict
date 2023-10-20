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
	var f func(*html.Node)
	f = func(n *html.Node) {
		t.Logf("Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	// log.Printf("result: %v", readText(doc))
	f(doc)
}
