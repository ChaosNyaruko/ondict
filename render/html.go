package render

import (
	"bytes"
	"log"
	"strings"

	"golang.org/x/net/html"
)

type Renderer interface {
	Render() string
}

type HTMLRender struct {
	Raw string
}

func (h *HTMLRender) Render() string {
	return h.Raw
	info := strings.NewReader(h.Raw)
	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	dfs(doc, 0, nil, "")
	var b bytes.Buffer
	err = html.Render(&b, doc)
	if err != nil {
		log.Printf("html.Render err: %v", err)
		return h.Raw
	}
	return b.String()
}

func modifyImgSrc(n *html.Node) {
	if n.Type != html.ElementNode || (n.DataAtom.String() != "img" && n.Data != "img") {
		log.Fatalf("Error: an img element is expected")
	}
	for i, a := range n.Attr {
		if a.Key == "src" {
			n.Attr[i].Val = "tmp/" + a.Val
		}
	}
	// log.Printf("modifyImgSrc %#v", n)
}

func dfs(n *html.Node, level int, parent *html.Node, ft string) string {
	if n.Type == html.TextNode {
		// t.Logf("text: [%s] level %d", n.Data, level)
		return ""
	}
	if IsElement(n, "img", "") {
		modifyImgSrc(n)
		return ""
	}

	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += dfs(c, level+1, n, ft)
	}
	return s
}

func IsElement(n *html.Node, ele string, class string) bool {
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
