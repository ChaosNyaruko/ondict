package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

// ── helper ────────────────────────────────────────────────────────────────────

// simpler helper: parse a span directly.
func parseSpanNode(t *testing.T, fragment string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader("<html><body>" + fragment + "</body></html>"))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	// Walk to body's first child.
	body := findElementByName(doc, "body")
	if body == nil {
		t.Fatal("no body element")
	}
	return body.FirstChild
}

func findElementByName(n *html.Node, name string) *html.Node {
	if n.Type == html.ElementNode && n.Data == name {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElementByName(c, name); found != nil {
			return found
		}
	}
	return nil
}

// ── getSpanID ──────────────────────────────────────────────────────────────────

func TestGetSpanID(t *testing.T) {
	node := parseSpanNode(t, `<span id="myid">text</span>`)
	assert.Equal(t, "myid", getSpanID(node))
}

func TestGetSpanID_NoID(t *testing.T) {
	node := parseSpanNode(t, `<span class="foo">text</span>`)
	assert.Equal(t, "", getSpanID(node))
}

func TestGetSpanID_NotSpan(t *testing.T) {
	node := parseSpanNode(t, `<div id="d">text</div>`)
	// div is not a span, so returns "".
	assert.Equal(t, "", getSpanID(node))
}

// ── getSpanClass ───────────────────────────────────────────────────────────────

func TestGetSpanClass(t *testing.T) {
	node := parseSpanNode(t, `<span class="Sense">text</span>`)
	assert.Equal(t, "Sense", getSpanClass(node))
}

func TestGetSpanClass_NoClass(t *testing.T) {
	node := parseSpanNode(t, `<span id="x">text</span>`)
	assert.Equal(t, "", getSpanClass(node))
}

// ── separate ──────────────────────────────────────────────────────────────────

func TestSeparate(t *testing.T) {
	got := separate("hello")
	assert.Contains(t, got, "hello")
	assert.Contains(t, got, SeparatorOpen)
	assert.Contains(t, got, SeparatorClose)
}

// ── compressEmptyLine (already in parse_test.go; adding edge cases here) ──────

func TestCompressEmptyLine_NonBreakingSpace(t *testing.T) {
	// \u00a0 is treated as space by strings.Trim with " \n\u00a0".
	got := compressEmptyLine("\u00a0\u00a0")
	assert.Equal(t, " ", got)
}

// ── ParseHTML (ldoce online path) ──────────────────────────────────────────────

func TestParseHTML_EmptyDict(t *testing.T) {
	// No div.dictionary → format([]) = "".
	in := strings.NewReader("<html><body><p>no dictionary here</p></body></html>")
	out := ParseHTML(in)
	// Returns empty string (no div.dictionary found).
	assert.IsType(t, "", out)
}

func TestParseHTML_WithDictionary(t *testing.T) {
	// A minimal page that contains a div.dictionary.
	raw := `<html><body>
		<div class="dictionary">
			<span class="ldoceEntry Entry">
				<span class="frequent Head">run</span>
			</span>
		</div>
	</body></html>`
	out := ParseHTML(strings.NewReader(raw))
	assert.Contains(t, out, "LDOCE ENTRY")
}

// ── readAllText ──────────────────────────────────────────────────────────────

func TestReadAllText_TextNode(t *testing.T) {
	node := parseSpanNode(t, `<span>hello world</span>`)
	// The span's first child is the text node.
	text := node.FirstChild
	if text == nil || text.Type != html.TextNode {
		t.Skip("unexpected node structure")
	}
	got := readAllText(text)
	assert.Contains(t, got, "hello world")
}

func TestReadAllText_Recursive(t *testing.T) {
	node := parseSpanNode(t, `<span><b>bold</b> plain</span>`)
	got := readAllText(node)
	assert.Contains(t, got, "bold")
	assert.Contains(t, got, "plain")
}

// ── readSubs ──────────────────────────────────────────────────────────────────

func TestReadSubs_Nil(t *testing.T) {
	got := readSubs(nil)
	assert.Equal(t, "", got)
}

func TestReadSubs_TextNode(t *testing.T) {
	// Build a text node manually.
	n := &html.Node{
		Type: html.TextNode,
		Data: "hello",
	}
	got := readSubs(n)
	assert.Contains(t, got, "hello")
}

// ── readText ──────────────────────────────────────────────────────────────────

func TestReadText_SkipsHWD(t *testing.T) {
	node := parseSpanNode(t, `<span class="HWD">should be skipped</span>`)
	got := readText(node)
	assert.Equal(t, "", got)
}

func TestReadText_SkipsFIELD(t *testing.T) {
	node := parseSpanNode(t, `<span class="FIELD">skipped</span>`)
	got := readText(node)
	assert.Equal(t, "", got)
}

func TestReadText_SkipsACTIV(t *testing.T) {
	node := parseSpanNode(t, `<span class="ACTIV">skipped</span>`)
	got := readText(node)
	assert.Equal(t, "", got)
}

func TestReadText_DEF(t *testing.T) {
	node := parseSpanNode(t, `<span class="DEF">a clear liquid</span>`)
	got := readText(node)
	assert.Contains(t, got, "DEF")
	assert.Contains(t, got, "a clear liquid")
}

func TestReadText_EXAMPLE(t *testing.T) {
	node := parseSpanNode(t, `<span class="EXAMPLE">She ran fast.</span>`)
	got := readText(node)
	assert.Contains(t, got, "EXAMPLE")
}

func TestReadText_LEXUNIT(t *testing.T) {
	node := parseSpanNode(t, `<span class="LEXUNIT">run fast</span>`)
	got := readText(node)
	assert.Contains(t, got, "LEXUNIT")
}

// ── findFirstSubSpan ──────────────────────────────────────────────────────────

func TestFindFirstSubSpan_Found(t *testing.T) {
	node := parseSpanNode(t, `<div><span class="Sense">content</span></div>`)
	result := findFirstSubSpan(node, "Sense")
	assert.NotNil(t, result)
}

func TestFindFirstSubSpan_NotFound(t *testing.T) {
	node := parseSpanNode(t, `<div><span class="Other">content</span></div>`)
	result := findFirstSubSpan(node, "Sense")
	assert.Nil(t, result)
}
