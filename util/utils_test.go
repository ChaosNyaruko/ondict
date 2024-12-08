package util_test

import (
	"fmt"
	"os"
	"testing"

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
