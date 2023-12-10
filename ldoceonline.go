package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/fatih/color"
	"golang.org/x/net/html"
)

var separatorOpen = "{"
var separatorClose = "}"

func queryByURL(word string) string {
	start := time.Now()
	// queryURL := fmt.Sprintf("https://ldoceonline.com/dictionary/%s", word)
	queryURL := fmt.Sprintf("https://ldoceonline.com/search/english/direct/?q=%s", url.QueryEscape(word))
	// resp, err := http.Get(queryURL) // an unexpected EOF will occur
	// Refer to https://www.reddit.com/r/golang/comments/y971ye/unexpected_eof_from_http_request/ --> not working
	// https://bugz.pythonanywhere.com/golang/Unexpected-EOF-golang-http-client-error --> not working either
	// Maybe not my problem? It's work when I developed the first demo version. https://www.appsloveworld.com/go/2/golang-http-request-results-in-eof-errors-when-making-multiple-requests-successiv
	// I change my User-Agent to curl, it works then. 🥲
	client := &http.Client{}
	req, err := http.NewRequest(
		"GET",
		queryURL,
		http.NoBody,
	)
	req.Close = true
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Accept-Encoding", "identity") // NOTE THIS LINE
	req.Header.Set("User-Agent", "curl/8.1.2")

	resp, err := client.Do(req)

	log.Printf("query %q cost: %v", queryURL, time.Since(start))
	if err != nil {
		log.Printf("Get url %v err: %v", queryURL, err)
		return fmt.Sprintf("ERROR: %v", err)
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
			res = ldoceOnline(n)
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

func compressEmptyLine(s string) string {
	t := strings.Trim(s, " \n\u00a0")
	if len(t) == 0 {
		return " "
	}
	return s
}

// pureEmptyLine returns whether it's an empty line, only consisting of "IsSpace" Code
func pureEmptyLineLF(s string) bool {
	lf := false
	for _, c := range s {
		if c == '\n' || c == '\u0020' {
			lf = true
		}
		if !unicode.IsSpace(c) {
			return false
		}
	}
	return lf && true
}

// pureEmptyLineLF returns whether it's an empty line ended with '\n' or '\u00a0'
func pureEmptyLineEndLF(s string) bool {
	var last rune
	for _, c := range s {
		last = c
		if unicode.IsSpace(c) {
			continue
		}
		return false
	}
	return last == '\n' || last == '\u00a0'
}

// format removes consecutive CRLFs(the input lines are has been "compressed" in readText)
// TODO: make it elegant and robust.
func format(input []string) string {
	joined := strings.Join(input, "\n")
	var res string
	var prev rune
	for i, c := range joined {
		if i > 0 && c == '\n' && prev == '\n' {
			continue
		}
		res += string(c)
		prev = c
	}
	return res
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
		blue := color.New(color.FgBlue).SprintfFunc()
		head := fmt.Sprintf("%s", separate(readText(n)))
		head = strings.TrimLeft(head, " ")
		head = strings.ReplaceAll(head, "\n", " ")
		return []string{blue("%s", head)}
	}
	// read Sense for DEF
	if isElement(n, "span", "Sense") {
		red := color.New(color.FgRed).SprintfFunc()
		sense := fmt.Sprintf("%sSense%s", strings.Repeat("\t", 0), separate(readText(n)))
		sense = strings.TrimLeft(sense, " ")
		log.Printf("Sense: %q", sense)
		return []string{red("%s", sense)}
	}
	if isElement(n, "span", "PhrVbEntry") {
		pvb := fmt.Sprintf("%sPhrVbEntry:%s", "", separate(readText(n)))
		pvb = strings.TrimLeft(pvb, " ")
		log.Printf("PhrVbEntry: %q", pvb)
		return []string{pvb}
	}
	if isElement(n, "span", "Head") {
		cyan := color.New(color.FgCyan).SprintfFunc()
		head := fmt.Sprintf("%s", separate(readText(n)))
		head = strings.TrimLeft(head, " ")
		head = strings.ReplaceAll(head, "\n", " ")
		return []string{cyan("%s", fmt.Sprintf("%s", separate(head)))}
	}
	var res []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res = append(res, readLongmanEntry(c)...)
	}
	return res
}

func ldoceOnline(n *html.Node) []string {
	var res []string
	if isElement(n, "span", "ldoceEntry Entry") {
		res = append(res, fmt.Sprintf("\n*****LDOCE ENTRY*****\n"))
		res = append(res, readLongmanEntry(n)...)
		return res
	}

	if isElement(n, "span", "bussdictEntry Entry") {
		res = append(res, fmt.Sprintf("\n*****BUSS ENTRY*****\n"))
		res = append(res, readLongmanEntry(n)...)
		return res
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		res = append(res, ldoceOnline(c)...)
	}

	return res
}

func isElement(n *html.Node, ele string, class string) bool {
	if n.Type == html.ElementNode && (n.DataAtom.String() == ele || n.Data == ele) {
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

func readAllText(n *html.Node) string {
	var s string
	defer func() {
		log.Printf("alltext[%q]:", s)
	}()
	if n.Type == html.TextNode {
		return compressEmptyLine(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += readAllText(c)
	}
	return s
}

func readText(n *html.Node) string {
	if n.Type == html.TextNode {
		log.Printf("text: [%q]", n.Data)
		return compressEmptyLine(n.Data)
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
	if isElement(n, "span", "LEXUNIT") {
		noColor := color.New().SprintfFunc()
		return noColor("%s", fmt.Sprintf("\n%sLEXUNIT: %s \n", "", strings.TrimLeft(readSubs(n), " ")))
	}
	if isElement(n, "span", "DEF") {
		noColor := color.New().SprintfFunc()
		return noColor("%s", fmt.Sprintf("%sDEF: %s \n", "", strings.TrimLeft(readSubs(n), " ")))
	}

	if isElement(n, "span", "ColloExa") {
		noColor := color.New().SprintfFunc()
		return noColor("%s", fmt.Sprintf("%sColloExa: %s \n", "", separate(strings.TrimLeft(readSubs(n), " "))))
	}

	if isElement(n, "span", "F2NBox") {
		noColor := color.New().SprintfFunc()
		return noColor("%s", fmt.Sprintf("%sF2NBox: %s \n", "", separate(strings.TrimLeft(readSubs(n), " "))))
	}

	if isElement(n, "span", "heading span") {
		noColor := color.New().SprintfFunc()
		return noColor("%s", fmt.Sprintf("%sheading span:%s\n", "", separate(strings.TrimLeft(readSubs(n), " "))))
	}

	if isElement(n, "span", "GramExa") {
		noColor := color.New().SprintfFunc()
		return noColor("%s", fmt.Sprintf("%sGramExa:%s\n", "", separate(strings.TrimLeft(readSubs(n), " "))))
	}
	if isElement(n, "span", "EXAMPLE") {
		noColor := color.New().SprintfFunc()
		return noColor("%s", fmt.Sprintf("\n%sEXAMPLE:> %s <\n", strings.Repeat("\t", 2), strings.TrimLeft(readAllText(n), " ")))
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

func readSubs(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return compressEmptyLine(n.Data)
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += readText(c)
	}
	return s
}

func separate(s string) string {
	return fmt.Sprintf("%s%s%s", separatorOpen, s, separatorClose)
}
