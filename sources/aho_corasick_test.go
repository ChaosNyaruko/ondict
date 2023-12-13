package sources

import (
	"os"
	"testing"
)

func Test_New(t *testing.T) {
	var g MdxDict
	if os.Getenv("FULLTEST") == "1" {
		LoadConfig()
		g = GlobalDict
	} else {
		dataPath = "../testdata/"
		d := MdxDict{
			mdxFile: "test_dict",
		}
		g = d
	}
	g.Register()
	ack := New(g.mdxDict)
	res := ack.GetRawOutputs("jesus")
	t.Logf("%q output: %v", "jesus", res)
}
