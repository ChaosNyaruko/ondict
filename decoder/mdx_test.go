package decoder_test

import (
	"errors"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/ChaosNyaruko/ondict/decoder"
)

func Test_Decode(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))
	assert.NotEqual(t, 0, len(m.Keys()))
	dict, err := m.DumpDict(10)
	assert.Nil(t, err)
	assert.NotNil(t, dict)

	err = m.DumpData()
	assert.NotNil(t, err)
}

func Test_DecodeMDD(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	n := decoder.MDict{}
	// The mdd files are usually too big to be included in the Git repo.
	// Only test it offline for now
	ldoce5 := "../tmp/Longman Dictionary of Contemporary English.mdd"
	err := n.Decode(ldoce5, false)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	assert.Nil(t, err)

	x := n.Keys()
	assert.NotEqual(t, 0, len(x))
	t.Logf("keys num of mdd: %v", len(x))
	dict, err := n.DumpDict(0)
	assert.NotNil(t, err)
	assert.Nil(t, dict)
	n.ReadAtOffset(185995)
	assert.Nil(t, n.DumpData())
}
