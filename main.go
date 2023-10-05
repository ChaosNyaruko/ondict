package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var help = flag.Bool("h", false, "show this help doc")
var word = flag.String("q", "", "specify the word that you want to query")
var easyMode = flag.Bool("e", false, "true to show only 'frequent' meaning")
var dev = flag.Bool("d", false, "if specified, a static html file will be parsed, instead of an online query")
var verbose = flag.Bool("v", false, "show debug logs")
var interactive = flag.Bool("i", false, "launch an interactive CLI app")
var server = flag.Bool("s", false, "serve as a HTTP server, for cache stuff, make it quicker!")
var remote = flag.String("c", "", "it can serve as a HTTP client, to get response from server")

var mu sync.Mutex // owns history
var history map[string]string = make(map[string]string)
var dataPath string
var historyFile string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	config := filepath.Join(home, ".config")
	dataPath = filepath.Join(config, "ondict")
	historyFile = filepath.Join(dataPath, "history.json")
	if dataPath == "" || historyFile == "" {
		log.Fatalf("empty datapath/historyfile: %v||%v", dataPath, historyFile)
	}
}

func main() {
	flag.Parse()
	if *help || flag.NFlag() == 0 || len(flag.Args()) > 0 {
		flag.PrintDefaults()
		return
	}
	if !*verbose && !*server {
		log.SetOutput(io.Discard)
	}

	if *interactive {
		startLoop()
		return
	}

	if *server {
		p := new(proxy)
		log.Fatal(http.ListenAndServe(":8999", p)) // TODO: use gin instead?
		return
	}

	if *remote == "auto" {
		res, err := http.Get(fmt.Sprintf("http://localhost:8999/?query=%s", url.QueryEscape(*word)))
		if err != nil {
			log.SetOutput(os.Stderr)
			log.Fatalf("new request error %v/%v", res, err)
		}
		defer res.Body.Close()
		if res, err := io.ReadAll(res.Body); err != nil {
			log.Printf("read body error %v", err)
		} else {
			fmt.Println(string(res))

		}
		return
	}

	// just for offline test.
	if *dev {
		fd, err := os.Open("./doctor_ldoce.html")
		if err != nil {
			log.Fatal(err)
		}
		defer fd.Close()
		fmt.Println(parseHTML(fd))
		return
	}
	fmt.Println(queryByURL(*word))
}

func query(word string) string {
	var res string
	mu.Lock()
	if ex, ok := history[word]; ok {
		log.Printf("cache hit!")
		res = ex
	} else {
		res = queryByURL(word)
		history[word] = res
	}
	mu.Unlock() // TODO: performance
	return res
}

func queryByURL(word string) string {
	start := time.Now()
	// url := fmt.Sprintf("https://ldoceonline.com/dictionary/%s", word)
	url := fmt.Sprintf("https://ldoceonline.com/search/english/direct/?q=%s", url.QueryEscape(word))
	resp, err := http.Get(url)
	log.Printf("query %q cost: %v", url, time.Since(start))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	return parseHTML(resp.Body)
}

func parseHTML(info io.Reader) string {
	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	// Type      NodeType
	// DataAtom  atom.Atom
	// Data      string
	// Namespace string
	// Attr      []Attribute
	var res []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		// log.Printf("Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
		if isElement(n, "div", "dictionary") {
			res = ldoceDict(n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	// log.Printf("result: %v", readText(doc))
	f(doc)
	return format(res)
}

func pureEmpty(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\n' || c == '\t' || c == '\u00a0' {
			continue
		}
		return false
	}
	return true
}

func format(input []string) string {
	// TODO: remove consecutive CRLFs or "empty lines"?
	return strings.Join(input, "\n")
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

func readLongmanEntry(n *html.Node) []string {
	// read "frequent head" for PRON
	if isElement(n, "span", "frequent Head") {
		return []string{fmt.Sprintf("frequent HEAD %s", readText(n))}
	}
	// read Sense for DEF
	if isElement(n, "span", "Sense") {
		return []string{fmt.Sprintf("Sense(%v):%s", getSpanID(n), readText(n))}
	}
	if isElement(n, "span", "Head") {
		return []string{fmt.Sprintf("HEAD %s", readText(n))}
	}
	var res []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res = append(res, readLongmanEntry(c)...)
	}
	return res
}

func ldoceDict(n *html.Node) []string {
	var res []string
	if isElement(n, "span", "ldoceEntry Entry") {
		res = append(res, fmt.Sprintf("==find an ldoce entry=="))
		res = append(res, readLongmanEntry(n)...)
		return res
	}

	if !*easyMode && isElement(n, "span", "bussdictEntry Entry") {
		res = append(res, fmt.Sprintf("==find a buss entry=="))
		res = append(res, readLongmanEntry(n)...)
		return readLongmanEntry(n)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res = append(res, ldoceDict(c)...)
	}

	return res
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

func readOneExample(n *html.Node) string {
	var s string
	defer func() {
		log.Printf("example[%q]:", s)
	}()
	if n.Type == html.TextNode {
		return n.Data
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += readText(c)
	}
	return s
}

func readText(n *html.Node) string {
	if n.Type == html.TextNode {
		log.Printf("text: [%q]", n.Data)
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
		return fmt.Sprintf("\n%sEXAMPLE:%s", strings.Repeat(" ", 0), readOneExample(n))
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += readText(c)
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

func Restore() {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		log.Printf("open file history err: %v", err)
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &history)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("history: %v", history)
}

func Store() {
	his, err := json.Marshal(history)
	if err != nil {
		log.Fatal("marshal err ", err)
	}
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		log.Fatal("make dir err", err)
	}
	f, err := os.Create(historyFile)
	if err != nil {
		log.Fatal("create file err", err)
	}

	defer f.Close()

	_, err = f.Write(his)

	if err != nil {
		log.Fatal("write file err", err)
	}
}
