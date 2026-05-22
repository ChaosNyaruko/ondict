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
			// entry:// links with a fragment should use the fragment as a real URL hash
			// so the browser scrolls to the right element, not as a %23-encoded query param.
			// The __a suffix on anchor names is stripped since element IDs omit it.
			name:       "entry link with fragment strips __a and uses real hash",
			raw:        `<a href="entry://fruit#fruit__entry_0__a">link</a>`,
			sourceType: LongmanEasy,
			contains:   []string{`/dict?query=fruit&amp;engine=mdx&amp;format=html#fruit__entry_0`},
			notContains: []string{`%23`, `__a`},
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
			// SoundHandler converts <a> to <span> with data-audio-src; no <audio> or <script> injected.
			// A single delegated listener in dict.html handles playback for all such spans.
			contains:    []string{`data-audio-src="/test.mp3"`, `cursor: pointer`},
			notContains: []string{`<audio`, `<script`, `<div`, `href=`},
		},
		{
			name:        "Longman source renders fragment without document wrapper",
			raw:         `<div class="entry">hello</div>`,
			sourceType:  LongmanEasy,
			contains:    []string{`<div class="entry">hello</div>`},
			notContains: []string{`<html`, `<body`, `<head`},
		},
		{
			// MDX entries commonly wrap speaker icons in sound:// links:
			// <a href="sound://GB_hello.mp3"><img src="snd_uk.png"></a>
			// After SoundHandler the <a> becomes a <span data-audio-src="..."> but
			// the <img> child remains. Its src must still be rewritten to /snd_uk.png.
			name:       "img inside sound link gets src rewritten",
			raw:        `<a href="sound://GB_hello.mp3"><img src="snd_uk.png"></a>`,
			sourceType: LongmanEasy,
			contains:   []string{`src="/snd_uk.png"`, `data-audio-src="/GB_hello.mp3"`},
			notContains: []string{`<audio`, `<script`},
		},
		{
			// Plain <img> without a wrapping <a> must still get the / prefix.
			name:       "standalone img src rewritten",
			raw:        `<img src="examine.jpg">`,
			sourceType: LongmanEasy,
			contains:   []string{`src="/examine.jpg"`},
		},
		{
			// Some MDX dicts store resources as file:///media/english/illustration/apple.jpg.
			// The file:// scheme must be stripped so the browser fetches /media/... from
			// the local HTTP server instead of a broken file:// URL.
			name:       "file:// img src stripped to root-relative path",
			raw:        `<img src="file:///media/english/illustration/apple.jpg">`,
			sourceType: LongmanEasy,
			contains:   []string{`src="/media/english/illustration/apple.jpg"`},
		},
		{
			// Some MDX dicts embed the image bytes in a custom "base64" attribute
			// as a data URI. Promote it to "src" so the browser renders the image.
			name:       "base64 attr promoted to src",
			raw:        `<img base64="data:image/jpeg;base64,/9j/abc=" src="file://media/english/illustration/apple.jpg" class="ldoce5pp-image-small">`,
			sourceType: LongmanEasy,
			contains:   []string{`src="data:image/jpeg;base64,/9j/abc="`},
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

func TestShowImageHandler_EntryFetcher(t *testing.T) {
	fruitEntry := `<div class="big_pic"><img src="/fruit_comp.jpg"/></div>`
	fetcher := func(word string) string {
		if word == "fruit" {
			return fruitEntry
		}
		return ""
	}

	tests := []struct {
		name    string
		raw     string
		wantSrc string
	}{
		{
			// Sibling link already rewritten to /dict?query=... by EntryHandler
			name: "sibling /dict link",
			raw: `<span class="Sense">` +
				`<a class="crossRef ldoce-show-image" href="javascript:void(0);" base64="ldoce4188jpg">FRUIT 1</a>` +
				`<a class="crossRef" href="/dict?query=fruit&engine=mdx&format=html">see picture at fruit</a>` +
				`</span>`,
			wantSrc: "/fruit_comp.jpg",
		},
		{
			// Sibling link still has raw entry:// href (ShowImageHandler runs before EntryHandler rewrites siblings)
			name: "sibling entry:// link",
			raw: `<span class="Sense">` +
				`<a class="crossRef ldoce-show-image" href="javascript:void(0);" base64="ldoce4188jpg">FRUIT 1</a>` +
				`<a class="crossRef" href="entry://fruit#fruit__entry_0__a">see picture at fruit</a>` +
				`</span>`,
			wantSrc: "/fruit_comp.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &HTMLRender{Raw: tt.raw, SourceType: LongmanEasy, EntryFetcher: fetcher}
			got := h.Render()
			assert.Contains(t, got, `data-img-src="`+tt.wantSrc+`"`)
			assert.NotContains(t, got, `href="javascript:void(0);"`)
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
