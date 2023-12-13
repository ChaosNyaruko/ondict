package decoder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ChaosNyaruko/ondict/decoder"
)

func Test_Decode(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5))
	t.Logf("dict %v: %v", ldoce5, m.Dict())
}
