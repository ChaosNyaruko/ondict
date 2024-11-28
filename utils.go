package main

import (
	"bytes"
	"log"
	"os"

	"golang.org/x/net/html"
)

func dfs(n *html.Node) {
	if n.Type == html.TextNode {
		log.Printf("TextNode: %v/%d, Parent: %v", n.Data, n.DataAtom, n.Parent.DataAtom)
		return
	}
	log.Printf("Type: %#v, DataAtom: %v, Attr: %#v", n.Type, n.DataAtom, n.Attr)

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		dfs(c)
	}
	return
}

func DumpHTMLDoc(filename string) {
	hdoc, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	info := bytes.NewReader(hdoc)
	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	dfs(doc)
}
