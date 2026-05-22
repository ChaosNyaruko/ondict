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
