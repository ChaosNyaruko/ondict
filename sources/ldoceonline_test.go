package sources

import "testing"

func Test_QueryByURL(t *testing.T) {
	get := QueryByURL("doctor")
	t.Logf("get doctor from ldoceonline: %q", get)
}
