package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_pureEmptyLine(t *testing.T) {
	assert.Equal(t, true, pureEmptyLineEndLF("\n"))
	assert.Equal(t, true, pureEmptyLineEndLF("\n    \u00a0"))
	assert.Equal(t, true, pureEmptyLineEndLF("\n    \u00a0"))
	assert.Equal(t, false, pureEmptyLineEndLF(""))
	assert.Equal(t, false, pureEmptyLineEndLF("\n    \u00a0 "))
}
