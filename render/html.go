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
	if !strings.HasPrefix(h.SourceType, "LONGMAN") {
		return h.Raw
	}
	info := strings.NewReader(h.Raw)
	doc, err := html.ParseWithOptions(info, html.ParseOptionEnableScripting(false))
	if err != nil {
		log.Fatal(err)
	}
	h.dfs(doc, 0, nil, "")
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
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "-", "_")
	divID := "__div__" + name
	audioID := "__audio__" + name
	n.Attr = append(n.Attr, []html.Attribute{
		{Key: "id", Val: divID},
		// {Key: "class", Val: "__clickable__"},
		{Key: "style", Val: "cursor: pointer"},
	}...)
	node := newAudioTag(name, new)
	playIconVar := fmt.Sprintf("playIcon_%s", name)
	audioPlayerVar := fmt.Sprintf("audioPlayer_%s", name)
	jsChild := html.Node{
		Parent:      nil,
		FirstChild:  nil,
		LastChild:   nil,
		PrevSibling: nil,
		NextSibling: nil,
		Type:        html.TextNode,
		DataAtom:    0,
		Data:        fmt.Sprintf(jsTempl, playIconVar, divID, audioPlayerVar, audioID, playIconVar, audioPlayerVar),
		Namespace:   "",
		Attr:        nil,
	}
	jsNode := html.Node{
		Parent:      nil,
		FirstChild:  nil,
		LastChild:   nil,
		PrevSibling: nil,
		NextSibling: nil,
		Type:        html.ElementNode,
		DataAtom:    atom.Script,
		Data:        "script",
		Namespace:   "",
		Attr:        []html.Attribute{},
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

func newAudioTag(name, src string) *html.Node {
	res := html.Node{
		Parent:      nil,
		FirstChild:  nil,
		LastChild:   nil,
		PrevSibling: nil,
		NextSibling: nil,
		Type:        html.ElementNode,
		DataAtom:    atom.Audio,
		Data:        "audio",
		Namespace:   "",
		Attr: []html.Attribute{
			{Key: "id", Val: `__audio__` + name},
			{Key: "src", Val: src},
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
				new := fmt.Sprintf("/%s", strings.TrimPrefix(a.Val, "sound://"))
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
		log.Infof("TextNode: %v, DataAtom:%v", n.Type, n.DataAtom)
		return ""
	}
	if IsElement(n, "a", "") {
		log.Debugf("<a> %v", n)
		h.modifyHref(n)
		return ""
	}
	if IsElement(n, "img", "") {
		// modifyImgSrc(n)
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

const jsTempl = `
   let %s = document.getElementById('%s');
   let %s = document.getElementById('%s');

    %s.addEventListener('click', () => {
        %s.play().catch(error => {
            console.error('Error playing audio:', error);
        });
    });
`
