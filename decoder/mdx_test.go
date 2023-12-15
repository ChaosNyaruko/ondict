package decoder_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ChaosNyaruko/ondict/decoder"
)

func Test_Decode(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5))
	assert.NotEqual(t, 0, len(m.Keys()))
	// t.Logf("%v", m.Dict())
}

func Test_DecodeMDD(t *testing.T) {
	n := decoder.MDict{}
	ldoce5 := "../tmp/Longman Dictionary of Contemporary English.mdd"
	assert.Nil(t, n.Decode(ldoce5))
	x := n.Keys()
	assert.NotEqual(t, 0, len(x))
	t.Logf("keys num of mdd: %v", len(x))
	for i, k := range x {
		t.Logf("key[%d] of mdd: %q", i, k)
	}
	dict, err := n.DumpDict()
	assert.NotNil(t, err)
	assert.Nil(t, dict)
	assert.Nil(t, n.DumpData())
}
