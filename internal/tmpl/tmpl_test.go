package tmpl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMust_ParsesTemplates(t *testing.T) {
	tpl := Must()
	require.NotNil(t, tpl)
	// The embedded templates/ dir must contain at least portal.html and dict.html.
	assert.NotNil(t, tpl.Lookup("portal.html"), "portal.html template must exist")
	assert.NotNil(t, tpl.Lookup("dict.html"), "dict.html template must exist")
}
