package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSoundHandler_PreservesClass(t *testing.T) {
	// LDOCE5++ uses FontAwesome class on the sound anchor:
	// <a class="speaker brefile fa fa-volume-up" href="sound://..." data-src-mp3="...">
	// The fa fa-volume-up class must be preserved so the icon renders.
	// The handler must NOT inject <audio> or <script> — it sets data-audio-src
	// so a single delegated listener in dict.html handles playback.
	raw := `<a class="speaker brefile fa fa-volume-up" data-src-mp3="/media/doctor.mp3" href="sound://media/english/breProns/doctor.mp3" title="Play"> </a>`
	h := &HTMLRender{Raw: raw, SourceType: LongmanEasy}
	got := h.Render()
	assert.Contains(t, got, `class="speaker brefile fa fa-volume-up"`)
	assert.Contains(t, got, `data-src-mp3="/media/doctor.mp3"`)
	assert.Contains(t, got, `data-audio-src="/media/english/breProns/doctor.mp3"`)
	assert.NotContains(t, got, `href=`)
	assert.NotContains(t, got, `<audio`)
	assert.NotContains(t, got, `<script`)
}

func TestSoundHandler_DataSrcMp3Only(t *testing.T) {
	// LDOCE5++ example audio spans have data-src-mp3 but NO href="sound://...":
	// <span class="speaker exafile fa fa-volume-up" data-src-mp3="/media/english/exaProns/p008-000810649.mp3" title="Play Example">
	// SoundHandler must wire these via data-audio-src too.
	raw := `<span class="speaker exafile fa fa-volume-up" data-src-mp3="/media/english/exaProns/p008-000810649.mp3" title="Play Example"> </span>`
	h := &HTMLRender{Raw: raw, SourceType: "LONGMAN5/Online" + "x"} // any non-Online type
	h.SourceType = LongmanEasy
	got := h.Render()
	assert.Contains(t, got, `class="speaker exafile fa fa-volume-up"`)
	assert.Contains(t, got, `data-audio-src="/media/english/exaProns/p008-000810649.mp3"`)
	assert.NotContains(t, got, `href=`)
	assert.NotContains(t, got, `<audio`)
	assert.NotContains(t, got, `<script`)
}

func TestSoundHandler_DataSrcMp3OnlineIgnored(t *testing.T) {
	// For online sources, data-src-mp3-only elements should NOT be converted
	// (online sources use plain href rewrites, not data-audio-src triggers).
	raw := `<span class="speaker exafile" data-src-mp3="/media/english/exaProns/p008.mp3"> </span>`
	h := &HTMLRender{Raw: raw, SourceType: Longman5Online}
	got := h.Render()
	// Should be unchanged — no data-audio-src added
	assert.NotContains(t, got, `data-audio-src`)
}
