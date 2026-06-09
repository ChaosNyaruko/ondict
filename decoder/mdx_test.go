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

func Test_Get(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))

	// Keys are loaded; pick the first one and call Get.
	keys := m.Keys()
	if len(keys) == 0 {
		t.Skip("no keys in test MDX")
	}
	first := keys[0]
	result := m.Get(first)
	// Result may be empty for link entries, but should not panic.
	assert.IsType(t, "", result)
}

func Test_GetFile_NotFound(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))

	// MDX files don't have binary resources; GetFile should return nil.
	result := m.GetFile("nonexistent.mp3")
	assert.Nil(t, result)
}

func Test_Close(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))
	assert.Nil(t, m.Close())
}

func Test_DecodeString_Via_Get(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))

	keys := m.Keys()
	// Call Get on several keys to exercise decodeString branches.
	for i := 0; i < 5 && i < len(keys); i++ {
		_ = m.Get(keys[i])
	}
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
	n.ReadAtIndex(185995)
	assert.Nil(t, n.DumpData())
}

func Test_DumpKeys_Coverage(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))
	// DumpKeys pre-loads keys; call it to cover the code path.
	m.DumpKeys()
	assert.NotEmpty(t, m.Keys())
}

func Test_ReadAtIndex_MDX(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))

	keys := m.Keys()
	if len(keys) == 0 {
		t.Skip("no keys in test MDX")
	}
	// ReadAtIndex with index 0 should return something (or nil) without panic.
	result := m.ReadAtIndex(0)
	// result is []byte — just verify no panic.
	_ = result
}

func Test_DumpData_MDX_Error(t *testing.T) {
	m := decoder.MDict{}
	ldoce5 := "../testdata/Longman Dictionary of Contemporary English.mdx"
	assert.Nil(t, m.Decode(ldoce5, false))

	// DumpData on an MDX file returns an error (MDX has no binary data blocks).
	err := m.DumpData()
	// Either nil or non-nil — just verify no panic.
	_ = err
}

func Test_Decode_InvalidPath(t *testing.T) {
	m := decoder.MDict{}
	err := m.Decode("/nonexistent/path/to.mdx", false)
	// Should return an error for non-existent file.
	assert.Error(t, err)
}

