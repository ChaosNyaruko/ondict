package render

import (
	"fmt"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// ------------------------------------------------------------------ entry://

// EntryHandler rewrites <a href="entry://word[#frag]"> to a local /dict URL.
// The fragment (if any) is preserved as a real URL hash after stripping the
// trailing __a suffix that dict anchor names carry but element IDs don't.
type EntryHandler struct{}

func (EntryHandler) HandleNode(n *html.Node, ctx RenderContext) bool {
	if !IsElement(n, "a", "") {
		return false
	}
	for i, a := range n.Attr {
		if a.Key != "href" || !strings.HasPrefix(a.Val, "entry://") {
			continue
		}
		target := strings.TrimPrefix(a.Val, "entry://")
		word, frag, hasFragment := strings.Cut(target, "#")
		var newHref string
		if hasFragment {
			frag = strings.TrimSuffix(frag, "__a")
			newHref = fmt.Sprintf("/dict?query=%s&engine=mdx&format=html#%s",
				url.QueryEscape(word), frag)
		} else {
			newHref = fmt.Sprintf("/dict?query=%s&engine=mdx&format=html",
				url.QueryEscape(word))
		}
		log.Infof("entry handler: %q → %q", target, newHref)
		n.Attr[i].Val = newHref
	}
	return false // recurse into children
}

// ------------------------------------------------------------------ sound://

// SoundHandler rewrites <a href="sound://file.mp3">.
// For online sources: replaces href with /file.mp3.
// For MDX sources: converts the <a> into a <div> with an embedded <audio>
// element and a click-to-play <script>.
type SoundHandler struct{}

func (SoundHandler) HandleNode(n *html.Node, ctx RenderContext) bool {
	if !IsElement(n, "a", "") {
		return false
	}
	for i, a := range n.Attr {
		if a.Key != "href" || !strings.HasPrefix(a.Val, "sound://") {
			continue
		}
		audioFile := strings.TrimPrefix(a.Val, "sound://")
		newPath := "/" + audioFile
		if strings.HasSuffix(ctx.SourceType, "Online") {
			n.Attr[i].Val = newPath
		} else {
			name := strings.TrimSuffix(audioFile, ".mp3")
			log.Infof("sound handler: %q → %q", name, newPath)
			convertAnchorToAudioDiv(n, newPath)
		}
	}
	return false // recurse: children <img> still need their src fixed
}

// convertAnchorToAudioDiv mutates n (an <a> element) into:
//
//	<div style="cursor: pointer">
//	  <audio src="/file.mp3" preload="none"></audio>
//	  <script>/* IIFE click handler */</script>
//	  [original children]
//	</div>
func convertAnchorToAudioDiv(n *html.Node, src string) {
	n.DataAtom = atom.Div
	n.Data = "div"
	// Keep only the cursor style; drop href and other anchor-specific attrs.
	n.Attr = []html.Attribute{{Key: "style", Val: "cursor: pointer"}}

	audio := newAudioTag(src)
	script := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Script,
		Data:     "script",
	}
	script.AppendChild(&html.Node{Type: html.TextNode, Data: jsTempl})

	// Prepend audio then script before any existing children (the icon <img> tags).
	n.InsertBefore(audio, n.FirstChild)
	n.InsertBefore(script, audio.NextSibling)
}

// ------------------------------------------------------------------ <img>

// ImgHandler normalises <img> src and promotes embedded base64 data URIs.
//
// Priority order:
//  1. base64 attribute present and is a data URI → use as src
//  2. src has file:// prefix → strip the scheme
//  3. src is relative (no leading / or http) → prefix with /
//  4. src already root-relative or http → leave unchanged
type ImgHandler struct{}

func (ImgHandler) HandleNode(n *html.Node, ctx RenderContext) bool {
	if !IsElement(n, "img", "") {
		return false
	}

	// Check for embedded base64 data URI in the custom "base64" attribute.
	for _, a := range n.Attr {
		if a.Key == "base64" && strings.HasPrefix(a.Val, "data:") {
			setAttr(n, "src", a.Val)
			return true // <img> has no meaningful children
		}
	}

	// Normalise the src attribute.
	for i, a := range n.Attr {
		if a.Key != "src" {
			continue
		}
		v := a.Val
		if strings.HasPrefix(v, "file://") {
			v = strings.TrimPrefix(v, "file://")
		}
		if !strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "http") {
			v = "/" + v
		}
		n.Attr[i].Val = v
	}
	return true // <img> is a void element
}

// ------------------------------------------------------------------ show-image cross-ref

// ShowImageHandler resolves <a class="ldoce-show-image" base64="key123jpg">
// cross-references that point to an illustration living in another entry.
//
// Resolution strategy:
//  1. Look for a sibling "see picture at WORD" link within the same .Sense span.
//     Fetch that entry (using ctx.EntryFetcher), find the first .big_pic img,
//     and rewrite the <a> href to that image path so the browser can open it.
//  2. Fall back to deriving a filename from the base64 key
//     (e.g. "ldoce4188jpg" → "ldoce4188.jpg") and using /filename directly.
//     If the entryFetcher is nil (non-HTML context), just leave the element
//     as-is.
//
// The <a> is converted to a button-style span so it no longer carries a
// javascript:void(0) href; the JS lightbox in dict.html opens the image on
// click via the data-img-src attribute.
type ShowImageHandler struct{}

func (ShowImageHandler) HandleNode(n *html.Node, ctx RenderContext) bool {
	if !IsElement(n, "a", "") {
		return false
	}
	if !hasClass(n, "ldoce-show-image") {
		return false
	}

	key := attrVal(n, "base64")
	if key == "" {
		return false
	}
	// data URI on an <a> is not expected — skip
	if strings.HasPrefix(key, "data:") {
		return false
	}

	// Strategy 1: find sibling "see picture at WORD" link in the same .Sense span.
	if ctx.EntryFetcher != nil {
		if src := resolveViaEntryFetcher(n, ctx.EntryFetcher); src != "" {
			rewriteShowImageAnchor(n, src)
			return false
		}
	}

	// Strategy 2: derive filename from the base64 key.
	filename := deriveFilename(key)
	rewriteShowImageAnchor(n, "/"+filename)
	return false
}

// resolveViaEntryFetcher finds the sibling "see picture at WORD" link in the
// same .Sense ancestor, fetches that entry, and returns the big_pic img src.
func resolveViaEntryFetcher(n *html.Node, fetcher func(string) string) string {
	sense := closestClass(n, "Sense")
	if sense == nil {
		return ""
	}
	// Find a link whose text contains "picture".
	var pictureLink *html.Node
	walkNodes(sense, func(node *html.Node) bool {
		if IsElement(node, "a", "") {
			href := attrVal(node, "href")
			isPictureHref := strings.Contains(href, "/dict") || strings.HasPrefix(href, "entry://")
			if isPictureHref && strings.Contains(textContent(node), "picture") {
				pictureLink = node
				return true // stop
			}
		}
		return false
	})
	if pictureLink == nil {
		return ""
	}

	// Extract the word from the href query param, strip fragment.
	href := attrVal(pictureLink, "href")
	word := wordFromDictHref(href)
	if word == "" {
		return ""
	}

	entryHTML := fetcher(word)
	if entryHTML == "" {
		return ""
	}

	return bigPicSrc(entryHTML)
}

// bigPicSrc parses rendered entry HTML and returns the src of the first
// <div class="big_pic"><img src="..."> found.
func bigPicSrc(entryHTML string) string {
	doc, err := html.ParseWithOptions(strings.NewReader(entryHTML),
		html.ParseOptionEnableScripting(true))
	if err != nil {
		return ""
	}
	var src string
	walkNodes(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode {
			cls := attrVal(n, "class")
			if strings.Contains(cls, "big_pic") {
				// find first <img> child
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if IsElement(c, "img", "") {
						src = attrVal(c, "src")
						return true // stop walk
					}
				}
			}
		}
		return false
	})
	return src
}

// rewriteShowImageAnchor converts the <a> into a <span> that carries the
// resolved image path in a data-img-src attribute. The JS lightbox picks this
// up on click without needing a javascript:void(0) href.
func rewriteShowImageAnchor(n *html.Node, src string) {
	// Remove href (no more dead javascript:void(0)).
	removeAttr(n, "href")
	// Store the resolved image src for the JS lightbox.
	setAttr(n, "data-img-src", src)
	setAttr(n, "style", "cursor:zoom-in")
}

// deriveFilename converts an MDD key like "ldoce4188jpg" → "ldoce4188.jpg"
// by inserting a dot before the last 3 characters (the extension).
func deriveFilename(key string) string {
	if len(key) > 3 {
		return key[:len(key)-3] + "." + key[len(key)-3:]
	}
	return key
}

// ------------------------------------------------------------------ helpers

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func removeAttr(n *html.Node, key string) {
	out := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key != key {
			out = append(out, a)
		}
	}
	n.Attr = out
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, cls string) bool {
	for _, c := range strings.Fields(attrVal(n, "class")) {
		if c == cls {
			return true
		}
	}
	return false
}

// closestClass walks up the DOM tree and returns the first ancestor (or self)
// that has the given CSS class.
func closestClass(n *html.Node, cls string) *html.Node {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur.Type == html.ElementNode && hasClass(cur, cls) {
			return cur
		}
	}
	return nil
}

// walkNodes does a depth-first pre-order walk. The visitor returns true to
// stop the walk early.
func walkNodes(n *html.Node, visit func(*html.Node) bool) bool {
	if n == nil {
		return false
	}
	if visit(n) {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if walkNodes(c, visit) {
			return true
		}
	}
	return false
}

// textContent returns the concatenated text content of a node and all descendants.
func textContent(n *html.Node) string {
	var b strings.Builder
	walkNodes(n, func(node *html.Node) bool {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		return false
	})
	return b.String()
}

// wordFromDictHref extracts the plain word from a /dict?query=WORD&... href
// or an entry://WORD#frag href, stripping any fragment.
func wordFromDictHref(href string) string {
	// Raw MDX href: entry://fruit#fruit__entry_0__a
	if strings.HasPrefix(href, "entry://") {
		target := strings.TrimPrefix(href, "entry://")
		word, _, _ := strings.Cut(target, "#")
		return word
	}
	// Already-rewritten href: /dict?query=fruit&engine=mdx&format=html#frag
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	q := u.Query().Get("query")
	word, _, _ := strings.Cut(q, "#")
	return word
}
