package render

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSoundHandler_PreservesClass(t *testing.T) {
	// LDOCE5++ uses FontAwesome class on the sound anchor:
	// <div class="speaker brefile fa fa-volume-up" href="sound://..." data-src-mp3="...">
	// The fa fa-volume-up class must be preserved so the icon renders.
	raw := `<div class="speaker brefile fa fa-volume-up" data-src-mp3="/media/doctor.mp3" href="sound://media/english/breProns/doctor.mp3" title="Play"> </div>`
	h := &HTMLRender{Raw: raw, SourceType: LongmanEasy}
	got := h.Render()
	assert.Contains(t, got, `class="speaker brefile fa fa-volume-up"`)
	assert.Contains(t, got, `data-src-mp3="/media/doctor.mp3"`)
	assert.NotContains(t, got, `href=`)
	assert.Contains(t, got, `<audio src="/media/english/breProns/doctor.mp3"`)
}
