package sources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_loadCssFiles(t *testing.T) {
	res, err := loadAllCss()
	assert.Nil(t, err)
	t.Logf("css file concatenation: \n%v", res)
}
