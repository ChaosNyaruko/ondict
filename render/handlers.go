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
// The format query parameter is taken from ctx.LinkFormat (defaults to "html").
// The fragment (if any) is preserved as a real URL hash after stripping the
// trailing __a suffix that dict anchor names carry but element IDs don't.
type EntryHandler struct{}

func (EntryHandler) HandleNode(n *html.Node, ctx RenderContext) bool {
	if !IsElement(n, "a", "") {
		return false
	}
	format := ctx.LinkFormat
	if format == "" {
		format = "html"
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
			newHref = fmt.Sprintf("/dict?query=%s&engine=mdx&format=%s#%s",
				url.QueryEscape(word), format, frag)
		} else {
			newHref = fmt.Sprintf("/dict?query=%s&engine=mdx&format=%s",
				url.QueryEscape(word), format)
		}
		log.Infof("entry handler: %q → %q", target, newHref)
		n.Attr[i].Val = newHref
	}
	return false // recurse into children
}

// ------------------------------------------------------------------ sound://

// SoundHandler wires audio playback for two raw MDX patterns:
//
//  1. href="sound://file.mp3" — present in LDOCE5++ and Longman Easy for all
//     speaker elements (headword and example). For online sources the href is
//     rewritten to a plain /path; for MDX sources the element is converted to
//     a data-audio-src trigger (see convertToAudioTrigger).
//
//  2. data-src-mp3="/path/to.mp3" with no href — present in LDOCE5++ for
//     inline example audio spans like:
//     <span class="speaker exafile fa fa-volume-up" data-src-mp3="/media/...">
//     These have no href at all so case 1 misses them entirely. We detect them
//     by the presence of data-src-mp3 when no sound:// href was found, and
//     apply the same data-audio-src conversion.
type SoundHandler struct{}

func (SoundHandler) HandleNode(n *html.Node, ctx RenderContext) bool {
	if n.Type != html.ElementNode {
		return false
	}

	// Case 1: href="sound://..."
	for i, a := range n.Attr {
		if a.Key != "href" || !strings.HasPrefix(a.Val, "sound://") {
			continue
		}
		audioFile := strings.TrimPrefix(a.Val, "sound://")
		newPath := "/" + audioFile
		if strings.HasSuffix(ctx.SourceType, "Online") {
			n.Attr[i].Val = newPath
		} else {
			log.Infof("sound handler (href): %q → %q", audioFile, newPath)
			convertToAudioTrigger(n, newPath)
		}
		return false // recurse: children <img> still need their src fixed
	}

	// Case 2: data-src-mp3 with no sound:// href (LDOCE5++ example audio spans).
	// Only applies to MDX sources — online sources use plain hrefs.
	if !strings.HasSuffix(ctx.SourceType, "Online") {
		for _, a := range n.Attr {
			if a.Key != "data-src-mp3" || a.Val == "" {
				continue
			}
			// data-src-mp3 values are already root-relative (/media/english/...).
			log.Infof("sound handler (data-src-mp3): %q", a.Val)
			convertToAudioTrigger(n, a.Val)
			return false
		}
	}

	return false // recurse: children <img> still need their src fixed
}

// convertToAudioTrigger mutates n (any element with a sound:// href) into a
// plain <span> that carries the audio path in a data-audio-src attribute:
//
//	<span [original attrs minus href, plus data-audio-src="/file.mp3" and cursor:pointer]>
//	  [original children — the speaker icon <img> or FontAwesome glyph]
//	</span>
//
// A single delegated click listener on .entry-card in dict.html handles playback
// for all such spans, so no per-element <script> injection is needed.
// This also works in non-browser contexts (Android WebView intercept,
// native UI) where document.currentScript is unavailable.
//
// We use <span> (inline) not <div> (block) because the original <a> is inline
// and often lives inside other inline elements like <span class="Head">.
// We preserve original attributes (especially class for FontAwesome icons)
// and only remove href to avoid navigating to sound:// or javascript:void(0).
func convertToAudioTrigger(n *html.Node, src string) {
	n.DataAtom = atom.Span
	n.Data = "span"
	// Keep all original attributes except href — preserves class (FontAwesome
	// icons), data-src-mp3, title, etc.
	newAttrs := make([]html.Attribute, 0, len(n.Attr)+2)
	hasCursorStyle := false
	for _, a := range n.Attr {
		if a.Key == "href" {
			continue // drop href so it doesn't navigate
		}
		if a.Key == "style" {
			a.Val = a.Val + "; cursor: pointer"
			hasCursorStyle = true
		}
		newAttrs = append(newAttrs, a)
	}
	if !hasCursorStyle {
		newAttrs = append(newAttrs, html.Attribute{Key: "style", Val: "cursor: pointer"})
	}
	newAttrs = append(newAttrs, html.Attribute{Key: "data-audio-src", Val: src})
	n.Attr = newAttrs
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

	// base64 attr is a full data URI (embedded image bytes) — promote directly
	// to data-img-src so the JS lightbox can open it on click.
	if strings.HasPrefix(key, "data:") {
		rewriteShowImageAnchor(n, key)
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
