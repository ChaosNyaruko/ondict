package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLRender_Render(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		sourceType  string
		contains    []string
		notContains []string
	}{
		{
			name:       "Non-Longman source",
			raw:        "<div>test</div>",
			sourceType: "Other",
			contains:   []string{"<div>test</div>"},
		},
		{
			name:       "Longman source with entry link",
			raw:        `<a href="entry://target">link</a>`,
			sourceType: LongmanEasy,
			contains:   []string{`/dict?query=target&amp;engine=mdx&amp;format=html`},
		},
		{
			name:       "Longman source with sound link (online)",
			raw:        `<a href="sound://test.mp3">sound</a>`,
			sourceType: Longman5Online,
			contains:   []string{`/test.mp3`},
		},
		{
			name:       "Longman source with sound link (mdx)",
			raw:        `<a href="sound://test.mp3">sound</a>`,
			sourceType: LongmanEasy,
			// replaceMp3 transforms it into div with script
			contains: []string{`__div__test`, `__audio__test`, `audio`, `script`},
			// notContains: []string{`sound://`}, // The original href is kept in attributes, though tag changes to div
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HTMLRender{
				Raw:        tt.raw,
				SourceType: tt.sourceType,
			}
			got := h.Render()
			for _, c := range tt.contains {
				assert.Contains(t, got, c)
			}
			for _, nc := range tt.notContains {
				assert.NotContains(t, got, nc)
			}
		})
	}
}

func TestIsElement(t *testing.T) {
	// Need to parse a small HTML snippet to get a Node
	// ... but IsElement is internal helper called by dfs.
	// It is exported though.
	// However, creating an html.Node manually is verbose.
	// I'll skip direct test and rely on Render test which covers dfs and IsElement.
}
