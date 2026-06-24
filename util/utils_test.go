package util_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ChaosNyaruko/ondict/render"
	"github.com/ChaosNyaruko/ondict/util"
)

func Test_Parse(t *testing.T) {
	util.DumpHTMLDoc("play-with-audio.html")
}

func Test_ReplaceMp3(t *testing.T) {
	hdoc, err := os.ReadFile("detour.html")
	if err != nil {
		panic(err)
	}
	h := render.HTMLRender{
		Raw:        string(hdoc),
		SourceType: "LONGMAN/Easy",
	}
	out := h.Render()
	fmt.Fprintf(os.Stdout, "%v", out)
}

func TestDumpHTMLDoc_ValidFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.html")
	require.NoError(t, os.WriteFile(f, []byte("<html><body>hello world</body></html>"), 0o644))
	// Should not panic.
	util.DumpHTMLDoc(f)
}
