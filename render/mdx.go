package render

import (
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

func ParseMDX(info io.Reader, ft string) string {
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
	// log.Debugf("LEVEL[%d %p <- %p] Type: [%#v], DataAtom: [%s], Data: [%#v], Namespace: [%#v], Attr: [%#v]", level, n, parent, n.Type, n.DataAtom, n.Data, n.Namespace, n.Attr)
	if n.Type == html.TextNode {
		// t.Logf("text: [%s] level %d", n.Data, level)
		return compressEmptyLine(n.Data)
	}
	if IsElement(n, "div", "") {
		// return "\n" + readS(n) + "\n"
		return "\n"
	}
	// italic
	if IsElement(n, "i", "") {
		return renderMD(readS(n, ft), italic)
	}
	// bold
	if IsElement(n, "b", "") {
		return renderMD(readS(n, ft), bold)
	}
	if IsElement(n, "table", "") {
		return "\n" + readS(n, ft) + "\n"
	}
	if IsElement(n, "ex", "") {
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
