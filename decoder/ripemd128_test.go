package decoder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ripemd128(t *testing.T) {
	input := []byte("The quick brown fox jumps over the lazy dog")
	res := ripemd128(input)
	t.Logf("len: %v", len(res))
	assert.Equal(t, []byte{0x3f, 0xa9, 0xb5, 0x7f, 0x05, 0x3c, 0x05, 0x3f, 0xbe, 0x27, 0x35, 0xb2, 0x38, 0x0d, 0xb5, 0x96}, res)
}
