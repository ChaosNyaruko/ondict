package render

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

type Renderer interface {
	Render() string
}

const (
	Longman5Online = "LONGMAN5/Online"
	LongmanEasy    = "LONGMAN/Easy"
	OLD9           = "OLD9"
)

type HTMLRender struct {
	Raw        string
	SourceType string
}

func (h *HTMLRender) Render() string {
	if !strings.HasPrefix(h.SourceType, "LONGMAN") {
		return h.Raw
	}
	info := strings.NewReader(h.Raw)
	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	dfs(doc, 0, nil, "")
	var b bytes.Buffer
	err = html.Render(&b, doc)
	if err != nil {
		log.Debugf("html.Render err: %v", err)
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
	// log.Debugf("modifyImgSrc %#v", n)
}

func replaceMp3(n *html.Node, val string, i int) {
	new := fmt.Sprintf("/%s", url.QueryEscape(strings.TrimPrefix(val, "sound://")))
	log.Infof("href sound: %v, new: %q", strings.TrimPrefix(val, "sound://"), new)
	n.Attr[i].Val = new
}

func modifyHref(n *html.Node) {
	for i, a := range n.Attr {
		if a.Key == "href" {
			if strings.HasPrefix(a.Val, "entry://") {
				new := fmt.Sprintf("/dict?query=%s&engine=mdx&format=html", url.QueryEscape(strings.TrimPrefix(a.Val, "entry://")))
				log.Infof("href entry: %v, new: %q", strings.TrimPrefix(a.Val, "entry://"), new)
				n.Attr[i].Val = new
			} else if strings.HasPrefix(a.Val, "sound://") {
				replaceMp3(n, a.Val, i)
			}
		}
	}
}

func dfs(n *html.Node, level int, parent *html.Node, ft string) string {
	if n.Type == html.TextNode {
		return ""
	}
	if IsElement(n, "a", "") {
		log.Debugf("<a> %v", n)
		modifyHref(n)
		return ""
	}
	if IsElement(n, "img", "") {
		// modifyImgSrc(n)
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
				log.Debugf("[wft] readElement good %v, %v, %#v", ele, class, n.Data)
				return true
			}
		}
	}
	return false
}
