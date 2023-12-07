package main

import "testing"

func Test_queryByURL(t *testing.T) {
	get := queryByURL("doctor")
	t.Logf("get doctor from ldoceonline: %q", get)
}
