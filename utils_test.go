package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/ChaosNyaruko/ondict/render"
)

func Test_Parse(t *testing.T) {
	DumpHTMLDoc("play-with-audio.html")
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
