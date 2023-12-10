package sources

import (
	"testing"
)

func Test_New(t *testing.T) {
	LoadConfig()
	GlobalDict.Register()
	ack := New(GlobalDict.mdxDict)
	res := ack.GetRawOutputs("jesus")
	t.Logf("%q output: %v", "jesus", res)
}
