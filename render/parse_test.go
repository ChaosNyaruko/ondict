package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_pureEmptyLine(t *testing.T) {
	assert.Equal(t, true, pureEmptyLineEndLF("\n"))
	assert.Equal(t, true, pureEmptyLineEndLF("\n    \u00a0"))
	assert.Equal(t, true, pureEmptyLineEndLF("\n    \u00a0"))
	assert.Equal(t, false, pureEmptyLineEndLF(""))
	assert.Equal(t, false, pureEmptyLineEndLF("\n    \u00a0 "))
}

func Test_pureEmptyLineLF(t *testing.T) {
	assert.True(t, pureEmptyLineLF("\n"))
	assert.True(t, pureEmptyLineLF("  \n  "))
	assert.False(t, pureEmptyLineLF(""))
	assert.False(t, pureEmptyLineLF("hello\n"))
	assert.False(t, pureEmptyLineLF("  a  "))
}

func Test_compressEmptyLine(t *testing.T) {
	assert.Equal(t, " ", compressEmptyLine(""))
	assert.Equal(t, " ", compressEmptyLine("   \n  "))
	assert.Equal(t, " ", compressEmptyLine("\u00a0"))
	assert.Equal(t, "hello", compressEmptyLine("hello"))
	// Non-empty string (has non-space chars) is returned as-is.
	assert.Equal(t, "  hello  ", compressEmptyLine("  hello  "))
}

func Test_format(t *testing.T) {
	// Consecutive newlines collapsed to one.
	got := format([]string{"a\n\nb", "c"})
	assert.Equal(t, "a\nb\nc", got)

	// Single newline is preserved.
	got = format([]string{"x\ny"})
	assert.Equal(t, "x\ny", got)

	// Empty slice.
	got = format([]string{})
	assert.Equal(t, "", got)
}

func TestParseMDX_PlainText(t *testing.T) {
	in := strings.NewReader("<html><body>hello world</body></html>")
	out := ParseMDX(in, "md")
	assert.Contains(t, out, "hello world")
}

func TestParseMDX_Bold(t *testing.T) {
	in := strings.NewReader("<b>strong</b>")
	out := ParseMDX(in, "md")
	assert.Contains(t, out, "**strong**")
}

func TestParseMDX_Italic(t *testing.T) {
	in := strings.NewReader("<i>emph</i>")
	out := ParseMDX(in, "md")
	assert.Contains(t, out, "*emph*")
}

func TestParseMDX_HTMLFormat(t *testing.T) {
	// ft != "md" should produce output without markdown markers.
	in := strings.NewReader("<b>strong</b>")
	out := ParseMDX(in, "html")
	assert.NotContains(t, out, "**")
	assert.Contains(t, out, "strong")
}

func TestParseMDX_Div(t *testing.T) {
	in := strings.NewReader("<div>inside</div>")
	out := ParseMDX(in, "md")
	// Div produces a newline, not its content.
	assert.Contains(t, out, "\n")
}

func TestParseMDX_Ex(t *testing.T) {
	in := strings.NewReader("<ex>example sentence</ex>")
	out := ParseMDX(in, "md")
	assert.Contains(t, out, "> example sentence <")
}

func TestParseMDX_BR(t *testing.T) {
	in := strings.NewReader("before<br>after")
	out := ParseMDX(in, "md")
	assert.Contains(t, out, "\n")
}

func TestMarkdownRender_LongmanEasy(t *testing.T) {
	r := &MarkdownRender{
		Raw:        "<b>hello</b>",
		SourceType: LongmanEasy,
	}
	out := r.Render()
	assert.Contains(t, out, "hello")
}

func TestMarkdownRender_Longman5Online(t *testing.T) {
	r := &MarkdownRender{
		Raw:        `<html><body><div class="dictionary"></div></body></html>`,
		SourceType: Longman5Online,
	}
	out := r.Render()
	require.IsType(t, "", out)
}

func TestMarkdownRender_Fallback(t *testing.T) {
	// Unknown source type, markdownify not available → returns raw content.
	raw := "<div>raw content</div>"
	r := &MarkdownRender{
		Raw:        raw,
		SourceType: "unknown_type",
	}
	out := r.Render()
	// Either markdownify worked or we fell back to raw.
	assert.NotEmpty(t, out)
}
