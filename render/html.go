package render

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
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
	info := strings.NewReader(h.Raw)
	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	h.dfs(doc, 0, nil, "")
	body := findElement(doc, atom.Body, "body")
	if body == nil {
		body = doc
	}
	rendered, err := renderChildren(body)
	if err != nil {
		log.Debugf("html.Render err: %v", err)
		return h.Raw
	}
	return rendered
}

func renderChildren(n *html.Node) (string, error) {
	var b bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&b, c); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func findElement(n *html.Node, atomName atom.Atom, data string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && (n.DataAtom == atomName || n.Data == data) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, atomName, data); found != nil {
			return found
		}
	}
	return nil
}

func modifyImgSrc(n *html.Node) {
	if n.Type != html.ElementNode || (n.DataAtom.String() != "img" && n.Data != "img") {
		log.Fatalf("Error: an img element is expected")
	}
	for i, a := range n.Attr {
		if a.Key == "src" && !strings.HasPrefix(a.Val, "/") && !strings.HasPrefix(a.Val, "http") {
			n.Attr[i].Val = "/" + a.Val
		}
	}
}

func (h *HTMLRender) replaceMp3(n *html.Node, val string, name, new string) {
	if false {
		var b bytes.Buffer
		err := html.Render(&b, n)
		if err != nil {
			panic(err)
		}
		file, err := os.OpenFile("origin-test-audio-"+strings.TrimPrefix(val, "sound://")+".html", os.O_WRONLY|os.O_CREATE, 0o666)
		if err != nil {
			panic(err)
		}
		file.Write(b.Bytes())
		file.Close()
	}
	log.Infof("href sound: %v, new: %q", strings.TrimPrefix(val, "sound://"), new)
	n.DataAtom = atom.Div
	n.Data = "div"
	n.Attr = append(n.Attr, []html.Attribute{
		{Key: "style", Val: "cursor: pointer"},
	}...)
	node := newAudioTag(new)
	jsChild := html.Node{
		Type: html.TextNode,
		Data: jsTempl,
	}
	jsNode := html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Script,
		Data:     "script",
	}
	jsNode.InsertBefore(&jsChild, nil)
	n.InsertBefore(node, nil)
	n.InsertBefore(&jsNode, nil)
	if false {
		var b bytes.Buffer
		err := html.Render(&b, n)
		if err != nil {
			panic(err)
		}
		file, err := os.OpenFile("test-audio-"+strings.TrimPrefix(val, "sound://")+".html", os.O_WRONLY|os.O_CREATE, 0o666)
		if err != nil {
			panic(err)
		}
		file.Write(b.Bytes())
		file.Close()
	}
}

func newAudioTag(src string) *html.Node {
	res := html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Audio,
		Data:     "audio",
		Attr: []html.Attribute{
			{Key: "src", Val: src},
			{Key: "preload", Val: "none"},
		},
	}
	return &res
}

func (h *HTMLRender) modifyHref(n *html.Node) {
	for i, a := range n.Attr {
		if a.Key == "href" {
			if strings.HasPrefix(a.Val, "entry://") {
				new := fmt.Sprintf("/dict?query=%s&engine=mdx&format=html", url.QueryEscape(strings.TrimPrefix(a.Val, "entry://")))
				log.Infof("href entry: %v, new: %q", strings.TrimPrefix(a.Val, "entry://"), new)
				n.Attr[i].Val = new
			} else if strings.HasPrefix(a.Val, "sound://") {
				name := strings.TrimSuffix(strings.TrimPrefix(a.Val, "sound://"), ".mp3")
				audioFile := strings.TrimPrefix(a.Val, "sound://")
				new := fmt.Sprintf("/%s", audioFile)
				if strings.HasSuffix(h.SourceType, "Online") {
					n.Attr[i].Val = new
				} else {
					h.replaceMp3(n, a.Val, name, new)
				}
			}
		}
	}
}

func (h *HTMLRender) dfs(n *html.Node, level int, parent *html.Node, ft string) string {
	if n.Type == html.TextNode {
		log.Debugf("TextNode: %v, DataAtom:%v", n.Type, n.DataAtom)
		return ""
	}
	if IsElement(n, "a", "") {
		log.Debugf("<a> %v", n)
		h.modifyHref(n)
		// Fall through to recurse into children: <a href="sound://..."><img src="..."></a>
		// is common in MDX entries. After replaceMp3 converts the <a> to a <div>, the
		// original <img> children remain and their src attributes still need rewriting.
	}
	if IsElement(n, "img", "") {
		modifyImgSrc(n)
		return ""
	}

	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += h.dfs(c, level+1, n, ft)
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

// jsTempl uses an IIFE (Immediately Invoked Function Expression) to scope
// variables, avoiding "redeclaration of let" errors when multiple dictionaries
// generate scripts for the same word on one page.
const jsTempl = `
(() => {
    let container = document.currentScript.parentElement;
    let audio = container.querySelector('audio');
    container.addEventListener('click', () => {
        audio.play().catch(error => {
            console.error('Error playing audio:', error);
        });
    });
})();
`
