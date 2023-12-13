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
	assert.NotEqual(t, 0, len(m.Dict()))
	// t.Logf("%v", m.Dict())
}
