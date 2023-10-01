package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var word = flag.String("q", "", "specify the word that you want to query")
var easyMode = flag.Bool("e", false, "true to show only 'frequent' meaning")
var dev = flag.Bool("d", false, "if specified, a static html file will be parsed, instead of an online query")

func main() {
	var info io.Reader
	flag.Parse()
	fmt.Println(flag.Args())
	if len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}
	if !*dev {
		log.SetOutput(io.Discard)
		start := time.Now()
		url := fmt.Sprintf("https://ldoceonline.com/dictionary/%s", *word)
		resp, err := http.Get(url)
		log.Printf("query %v cost: %v", url, time.Since(start))
		if err != nil {
			log.Fatal(err)
		}
		info = resp.Body
		defer resp.Body.Close()
	} else {
		fd, err := os.Open("./doctor_ldoce.html")
		if err != nil {
			log.Fatal(err)
		}
		info = fd
		defer fd.Close()
	}

	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
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
		// log.Printf("Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
		if isElement(n, "div", "dictionary") {
			ldoceDict(n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	// log.Printf("result: %v", readText(doc))
	f(doc)
}

func findFirstSubSpan(n *html.Node, class string) *html.Node {
	log.Printf("find class: %q, Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", class, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
	if isElement(n, "span", class) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := findFirstSubSpan(c, class); res != nil {
			return res
		}
	}
	return nil
}

func readLongmanEntry(n *html.Node) {
	// read "frequent head" for PRON
	if isElement(n, "span", "frequent Head") {
		fmt.Printf("frequent HEAD %s\n", readText(n, 0))
		return
	}
	// read Sense for DEF
	if isElement(n, "span", "Sense") {
		fmt.Printf("Sense(%v):%s\n", getSpanID(n), readText(n, 0))
		return
	}
	if isElement(n, "span", "Head") {
		fmt.Printf("\nHEAD %s\n", readText(n, 0))
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		readLongmanEntry(c)
	}
}

func ldoceDict(n *html.Node) bool {
	// log.Printf("Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
	// if isElement(n, "span", "dictionary_intro span") {
	// 	dictName := readText(n, 0)
	// 	fmt.Printf("dictionary_intro: %v\n", dictName)
	// }
	if isElement(n, "span", "ldoceEntry Entry") {
		fmt.Printf("==find an ldoce entry==\n")
		readLongmanEntry(n)
		return true
	}

	if isElement(n, "span", "bussdictEntry Entry") {
		fmt.Printf("==find an bussdict entry==\n")
		readLongmanEntry(n)
		return true
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		ldoceDict(c)
	}

	return false
}

func isElement(n *html.Node, ele string, class string) bool {
	if n.Type == html.ElementNode && n.DataAtom.String() == ele {
		if class == "" {
			return true
		}
		for _, a := range n.Attr {
			if a.Key == "class" && a.Val == class {
				log.Printf("[wft] readElement good %v, %v, %#v", ele, class, n.Data)
				return true
			}
		}
	}
	return false
}

// TODO: indent for format
func readOneExample(n *html.Node, eID int) string {
	var s string
	defer func() {
		log.Printf("example[%d/%q]:", eID, s)
	}()
	if n.Type == html.TextNode {
		return n.Data
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += readText(c, eID)
	}
	return s
}

// TODO: indent for format
func readText(n *html.Node, eID int) string {
	if n.Type == html.TextNode {
		log.Printf("text: [%d/%q]", eID, n.Data)
		return n.Data
	}
	if isElement(n, "script", "") {
		return ""
	}
	if getSpanClass(n) == "HWD" {
		return ""
	}
	if getSpanClass(n) == "FIELD" {
		return ""
	}
	if getSpanClass(n) == "ACTIV" {
		return ""
	}
	if isElement(n, "span", "EXAMPLE") {
		eID += 1
		return fmt.Sprintf("\n%sEXAMPLE%d:%s", strings.Repeat(" ", 0), eID, readOneExample(n, eID))
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += readText(c, eID)
	}
	return s
}

func getSpanID(n *html.Node) string {
	if n.Type == html.ElementNode && n.DataAtom.String() == "span" {
		for _, a := range n.Attr {
			if a.Key == "id" {
				return a.Val
			}
		}
	}
	return ""
}

func getSpanClass(n *html.Node) string {
	if n.Type == html.ElementNode && n.DataAtom.String() == "span" {
		for _, a := range n.Attr {
			if a.Key == "class" {
				return a.Val
			}
		}
	}
	return ""
}
