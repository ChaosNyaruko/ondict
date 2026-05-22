package render

import (
	"bytes"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Renderer is the common interface for all rendering backends.
type Renderer interface {
	Render() string
}

// Source type constants shared across HTML and Markdown renderers.
const (
	Longman5Online = "LONGMAN5/Online"
	LongmanEasy    = "LONGMAN/Easy"
	OLD9           = "OLD9"
)

// defaultHandlers is the ordered list of NodeHandlers applied during every
// HTMLRender.Render() walk. Add new handlers here to support new URL schemes
// or element transformations without touching the walk logic.
var defaultHandlers = []NodeHandler{
	EntryHandler{},
	SoundHandler{},
	ImgHandler{},
	ShowImageHandler{},
}

// HTMLRender renders raw MDX/online HTML into clean browser-ready HTML by
// applying all registered NodeHandlers in a single DOM walk.
type HTMLRender struct {
	Raw        string
	SourceType string
	// LinkFormat is passed to RenderContext.LinkFormat to control the format=
	// parameter in rewritten entry:// links. Defaults to "html" when empty.
	LinkFormat string
	// EntryFetcher, when set, allows handlers to fetch other entries at
	// render time (e.g. ShowImageHandler resolving big_pic cross-refs).
	EntryFetcher func(word string) string
}

func (h *HTMLRender) Render() string {
	doc, err := html.ParseWithOptions(
		bytes.NewReader([]byte(h.Raw)),
		html.ParseOptionEnableScripting(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := RenderContext{
		SourceType:   h.SourceType,
		LinkFormat:   h.LinkFormat,
		EntryFetcher: h.EntryFetcher,
	}
	walk(doc, ctx)

	body := findElement(doc, atom.Body, "body")
	if body == nil {
		body = doc
	}
	rendered, err := renderChildren(body)
	if err != nil {
		log.Debugf("html.Render err: %v", err)
		return h.Raw
	}

	// Re-emit any <script src="..."> tags that the HTML parser moved into
	// <head> (they come from the raw dict preamble, e.g. jquery + LM5Switch.js).
	// Without this, dict-provided JS (fold/unfold, popup menus) never loads.
	headScripts := headScriptTags(doc)
	if headScripts != "" {
		rendered = headScripts + rendered
	}

	return rendered
}

// walk performs a depth-first pre-order traversal of the DOM, applying all
// defaultHandlers to each element node. A handler may signal skipChildren=true
// to prevent recursion into that node's children (e.g. <img> is a void
// element). Otherwise the walk always recurses.
func walk(n *html.Node, ctx RenderContext) {
	if n == nil || n.Type == html.TextNode {
		return
	}

	skipChildren := false
	if n.Type == html.ElementNode {
		for _, h := range defaultHandlers {
			if h.HandleNode(n, ctx) {
				skipChildren = true
				// Don't break: multiple handlers may legitimately act on the
				// same node (e.g. a future handler might add aria attributes
				// after another rewrote the href).
			}
		}
	}

	if !skipChildren {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, ctx)
		}
	}
}

// headScriptTags collects all <script src="..."> nodes that the HTML parser
// moved into <head> (from the raw dict preamble) and re-serialises them as a
// string so they can be prepended to the body output. Inline <script> blocks
// are intentionally skipped — only external src= scripts are re-emitted.
func headScriptTags(doc *html.Node) string {
	head := findElement(doc, atom.Head, "head")
	if head == nil {
		return ""
	}
	var b bytes.Buffer
	for c := head.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.DataAtom != atom.Script {
			continue
		}
		hasSrc := false
		for _, a := range c.Attr {
			if a.Key == "src" && a.Val != "" {
				hasSrc = true
				break
			}
		}
		if !hasSrc {
			continue
		}
		if err := html.Render(&b, c); err == nil {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ── DOM helpers used by html.go and handlers.go ──────────────────────────────

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

// IsElement reports whether n is an element with the given tag name, and
// optionally an exact class match when class != "".
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

