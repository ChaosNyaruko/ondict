package sources

import (
	"os"
	"testing"
)

func Test_New(t *testing.T) {
	var g *MdxDict
	if os.Getenv("FULLTEST") == "1" {
		LoadConfig()
		g = (*G)[0]
	} else {
		d := MdxDict{
			MdxFile: "../testdata/test_dict",
		}
		g = &d
	}
	g.Register(false, false, false)
	ack := NewAho(g.MdxDict)
	res := ack.GetRawOutputs("jesus")
	t.Logf("%q output: %v", "jesus", res)
}
